package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"presensi-qr/internal/models"
	"presensi-qr/internal/store"
)

func (h *Handler) SchedulesPage(c *gin.Context) {
	ctx := c.Request.Context()
	list, err := h.Store.ListSchedules(ctx)
	if err != nil {
		serverErr(c, "/", err)
		return
	}
	views, _ := h.Store.VillageViews(ctx)
	h.render(c, "schedules.html", gin.H{
		"Title": "Jadwal pengajian", "Active": "schedules", "Schedules": list, "Views": views,
	})
}

func (h *Handler) ScheduleCreate(c *gin.Context) {
	n := store.NewSchedule{
		Title:     clean(c.PostForm("title")),
		Date:      c.PostForm("event_date"),
		StartTime: c.PostForm("start_time"),
		Location:  clean(c.PostForm("location")),
		Level:     c.PostForm("level"),
		VillageID: atoi64(c.PostForm("village_id")),
		GroupID:   atoi64(c.PostForm("group_id")),
		CreatedBy: userOf(c).ID,
	}
	switch {
	case n.Title == "":
		fail(c, "/schedules", "Nama pengajian wajib diisi.")
		return
	case !validDate(n.Date):
		fail(c, "/schedules", "Tanggal tidak valid.")
		return
	}
	if n.StartTime != "" {
		if _, err := time.Parse("15:04", n.StartTime); err != nil {
			fail(c, "/schedules", "Jam mulai tidak valid.")
			return
		}
	}
	switch n.Level {
	case models.LevelDaerah:
	case models.LevelDesa:
		if n.VillageID <= 0 {
			fail(c, "/schedules", "Pilih desa untuk pengajian desa.")
			return
		}
	case models.LevelKelompok:
		if n.GroupID <= 0 {
			fail(c, "/schedules", "Pilih kelompok untuk pengajian kelompok.")
			return
		}
	default:
		n.Level = models.LevelDaerah
	}

	if err := h.Store.CreateSchedule(c.Request.Context(), n); err != nil {
		if store.IsInUse(err) { // foreign key: desa/kelompok tidak ada
			fail(c, "/schedules", "Desa atau kelompok yang dipilih tidak ditemukan.")
			return
		}
		serverErr(c, "/schedules", err)
		return
	}
	success(c, "/schedules", "Jadwal \""+n.Title+"\" dibuat dan siap dipakai di scanner.")
}

func (h *Handler) ScheduleToggle(c *gin.Context) {
	active := c.PostForm("active") == "1"
	if err := h.Store.SetScheduleActive(c.Request.Context(), idParam(c), active); err != nil {
		serverErr(c, "/schedules", err)
		return
	}
	if active {
		success(c, "/schedules", "Jadwal diaktifkan.")
	} else {
		success(c, "/schedules", "Jadwal dinonaktifkan dan tidak muncul di scanner.")
	}
}

func (h *Handler) ScheduleDelete(c *gin.Context) {
	if err := h.Store.DeleteSchedule(c.Request.Context(), idParam(c)); err != nil {
		if store.IsInUse(err) {
			fail(c, "/schedules", "Jadwal sudah punya data kehadiran, jadi tidak bisa dihapus. Nonaktifkan saja.")
			return
		}
		serverErr(c, "/schedules", err)
		return
	}
	success(c, "/schedules", "Jadwal dihapus.")
}

