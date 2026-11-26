package handler

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"presensi-qr/internal/models"
	"presensi-qr/internal/store"
	"presensi-qr/internal/timeutil"
)

func validDate(s string) bool {
	_, err := time.Parse(timeutil.DateLayout, s)
	return err == nil
}

// parseAttFilter: default rentang = awal bulan ini s.d. hari ini (WIB).
func parseAttFilter(c *gin.Context) store.AttFilter {
	today := timeutil.TodayStr()
	f := store.AttFilter{
		From:       c.Query("from"),
		To:         c.Query("to"),
		VillageID:  atoi64(c.Query("village_id")),
		GroupID:    atoi64(c.Query("group_id")),
		ScheduleID: atoi64(c.Query("schedule_id")),
		Category:   c.Query("category"),
	}
	if !validCategory(f.Category) {
		f.Category = ""
	}
	if !validDate(f.From) {
		f.From = today[:8] + "01"
	}
	if !validDate(f.To) {
		f.To = today
	}
	if f.From > f.To {
		f.From, f.To = f.To, f.From
	}
	return f
}

func filterValues(f store.AttFilter) url.Values {
	v := url.Values{}
	v.Set("from", f.From)
	v.Set("to", f.To)
	if f.VillageID > 0 {
		v.Set("village_id", strconv.FormatInt(f.VillageID, 10))
	}
	if f.GroupID > 0 {
		v.Set("group_id", strconv.FormatInt(f.GroupID, 10))
	}
	if f.ScheduleID > 0 {
		v.Set("schedule_id", strconv.FormatInt(f.ScheduleID, 10))
	}
	if f.Category != "" {
		v.Set("category", f.Category)
	}
	return v
}

func (h *Handler) AttendancePage(c *gin.Context) {
	ctx := c.Request.Context()
	f := parseAttFilter(c)
	view := c.DefaultQuery("view", "detail")

	data := gin.H{"Title": "Rekap kehadiran", "Active": "attendance", "Filter": f, "View": view}
	if view == "summary" {
		rows, err := h.Store.MemberSummaries(ctx, f)
		if err != nil {
			serverErr(c, "/", err)
			return
		}
		present := 0
		for _, r := range rows {
			if r.Total > 0 {
				present++
			}
		}
		data["Summary"], data["PresentCount"] = rows, present
	} else {
		rows, err := h.Store.Attendances(ctx, f, 0)
		if err != nil {
			serverErr(c, "/", err)
			return
		}
		data["Rows"] = rows
	}
	views, _ := h.Store.VillageViews(ctx)
	schedules, _ := h.Store.ListSchedules(ctx)
	q := filterValues(f).Encode()
	data["Views"], data["Schedules"] = views, schedules
	data["Categories"] = models.Categories
	data["BackURL"] = "/attendance?" + c.Request.URL.RawQuery
	data["CSVURL"] = template.URL("/attendance/export?" + q + "&format=csv")
	data["XLSXURL"] = template.URL("/attendance/export?" + q + "&format=xlsx")
	h.render(c, "attendance.html", data)
}

var exportHeader = []string{"No", "Tanggal", "Jam (WIB)", "Nama", "Kategori", "No HP", "Desa", "Kelompok", "Jadwal", "Metode", "Dicatat oleh"}

var methodLabels = map[string]string{"qr": "QR", "rfid": "RFID", "manual": "Manual"}

// AttendanceExport  GET /attendance/export?format=csv|xlsx&from=&to=&...
func (h *Handler) AttendanceExport(c *gin.Context) {
	f := parseAttFilter(c)
	rows, err := h.Store.Attendances(c.Request.Context(), f, 0)
	if err != nil {
		c.String(http.StatusInternalServerError, "Gagal mengambil data")
		return
	}
	base := fmt.Sprintf("kehadiran_%s_sd_%s", f.From, f.To)

	line := func(i int) []string {
		a := rows[i]
		return []string{
			strconv.Itoa(i + 1), a.ScanDate.Format(timeutil.DateLayout), a.ScannedAt.In(timeutil.WIB).Format("15:04:05"),
			a.MemberName, a.Category, a.Phone, a.VillageName, a.GroupName, a.ScheduleTitle,
			methodLabels[a.Method], a.ScannedBy,
		}
	}

	if c.Query("format") == "xlsx" {
		x := excelize.NewFile()
		defer x.Close()
		sheet := "Kehadiran"
		x.SetSheetName("Sheet1", sheet)
		for i, title := range exportHeader {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			x.SetCellValue(sheet, cell, title)
		}
		bold, _ := x.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
			Fill: excelize.Fill{Type: "pattern", Color: []string{"173B34"}, Pattern: 1},
		})
		x.SetCellStyle(sheet, "A1", "K1", bold)
		for i := range rows {
			for j, v := range line(i) {
				cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
				x.SetCellValue(sheet, cell, v)
			}
		}
		for col, w := range []float64{5, 12, 11, 28, 14, 16, 18, 18, 28, 10, 20} {
			name, _ := excelize.ColumnNumberToName(col + 1)
			x.SetColWidth(sheet, name, name, w)
		}
		x.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})

		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", `attachment; filename="`+base+`.xlsx"`)
		_ = x.Write(c.Writer)
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+base+`.csv"`)
	c.Writer.Write([]byte("\xEF\xBB\xBF")) // BOM agar Excel membaca UTF-8 dengan benar
	w := csv.NewWriter(c.Writer)
	_ = w.Write(exportHeader)
	for i := range rows {
		_ = w.Write(line(i))
	}
	w.Flush()
}

// AttendanceDelete membatalkan satu catatan kehadiran (mis. salah scan / salah orang).
func (h *Handler) AttendanceDelete(c *gin.Context) {
	back := c.PostForm("back")
	if !strings.HasPrefix(back, "/attendance") || strings.HasPrefix(back, "//") {
		back = "/attendance"
	}
	if err := h.Store.DeleteAttendance(c.Request.Context(), idParam(c)); err != nil {
		serverErr(c, back, err)
		return
	}
	success(c, back, "Kehadiran dibatalkan.")
}
