package handler

import (
	"encoding/csv"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"

	"presensi-qr/internal/models"
	"presensi-qr/internal/service"
	"presensi-qr/internal/store"
	"presensi-qr/internal/timeutil"
)

var phoneRe = regexp.MustCompile(`^\+?[0-9]{8,15}$`)

func cleanPhone(s string) string {
	return strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "").Replace(s)
}

func validCategory(k string) bool {
	if k == models.CategoryUnknown {
		return true
	}
	for _, c := range models.Categories {
		if c.Key == k {
			return true
		}
	}
	return false
}

func memberFilterFrom(c *gin.Context) store.MemberFilter {
	f := store.MemberFilter{
		Q:         c.Query("q"),
		VillageID: atoi64(c.Query("village_id")),
		GroupID:   atoi64(c.Query("group_id")),
		Category:  c.Query("category"),
		Marital:   c.Query("marital"),
	}
	if !validCategory(f.Category) {
		f.Category = ""
	}
	if f.Marital != models.MaritalSingle && f.Marital != models.MaritalMarried {
		f.Marital = ""
	}
	return f
}

// countNonZero menghitung berapa kondisi yang bernilai true (jumlah filter aktif).
func countNonZero(conds ...bool) int {
	n := 0
	for _, c := range conds {
		if c {
			n++
		}
	}
	return n
}

func filterQuery(f store.MemberFilter) string {
	v := url.Values{}
	if f.Q != "" {
		v.Set("q", f.Q)
	}
	if f.VillageID > 0 {
		v.Set("village_id", strconv.FormatInt(f.VillageID, 10))
	}
	if f.GroupID > 0 {
		v.Set("group_id", strconv.FormatInt(f.GroupID, 10))
	}
	if f.Category != "" {
		v.Set("category", f.Category)
	}
	if f.Marital != "" {
		v.Set("marital", f.Marital)
	}
	return v.Encode()
}

func (h *Handler) MembersPage(c *gin.Context) {
	ctx := c.Request.Context()
	f := memberFilterFrom(c)
	members, err := h.Store.ListMembers(ctx, f)
	if err != nil {
		serverErr(c, "/", err)
		return
	}
	villages, _ := h.Store.ListVillages(ctx)
	groups, _ := h.Store.ListGroups(ctx)
	views, _ := h.Store.VillageViews(ctx)
	q := filterQuery(f)

	h.render(c, "members.html", gin.H{
		"Title": "Anggota", "Active": "members",
		"Members": members, "Villages": villages, "Groups": groups, "Views": views,
		"Categories": models.Categories,
		"Filter":     f, "HasMaster": len(groups) > 0,
		"ActiveFilters": countNonZero(f.VillageID > 0, f.GroupID > 0, f.Category != "", f.Marital != ""),
		"PageData": gin.H{"members": members, "villages": villages, "groups": groups, "categories": models.Categories},
		"CardsURL":  template.URL("/members/cards?" + q),
		"ExportURL": template.URL("/members/export?" + q),
	})
}

// parseMemberForm membaca & memvalidasi form tambah/ubah anggota.
func (h *Handler) parseMemberForm(c *gin.Context) (*models.Member, string) {
	m := &models.Member{
		FullName:    clean(c.PostForm("full_name")),
		Phone:       cleanPhone(c.PostForm("phone")),
		GroupID:     atoi64(c.PostForm("group_id")),
		IsActive:    c.PostForm("is_active") == "1",
		ParentName:  clean(c.PostForm("parent_name")),
		ParentPhone: cleanPhone(c.PostForm("parent_phone")),
	}
	switch {
	case m.FullName == "":
		return nil, "Nama lengkap wajib diisi."
	case len(m.FullName) > 120 || len(m.ParentName) > 120:
		return nil, "Nama terlalu panjang."
	case m.GroupID <= 0:
		return nil, "Pilih desa dan kelompok terlebih dulu."
	case m.Phone != "" && !phoneRe.MatchString(m.Phone):
		return nil, "Nomor HP tidak valid (8-15 digit angka)."
	case m.ParentPhone != "" && !phoneRe.MatchString(m.ParentPhone):
		return nil, "Nomor HP orang tua tidak valid (8-15 digit angka)."
	}

	// tanggal lahir: wajib, tidak di masa depan, umur wajar
	bs := strings.TrimSpace(c.PostForm("birth_date"))
	b, err := time.Parse(timeutil.DateLayout, bs)
	if err != nil {
		return nil, "Tanggal lahir wajib diisi."
	}
	if age := models.AgeOn(b, timeutil.Now()); age < 0 || age > 100 {
		return nil, "Tanggal lahir tidak masuk akal. Periksa kembali."
	}
	m.BirthDate = &b

	// status pernikahan hanya relevan (dan wajib) untuk Usia Mandiri 19-30
	if _, _, key := models.Classify(&b, timeutil.Now()); key == models.CategoryMandiri {
		ms := c.PostForm("marital_status")
		if ms != models.MaritalSingle && ms != models.MaritalMarried {
			return nil, "Pilih status pernikahan (belum menikah / menikah) untuk anggota Usia Mandiri."
		}
		m.MaritalStatus = &ms
	}

	// kartu RFID (opsional)
	if uid := service.NormalizeRFID(c.PostForm("rfid_uid")); uid != "" {
		if !service.ValidRFID(uid) {
			return nil, "Nomor kartu RFID tidak valid (4-24 karakter huruf/angka)."
		}
		m.RFID = &uid
	}

	if fh, err := c.FormFile("photo"); err == nil && fh.Size > 0 {
		u, err := h.Files.SavePhoto(c.Request.Context(), fh)
		if err != nil {
			return nil, "Foto: " + err.Error()
		}
		m.PhotoURL = &u
	}
	return m, ""
}

func (h *Handler) memberSaveErr(c *gin.Context, err error) {
	if store.IsUnique(err) {
		fail(c, "/members", "Kartu RFID tersebut sudah terdaftar pada anggota lain.")
		return
	}
	serverErr(c, "/members", err)
}

func (h *Handler) MemberCreate(c *gin.Context) {
	m, msg := h.parseMemberForm(c)
	if m == nil {
		fail(c, "/members", msg)
		return
	}
	if _, err := h.Store.CreateMember(c.Request.Context(), m); err != nil {
		h.memberSaveErr(c, err)
		return
	}
	success(c, "/members", m.FullName+" berhasil ditambahkan. QR Code siap diunduh.")
}

func (h *Handler) MemberUpdate(c *gin.Context) {
	m, msg := h.parseMemberForm(c)
	if m == nil {
		fail(c, "/members", msg)
		return
	}
	m.ID = c.Param("id")
	if err := h.Store.UpdateMember(c.Request.Context(), m); err != nil {
		h.memberSaveErr(c, err)
		return
	}
	success(c, "/members", "Data "+m.FullName+" disimpan.")
}

func (h *Handler) MemberDelete(c *gin.Context) {
	if err := h.Store.DeleteMember(c.Request.Context(), c.Param("id")); err != nil {
		serverErr(c, "/members", err)
		return
	}
	success(c, "/members", "Anggota dihapus beserta riwayat kehadirannya.")
}

// MembersExport  GET /members/export  (CSV, mengikuti filter daftar anggota)
func (h *Handler) MembersExport(c *gin.Context) {
	f := memberFilterFrom(c)
	f.Limit = 5000
	list, err := h.Store.ListMembers(c.Request.Context(), f)
	if err != nil {
		c.String(http.StatusInternalServerError, "Gagal mengambil data")
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="anggota_`+timeutil.TodayStr()+`.csv"`)
	c.Writer.Write([]byte("\xEF\xBB\xBF"))
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"No", "Nama", "Tanggal lahir", "Umur", "Kategori", "No HP", "Nama orang tua", "No HP orang tua", "Desa", "Kelompok", "Kartu RFID", "Status", "Status pernikahan"})
	for i, m := range list {
		birth, age, rfid, status := "", "", "", "Aktif"
		if m.BirthDate != nil {
			birth = m.BirthDate.Format(timeutil.DateLayout)
		}
		if m.Age != nil {
			age = strconv.Itoa(*m.Age)
		}
		if m.RFID != nil {
			rfid = *m.RFID
		}
		if !m.IsActive {
			status = "Nonaktif"
		}
		_ = w.Write([]string{strconv.Itoa(i + 1), m.FullName, birth, age, m.Category, m.Phone,
			m.ParentName, m.ParentPhone, m.VillageName, m.GroupName, rfid, status, m.MaritalLabel})
	}
	w.Flush()
}

// MemberQR  GET /members/:id/qr.png?size=512&download=1
func (h *Handler) MemberQR(c *gin.Context) {
	m, err := h.Store.MemberByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	size := int(atoi64(c.DefaultQuery("size", "512")))
	if size < 128 || size > 1024 {
		size = 512
	}
	png, err := qrcode.Encode(m.Token, qrcode.Medium, size)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if c.Query("download") == "1" {
		name := strings.ReplaceAll(strings.ToLower(m.FullName), " ", "-")
		c.Header("Content-Disposition", `attachment; filename="qr-`+url.PathEscape(name)+`.png"`)
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.Data(http.StatusOK, "image/png", png)
}

// MemberCard menampilkan satu kartu anggota siap cetak.
func (h *Handler) MemberCard(c *gin.Context) {
	m, err := h.Store.MemberByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Anggota tidak ditemukan")
		return
	}
	h.render(c, "card.html", gin.H{"Title": "Kartu " + m.FullName, "Members": []models.Member{*m}})
}

// MemberCards mencetak banyak kartu sekaligus (mengikuti filter daftar anggota).
func (h *Handler) MemberCards(c *gin.Context) {
	f := memberFilterFrom(c)
	f.Limit = 200
	list, err := h.Store.ListMembers(c.Request.Context(), f)
	if err != nil {
		serverErr(c, "/members", err)
		return
	}
	h.render(c, "card.html", gin.H{"Title": "Cetak kartu anggota", "Members": list})
}
