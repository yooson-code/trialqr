package handler

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"
	"unicode"

	"presensi-qr/internal/models"
	"presensi-qr/internal/timeutil"
)

var (
	bulan = []string{"", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	hari  = []string{"Min", "Sen", "Sel", "Rab", "Kam", "Jum", "Sab"}
)

// icons: isi <svg> bergaya garis (stroke) 24x24.
var icons = map[string]string{
	"home":     `<path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1z"/>`,
	"scan":     `<path d="M3 7V5a2 2 0 0 1 2-2h2M17 3h2a2 2 0 0 1 2 2v2M21 17v2a2 2 0 0 1-2 2h-2M7 21H5a2 2 0 0 1-2-2v-2"/><path d="M7 12h10"/>`,
	"calendar": `<rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/>`,
	"users":    `<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/>`,
	"table":    `<path d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2"/><rect x="9" y="3" width="6" height="4" rx="1"/><path d="m9 14 2 2 4-4"/>`,
	"pin":      `<path d="M20 10c0 6-8 12-8 12S4 16 4 10a8 8 0 0 1 16 0z"/><circle cx="12" cy="10" r="3"/>`,
	"shield":   `<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="m9 12 2 2 4-4"/>`,
	"logout":   `<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9"/>`,
	"menu":     `<path d="M4 6h16M4 12h16M4 18h16"/>`,
	"x":        `<path d="M18 6 6 18M6 6l12 12"/>`,
	"plus":     `<path d="M12 5v14M5 12h14"/>`,
	"edit":     `<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z"/>`,
	"trash":    `<path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6M10 11v6M14 11v6"/>`,
	"qr":       `<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><path d="M14 14h3v3h-3zM20 14v.01M14 20h3M20 17v4"/>`,
	"download": `<path d="M12 3v12m0 0 4-4m-4 4-4-4M4 21h16"/>`,
	"print":    `<path d="M6 9V3h12v6M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2"/><rect x="6" y="14" width="12" height="8" rx="1"/>`,
	"camera":   `<path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/><circle cx="12" cy="13" r="4"/>`,
	"flip":     `<path d="M3 12a9 9 0 0 1 15-6.7L21 8M21 3v5h-5M21 12a9 9 0 0 1-15 6.7L3 16M3 21v-5h5"/>`,
	"volume":   `<path d="M11 5 6 9H2v6h4l5 4zM15.5 8.5a5 5 0 0 1 0 7M19 5a9 9 0 0 1 0 14"/>`,
	"mute":     `<path d="M11 5 6 9H2v6h4l5 4zM22 9l-6 6M16 9l6 6"/>`,
	"check":    `<path d="m5 12 5 5L20 7"/>`,
	"key":      `<circle cx="7.5" cy="15.5" r="4.5"/><path d="m10.7 12.3 9.8-9.8M17 6l3 3M14 9l2 2"/>`,
	"search":   `<circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/>`,
	"file":     `<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5M9 13h6M9 17h6"/>`,
	"card":     `<rect x="2" y="5" width="20" height="14" rx="2"/><path d="M2 10h20M6 15h4"/>`,
	"phone":    `<path d="M22 16.9v3a2 2 0 0 1-2.2 2 19.8 19.8 0 0 1-8.6-3.1 19.5 19.5 0 0 1-6-6A19.8 19.8 0 0 1 2.1 4.2 2 2 0 0 1 4.1 2h3a2 2 0 0 1 2 1.7c.1 1 .4 1.9.7 2.8a2 2 0 0 1-.5 2.1L8.1 9.9a16 16 0 0 0 6 6l1.3-1.3a2 2 0 0 1 2.1-.4c.9.3 1.8.6 2.8.7a2 2 0 0 1 1.7 2z"/>`,
}

var nonDigit = regexp.MustCompile(`\D`)

func funcMap() template.FuncMap {
	return template.FuncMap{
		"icon": func(name string) template.HTML {
			return template.HTML(`<svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">` + icons[name] + `</svg>`)
		},
		"date":      func(t time.Time) string { return fmtDate(t, true) },
		"dateShort": func(t time.Time) string { return fmtDate(t, false) },
		"dateStr": func(s string) string {
			t, err := time.Parse(timeutil.DateLayout, s)
			if err != nil {
				return s
			}
			return fmtDate(t, false)
		},
		"dateInput": func(t time.Time) string { return t.Format(timeutil.DateLayout) },
		"clock":     func(t time.Time) string { return t.In(timeutil.WIB).Format("15:04") },
		"clockSec":  func(t time.Time) string { return t.In(timeutil.WIB).Format("15:04:05") },
		"initials":  initials,
		"monthLabel":  monthLabel,
		"periodInput": func(t time.Time) string { return t.Format("2006-01") },
		"itemStatus":  models.ItemStatusLabel,
		"itemChip":    itemChip,
		"dateTime": func(t time.Time) string {
			t = t.In(timeutil.WIB)
			return fmtDate(t, false) + ", " + t.Format("15:04") + " WIB"
		},
		"catChip":   catChip,
		"wa":        waLink,
		"add":       func(a, b int) int { return a + b },
		"qrart":     qrArt,
	}
}

func fmtDate(t time.Time, withDay bool) string {
	// kolom `date` dari Postgres tidak punya zona; jangan digeser ke WIB.
	s := fmt.Sprintf("%d %s %d", t.Day(), bulan[t.Month()], t.Year())
	if withDay {
		return hari[t.Weekday()] + ", " + s
	}
	return s
}

func initials(name string) string {
	var out []rune
	for _, w := range strings.Fields(name) {
		out = append(out, unicode.ToUpper([]rune(w)[0]))
		if len(out) == 2 {
			break
		}
	}
	return string(out)
}

// waLink mengubah 08xx / +62xx menjadi tautan wa.me.
func waLink(phone string) string {
	d := nonDigit.ReplaceAllString(phone, "")
	switch {
	case d == "":
		return ""
	case strings.HasPrefix(d, "0"):
		d = "62" + d[1:]
	case strings.HasPrefix(d, "8"):
		d = "62" + d
	}
	return "https://wa.me/" + d
}

// qrArt menggambar ilustrasi bergaya QR (dekoratif, bukan QR sungguhan) untuk halaman login.
func qrArt() template.HTML {
	const n = 17
	finder := func(x, y int) bool {
		for _, o := range [][2]int{{0, 0}, {n - 5, 0}, {0, n - 5}} {
			dx, dy := x-o[0], y-o[1]
			if dx >= 0 && dx < 5 && dy >= 0 && dy < 5 {
				ring := dx == 0 || dx == 4 || dy == 0 || dy == 4
				core := dx == 2 && dy == 2
				return ring || core
			}
		}
		return false
	}
	inFinderZone := func(x, y int) bool {
		for _, o := range [][2]int{{0, 0}, {n - 6, 0}, {0, n - 6}} {
			if x >= o[0] && x < o[0]+6 && y >= o[1] && y < o[1]+6 {
				return true
			}
		}
		return false
	}
	var b strings.Builder
	b.WriteString(`<svg viewBox="0 0 17 17" class="w-full h-full" fill="currentColor" shape-rendering="crispEdges" aria-hidden="true">`)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			on := false
			if inFinderZone(x, y) {
				on = finder(x, y)
			} else {
				on = (x*7+y*13+x*y*3)%5 < 2
			}
			if on {
				fmt.Fprintf(&b, `<rect x="%d" y="%d" width="1" height="1"/>`, x, y)
			}
		}
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// catChip: kelas warna untuk label kategori.
func catChip(key string) string {
	switch key {
	case "pra-remaja":
		return "bg-lamp-100 text-lamp-700"
	case "remaja":
		return "bg-pine-100 text-pine-700"
	case "mandiri":
		return "bg-pine-800 text-white"
	case "dewasa":
		return "bg-ink text-white"
	case "belum":
		return "bg-clay-100 text-clay-700"
	default:
		return "bg-ink/10 text-ink/70"
	}
}

var bulanPenuh = []string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

// monthLabel: "Mei 2026".
func monthLabel(t time.Time) string {
	return fmt.Sprintf("%s %d", bulanPenuh[t.Month()], t.Year())
}

// itemChip: warna label status kegiatan pada laporan.
func itemChip(key string) string {
	switch key {
	case "selesai":
		return "bg-pine-100 text-pine-700"
	case "tertunda":
		return "bg-clay-100 text-clay-700"
	case "belum":
		return "bg-ink/10 text-ink/70"
	default:
		return "bg-lamp-100 text-lamp-700"
	}
}
