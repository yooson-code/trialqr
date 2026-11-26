package handler

import (
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"presensi-qr/internal/models"
	"presensi-qr/internal/store"
	"presensi-qr/internal/timeutil"
)

const maxText = 4000

// monthStart mengubah "2026-05" menjadi 1 Mei 2026.
func monthStart(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01", strings.TrimSpace(s))
	return t, err == nil
}

func currentMonth() time.Time {
	n := timeutil.Now()
	return time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func periodRange(p time.Time) (from, to string) {
	return p.Format(timeutil.DateLayout), p.AddDate(0, 1, -1).Format(timeutil.DateLayout)
}

func canEditReport(u *models.Admin, r *models.Report) bool {
	return u.IsAdmin() || (r.AuthorID != 0 && r.AuthorID == u.ID)
}

func canReadReport(u *models.Admin, r *models.Report) bool {
	return r.IsSubmitted() || canEditReport(u, r)
}

// loadReport memuat laporan dan memeriksa hak akses; mengembalikan nil bila sudah merespons.
func (h *Handler) loadReport(c *gin.Context, needEdit bool) *models.Report {
	r, err := h.Store.GetReport(c.Request.Context(), idParam(c))
	u := userOf(c)
	if err != nil || !canReadReport(u, r) {
		fail(c, "/reports", "Laporan tidak ditemukan.")
		return nil
	}
	if needEdit && !canEditReport(u, r) {
		fail(c, "/reports/"+strconv.FormatInt(r.ID, 10), "Hanya penulis laporan atau admin yang bisa mengubahnya.")
		return nil
	}
	return r
}

func (h *Handler) reportAttendance(c *gin.Context, r *models.Report) models.AttendanceSummary {
	from, to := periodRange(r.Period)
	var vid, gid int64
	if r.VillageID != nil {
		vid = *r.VillageID
	}
	if r.GroupID != nil {
		gid = *r.GroupID
	}
	sum, _ := h.Store.AttendanceSummary(c.Request.Context(), from, to, r.Level, vid, gid)
	return sum
}

// ---------- daftar ----------

func (h *Handler) periodParam(c *gin.Context) (period time.Time, all bool) {
	q := c.Query("period")
	if q == "all" {
		return time.Time{}, true
	}
	if t, ok := monthStart(q); ok {
		return t, false
	}
	return currentMonth(), false
}

func (h *Handler) ReportsPage(c *gin.Context) {
	u := userOf(c)
	period, all := h.periodParam(c)
	f := store.ReportFilter{ViewerID: u.ID, ViewerAdmin: u.IsAdmin()}
	if !all {
		f.Period = period.Format(timeutil.DateLayout)
	}
	list, err := h.Store.ListReports(c.Request.Context(), f)
	if err != nil {
		serverErr(c, "/", err)
		return
	}
	submitted := 0
	for _, r := range list {
		if r.IsSubmitted() {
			submitted++
		}
	}
	data := gin.H{
		"Title": "Laporan musyawarah", "Active": "reports", "Reports": list,
		"Period": period, "All": all, "Submitted": submitted, "Drafts": len(list) - submitted,
		"ThisMonth": currentMonth(),
	}
	if !all {
		q := url.Values{"period": {period.Format("2006-01")}}.Encode()
		data["CompileURL"] = template.URL("/reports/compile?" + q)
		data["ExportURL"] = template.URL("/reports/export?" + q)
	} else {
		data["ExportURL"] = template.URL("/reports/export?period=all")
	}
	h.render(c, "reports.html", data)
}

// ---------- form ----------

func (h *Handler) reportForm(c *gin.Context, r *models.Report, isNew bool) {
	views, _ := h.Store.VillageViews(c.Request.Context())
	data := gin.H{
		"Title": "Laporan musyawarah", "Active": "reports", "Report": r, "IsNew": isNew, "Views": views,
		"Statuses": models.ItemStatuses, "FormAction": "/reports",
		"SelVillage": derefID(r.VillageID), "SelGroup": derefID(r.GroupID),
	}
	if !isNew {
		data["FormAction"] = "/reports/" + strconv.FormatInt(r.ID, 10)
		sum := h.reportAttendance(c, r)
		data["Summary"] = &sum
	}
	h.render(c, "report_form.html", data)
}

func (h *Handler) ReportNew(c *gin.Context) {
	u := userOf(c)
	period, _ := h.periodParam(c)
	if id, _, _ := h.Store.ReportOf(c.Request.Context(), u.ID, period.Format(timeutil.DateLayout)); id != 0 {
		done(c, "/reports/"+strconv.FormatInt(id, 10)+"/edit", "ok",
			"Anda sudah punya laporan untuk "+monthLabel(period)+". Lanjutkan dari sini.")
		return
	}
	h.reportForm(c, &models.Report{
		Period: period, AuthorName: u.FullName, Level: models.LevelDaerah,
		Items: []models.ReportItem{{Status: "berjalan"}},
	}, true)
}

func (h *Handler) ReportEdit(c *gin.Context) {
	r := h.loadReport(c, true)
	if r == nil {
		return
	}
	if len(r.Items) == 0 {
		r.Items = []models.ReportItem{{Status: "berjalan"}}
	}
	h.reportForm(c, r, false)
}

func derefID(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func trimText(s string) string { return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n")) }

func (h *Handler) parseReportForm(c *gin.Context) (*models.Report, string) {
	period, ok := monthStart(c.PostForm("period"))
	if !ok {
		return nil, "Periode laporan tidak valid. Pilih bulan dan tahun."
	}
	r := &models.Report{
		Period:         period,
		Bidang:         clean(c.PostForm("bidang")),
		Level:          c.PostForm("level"),
		Summary:        trimText(c.PostForm("summary")),
		AttendanceNote: trimText(c.PostForm("attendance_note")),
		Evaluation:     trimText(c.PostForm("evaluation")),
		BudgetNeeds:    trimText(c.PostForm("budget_needs")),
		Topics:         trimText(c.PostForm("topics")),
	}
	if r.Bidang == "" {
		return nil, "Bidang atau jabatan wajib diisi."
	}
	for _, t := range []string{r.Bidang, r.Summary, r.AttendanceNote, r.Evaluation, r.BudgetNeeds, r.Topics} {
		if utf8.RuneCountInString(t) > maxText {
			return nil, "Salah satu isian terlalu panjang (maksimal 4000 karakter)."
		}
	}

	switch r.Level {
	case models.LevelDesa:
		v := atoi64(c.PostForm("village_id"))
		if v <= 0 {
			return nil, "Pilih desa untuk laporan tingkat desa."
		}
		r.VillageID = &v
	case models.LevelKelompok:
		g := atoi64(c.PostForm("group_id"))
		if g <= 0 {
			return nil, "Pilih kelompok untuk laporan tingkat kelompok."
		}
		r.GroupID = &g
	default:
		r.Level = models.LevelDaerah
	}

	// kegiatan: input paralel item_name[], item_status[], ... (urutan sama dengan baris di form)
	names := c.PostFormArray("item_name")
	statuses := c.PostFormArray("item_status")
	ach := c.PostFormArray("item_achievement")
	obs := c.PostFormArray("item_obstacle")
	fol := c.PostFormArray("item_followup")
	at := func(a []string, i int) string {
		if i < len(a) {
			return trimText(a[i])
		}
		return ""
	}
	for i := range names {
		if len(r.Items) >= 40 {
			break
		}
		it := models.ReportItem{
			Name: clean(names[i]), Status: at(statuses, i),
			Achievement: at(ach, i), Obstacle: at(obs, i), Followup: at(fol, i),
		}
		if it.Name == "" {
			continue
		}
		if models.ItemStatusLabel(it.Status) == "Berjalan" && it.Status != "berjalan" {
			it.Status = "berjalan"
		}
		for _, t := range []string{it.Name, it.Achievement, it.Obstacle, it.Followup} {
			if utf8.RuneCountInString(t) > maxText {
				return nil, "Isian kegiatan terlalu panjang (maksimal 4000 karakter)."
			}
		}
		r.Items = append(r.Items, it)
	}
	return r, ""
}

func readyToSubmit(r *models.Report) string {
	if r.Summary == "" && len(r.Items) == 0 {
		return "Isi minimal ringkasan kinerja atau satu kegiatan sebelum mengirim laporan."
	}
	return ""
}

func reportURL(id int64) string { return "/reports/" + strconv.FormatInt(id, 10) }

func (h *Handler) ReportCreate(c *gin.Context) {
	u := userOf(c)
	r, msg := h.parseReportForm(c)
	if r == nil {
		fail(c, "/reports/new", msg)
		return
	}
	submit := c.PostForm("action") == "submit"
	if submit {
		if m := readyToSubmit(r); m != "" {
			fail(c, "/reports/new?period="+r.Period.Format("2006-01"), m)
			return
		}
		r.Status = models.ReportSubmitted
	} else {
		r.Status = models.ReportDraft
	}
	r.AuthorID, r.AuthorName = u.ID, u.FullName

	id, err := h.Store.CreateReport(c.Request.Context(), r)
	if err != nil {
		switch {
		case store.IsUnique(err):
			fail(c, "/reports", "Anda sudah membuat laporan untuk "+monthLabel(r.Period)+". Buka dan ubah laporan tersebut.")
		case store.IsInUse(err):
			fail(c, "/reports/new", "Desa atau kelompok yang dipilih tidak ditemukan.")
		default:
			serverErr(c, "/reports/new", err)
		}
		return
	}
	if submit {
		success(c, reportURL(id), "Laporan "+monthLabel(r.Period)+" dikirim dan bisa dibaca peserta musyawarah.")
	} else {
		success(c, reportURL(id), "Draft laporan disimpan. Hanya Anda yang bisa melihatnya sampai dikirim.")
	}
}

func (h *Handler) ReportUpdate(c *gin.Context) {
	old := h.loadReport(c, true)
	if old == nil {
		return
	}
	edit := reportURL(old.ID) + "/edit"
	r, msg := h.parseReportForm(c)
	if r == nil {
		fail(c, edit, msg)
		return
	}
	r.ID = old.ID
	submit := c.PostForm("action") == "submit"
	if submit {
		if m := readyToSubmit(r); m != "" {
			fail(c, edit, m)
			return
		}
	}
	if err := h.Store.UpdateReport(c.Request.Context(), r); err != nil {
		switch {
		case store.IsUnique(err):
			fail(c, edit, "Sudah ada laporan Anda untuk "+monthLabel(r.Period)+". Pilih bulan lain.")
		case store.IsInUse(err):
			fail(c, edit, "Desa atau kelompok yang dipilih tidak ditemukan.")
		default:
			serverErr(c, edit, err)
		}
		return
	}
	if submit && !old.IsSubmitted() {
		if err := h.Store.SetReportStatus(c.Request.Context(), old.ID, models.ReportSubmitted); err != nil {
			serverErr(c, edit, err)
			return
		}
		success(c, reportURL(old.ID), "Laporan dikirim dan bisa dibaca peserta musyawarah.")
		return
	}
	success(c, reportURL(old.ID), "Laporan disimpan.")
}

func (h *Handler) ReportSubmit(c *gin.Context) {
	r := h.loadReport(c, true)
	if r == nil {
		return
	}
	if m := readyToSubmit(r); m != "" {
		fail(c, reportURL(r.ID)+"/edit", m)
		return
	}
	if err := h.Store.SetReportStatus(c.Request.Context(), r.ID, models.ReportSubmitted); err != nil {
		serverErr(c, reportURL(r.ID), err)
		return
	}
	success(c, reportURL(r.ID), "Laporan dikirim dan bisa dibaca peserta musyawarah.")
}

func (h *Handler) ReportReopen(c *gin.Context) {
	r := h.loadReport(c, true)
	if r == nil {
		return
	}
	if err := h.Store.SetReportStatus(c.Request.Context(), r.ID, models.ReportDraft); err != nil {
		serverErr(c, reportURL(r.ID), err)
		return
	}
	success(c, reportURL(r.ID), "Laporan ditarik kembali menjadi draft.")
}

func (h *Handler) ReportDelete(c *gin.Context) {
	r := h.loadReport(c, true)
	if r == nil {
		return
	}
	if err := h.Store.DeleteReport(c.Request.Context(), r.ID); err != nil {
		serverErr(c, "/reports", err)
		return
	}
	success(c, "/reports?period="+r.Period.Format("2006-01"), "Laporan dihapus.")
}

// ---------- baca satu laporan ----------

func (h *Handler) ReportView(c *gin.Context) {
	r := h.loadReport(c, false)
	if r == nil {
		return
	}
	sum := h.reportAttendance(c, r)
	h.render(c, "report_view.html", gin.H{
		"Title": "Laporan " + monthLabel(r.Period), "Active": "reports",
		"Report": r, "Summary": &sum, "CanEdit": canEditReport(userOf(c), r),
	})
}

// ---------- ringkasan untuk musyawarah (bisa dicetak) ----------

type compileLine struct{ Author, Bidang, Scope, Item, Text, Followup string }

func (h *Handler) ReportsCompile(c *gin.Context) {
	period, _ := h.periodParam(c)
	ctx := c.Request.Context()
	list, err := h.Store.ListReports(ctx, store.ReportFilter{
		Period: period.Format(timeutil.DateLayout), OnlySubmitted: true,
	})
	if err != nil {
		serverErr(c, "/reports", err)
		return
	}
	var topics, budgets, obstacles []compileLine
	totalItems, doneItems := 0, 0
	for _, r := range list {
		base := compileLine{Author: r.AuthorName, Bidang: r.Bidang, Scope: r.Scope}
		if r.Topics != "" {
			l := base
			l.Text = r.Topics
			topics = append(topics, l)
		}
		if r.BudgetNeeds != "" {
			l := base
			l.Text = r.BudgetNeeds
			budgets = append(budgets, l)
		}
		for _, it := range r.Items {
			totalItems++
			if it.Status == "selesai" {
				doneItems++
			}
			if it.Obstacle != "" {
				l := base
				l.Item, l.Text, l.Followup = it.Name, it.Obstacle, it.Followup
				obstacles = append(obstacles, l)
			}
		}
	}
	from, to := periodRange(period)
	sum, _ := h.Store.AttendanceSummary(ctx, from, to, models.LevelDaerah, 0, 0)

	pct := 0
	if totalItems > 0 {
		pct = doneItems * 100 / totalItems
	}
	q := url.Values{"period": {period.Format("2006-01")}}.Encode()
	h.render(c, "report_compile.html", gin.H{
		"Title": "Ringkasan musyawarah", "Active": "reports", "Period": period, "Reports": list,
		"Summary": &sum, "Topics": topics, "Budgets": budgets, "Obstacles": obstacles,
		"TotalItems": totalItems, "DoneItems": doneItems, "ItemsPct": pct,
		"ExportURL": template.URL("/reports/export?" + q), "Printed": timeutil.Now(),
	})
}

// ---------- ekspor Excel ----------

func (h *Handler) ReportsExport(c *gin.Context) {
	u := userOf(c)
	period, all := h.periodParam(c)
	f := store.ReportFilter{ViewerID: u.ID, ViewerAdmin: u.IsAdmin(), OnlySubmitted: true}
	name := "laporan-musyawarah_semua-periode"
	if !all {
		f.Period = period.Format(timeutil.DateLayout)
		name = "laporan-musyawarah_" + period.Format("2006-01")
	}
	list, err := h.Store.ListReports(c.Request.Context(), f)
	if err != nil {
		c.String(http.StatusInternalServerError, "Gagal mengambil data")
		return
	}

	x := excelize.NewFile()
	defer x.Close()
	s1, s2 := "Laporan", "Kegiatan"
	x.SetSheetName("Sheet1", s1)
	_, _ = x.NewSheet(s2)

	head, _ := x.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"173B34"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
	})
	body, _ := x.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}})

	h1 := []any{"Periode", "Pengurus", "Bidang / jabatan", "Cakupan", "Dikirim", "Ringkasan kinerja", "Jumlah kegiatan", "Selesai",
		"Catatan kehadiran", "Poin evaluasi", "Kebutuhan anggaran / logistik", "Topik musyawarah"}
	h2 := []any{"Periode", "Pengurus", "Bidang / jabatan", "Kegiatan", "Status", "Capaian", "Kendala", "Rencana tindak lanjut"}
	_ = x.SetSheetRow(s1, "A1", &h1)
	_ = x.SetSheetRow(s2, "A1", &h2)

	r1, r2 := 2, 2
	for _, r := range list {
		sent := ""
		if r.SubmittedAt != nil {
			sent = r.SubmittedAt.In(timeutil.WIB).Format("02-01-2006 15:04")
		}
		row := []any{monthLabel(r.Period), r.AuthorName, r.Bidang, r.Scope, sent, r.Summary, len(r.Items), r.ItemsDone(),
			r.AttendanceNote, r.Evaluation, r.BudgetNeeds, r.Topics}
		cell, _ := excelize.CoordinatesToCellName(1, r1)
		_ = x.SetSheetRow(s1, cell, &row)
		r1++
		for _, it := range r.Items {
			row2 := []any{monthLabel(r.Period), r.AuthorName, r.Bidang, it.Name, models.ItemStatusLabel(it.Status),
				it.Achievement, it.Obstacle, it.Followup}
			cell, _ := excelize.CoordinatesToCellName(1, r2)
			_ = x.SetSheetRow(s2, cell, &row2)
			r2++
		}
	}
	_ = x.SetCellStyle(s1, "A1", "L1", head)
	_ = x.SetCellStyle(s2, "A1", "H1", head)
	if r1 > 2 {
		_ = x.SetCellStyle(s1, "A2", "L"+strconv.Itoa(r1-1), body)
	}
	if r2 > 2 {
		_ = x.SetCellStyle(s2, "A2", "H"+strconv.Itoa(r2-1), body)
	}
	for i, w := range []float64{14, 22, 22, 24, 17, 48, 10, 9, 36, 36, 36, 36} {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = x.SetColWidth(s1, col, col, w)
	}
	for i, w := range []float64{14, 22, 22, 32, 14, 40, 40, 40} {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = x.SetColWidth(s2, col, col, w)
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", `attachment; filename="`+name+`.xlsx"`)
	_ = x.Write(c.Writer)
}
