package handler

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"presensi-qr/internal/config"
	"presensi-qr/internal/middleware"
	"presensi-qr/internal/models"
	"presensi-qr/internal/service"
	"presensi-qr/internal/storage"
	"presensi-qr/internal/store"
	"presensi-qr/internal/timeutil"
	"presensi-qr/web"
)

type Handler struct {
	Store   *store.Store
	Cfg     config.Config
	Files   *storage.Storage
	Scan    *service.ScanService
	Limiter *middleware.LoginLimiter

	pages map[string]*template.Template
}

// Halaman yang tidak memakai kerangka sidebar: nama file -> nama template pembuka.
var standalone = map[string]string{"login.html": "login", "card.html": "card"}

type navItem struct {
	Key, Label, Href, Icon string
	AdminOnly              bool
}

var nav = []navItem{
	{"dashboard", "Dashboard", "/", "home", false},
	{"scanner", "Scanner", "/scanner", "scan", false},
	{"attendance", "Rekap kehadiran", "/attendance", "table", false},
	{"reports", "Laporan musyawarah", "/reports", "file", false},
	{"members", "Anggota", "/members", "users", false},
	{"schedules", "Jadwal pengajian", "/schedules", "calendar", false},
	{"master", "Desa & kelompok", "/master", "pin", false},
	{"accounts", "Akun pengurus", "/accounts", "shield", true},
}

type flash struct{ Kind, Message string }

func New(st *store.Store, cfg config.Config, files *storage.Storage, lim *middleware.LoginLimiter) (*Handler, error) {
	h := &Handler{
		Store: st, Cfg: cfg, Files: files, Limiter: lim,
		Scan:  service.NewScanService(st),
		pages: map[string]*template.Template{},
	}
	entries, err := fs.ReadDir(web.FS, "templates")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		n := e.Name()
		if n == "layout.html" {
			continue
		}
		t, err := template.New(n).Funcs(funcMap()).ParseFS(web.FS, "templates/layout.html", "templates/"+n)
		if err != nil {
			return nil, fmt.Errorf("template %s: %w", n, err)
		}
		h.pages[n] = t
	}
	return h, nil
}

func (h *Handler) render(c *gin.Context, page string, data gin.H) {
	t, ok := h.pages[page]
	if !ok {
		c.String(http.StatusInternalServerError, "template %s tidak ditemukan", page)
		return
	}
	if data == nil {
		data = gin.H{}
	}
	entry := "layout"
	if name, ok := standalone[page]; ok {
		entry = name
	}
	if u := middleware.CurrentUser(c); u != nil {
		data["User"] = u
		var items []navItem
		for _, n := range nav {
			if !n.AdminOnly || u.IsAdmin() {
				items = append(items, n)
			}
		}
		data["Nav"] = items
	}
	if f := popFlash(c); f != nil {
		data["Flash"] = f
	}
	data["Today"] = timeutil.TodayStr()

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, entry, data); err != nil {
		log.Printf("render %s: %v", page, err)
		c.String(http.StatusInternalServerError, "Terjadi kesalahan saat menampilkan halaman")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

// ---- flash message (cookie sekali pakai) ----

func setFlash(c *gin.Context, kind, msg string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("flash", kind+"|"+msg, 30, "/", "", false, true)
}

func popFlash(c *gin.Context) *flash {
	v, err := c.Cookie("flash")
	if err != nil || v == "" {
		return nil
	}
	c.SetCookie("flash", "", -1, "/", "", false, true)
	kind, msg, ok := strings.Cut(v, "|")
	if !ok {
		return nil
	}
	return &flash{Kind: kind, Message: msg}
}

// done menyimpan flash lalu redirect (pola Post/Redirect/Get).
func done(c *gin.Context, to, kind, msg string) {
	setFlash(c, kind, msg)
	c.Redirect(http.StatusSeeOther, to)
}

func success(c *gin.Context, to, msg string) { done(c, to, "ok", msg) }
func fail(c *gin.Context, to, msg string)    { done(c, to, "err", msg) }

func serverErr(c *gin.Context, to string, err error) {
	log.Printf("%s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	fail(c, to, "Terjadi kesalahan pada server. Coba lagi.")
}

func idParam(c *gin.Context) int64 {
	n, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	return n
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func userOf(c *gin.Context) *models.Admin { return middleware.CurrentUser(c) }
