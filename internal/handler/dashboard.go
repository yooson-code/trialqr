package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"presensi-qr/internal/models"
	"presensi-qr/internal/store"
	"presensi-qr/internal/timeutil"
)

type catRow struct {
	Key, Label     string
	Total, Present int
	Pct            int
}

type dayBar struct {
	Day, Label string
	Count, Pct int
	IsToday    bool
}

func (h *Handler) Dashboard(c *gin.Context) {
	ctx := c.Request.Context()
	now := timeutil.Now()
	today := now.Format(timeutil.DateLayout)
	monthStart := now.Format("2006-01") + "-01"

	stats, err := h.Store.Stats(ctx, today, monthStart)
	if err != nil {
		serverErr(c, "/login", err)
		return
	}
	from := now.AddDate(0, 0, -6).Format(timeutil.DateLayout)
	counts, _ := h.Store.DailyCounts(ctx, from)

	bars := make([]dayBar, 0, 7)
	max := 1
	for i := 6; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		key := d.Format(timeutil.DateLayout)
		n := counts[key]
		if n > max {
			max = n
		}
		bars = append(bars, dayBar{Day: d.Format("02"), Label: hari[d.Weekday()], Count: n, IsToday: i == 0})
	}
	for i := range bars {
		bars[i].Pct = bars[i].Count * 100 / max
		if bars[i].Count > 0 && bars[i].Pct < 6 {
			bars[i].Pct = 6
		}
	}

	recent, _ := h.Store.Attendances(ctx, store.AttFilter{From: today, To: today}, 8)

	cs, _ := h.Store.CategoryStats(ctx, today)
	var cats []catRow
	addCat := func(key, label string) {
		v := cs[key]
		if v[0] == 0 {
			return
		}
		cats = append(cats, catRow{Key: key, Label: label, Total: v[0], Present: v[1], Pct: v[1] * 100 / v[0]})
	}
	for _, c := range models.Categories {
		addCat(c.Key, c.Label)
	}
	addCat(models.CategoryUnknown, "Belum diisi")

	// pengingat laporan musyawarah bulan ini
	period := currentMonth()
	reportID, reportStatus, _ := h.Store.ReportOf(ctx, userOf(c).ID, period.Format(timeutil.DateLayout))

	pct := 0
	if stats.ActiveMembers > 0 {
		pct = stats.TodayCount * 100 / stats.ActiveMembers
	}
	h.render(c, "dashboard.html", gin.H{
		"Title": "Dashboard", "Active": "dashboard",
		"Stats": stats, "Bars": bars, "Cats": cats,
		"ReportID": reportID, "ReportStatus": reportStatus, "ReportPeriod": period, "Recent": recent, "Pct": pct,
		"NowDate": now.Format("2006-01-02"), "Now": now,
		"Greeting": greeting(now),
	})
}

func greeting(t time.Time) string {
	switch h := t.Hour(); {
	case h < 11:
		return "Selamat pagi"
	case h < 15:
		return "Selamat siang"
	case h < 18:
		return "Selamat sore"
	default:
		return "Selamat malam"
	}
}
