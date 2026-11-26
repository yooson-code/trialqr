package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"presensi-qr/internal/models"
	"presensi-qr/internal/service"
	"presensi-qr/internal/store"
	"presensi-qr/internal/timeutil"
)

// Tepat satu dari token / rfid_uid / member_id yang diisi.
type scanRequest struct {
	Token      string `json:"token"`
	RFID       string `json:"rfid_uid"`
	MemberID   string `json:"member_id"`
	ScheduleID int64  `json:"schedule_id"`
}

func (r scanRequest) method() (method, credential string) {
	switch {
	case strings.TrimSpace(r.RFID) != "":
		return service.MethodRFID, r.RFID
	case strings.TrimSpace(r.MemberID) != "":
		return service.MethodManual, r.MemberID
	default:
		return service.MethodQR, r.Token
	}
}

func memberPayload(m *models.Member) gin.H {
	if m == nil {
		return nil
	}
	return gin.H{
		"name":         m.FullName,
		"phone":        m.Phone,
		"group":        m.GroupName,
		"village":      m.VillageName,
		"photo_url":    m.PhotoURL,
		"initials":     initials(m.FullName),
		"age":          m.Age,
		"category":     m.Category,
		"category_key": m.CategoryKey,
		"marital_label": m.MaritalLabel,
	}
}

// ScanAPI  POST /api/v1/scan
//
//	{"token": "<uuid dari QR>",   "schedule_id": 3}   // scan QR
//	{"rfid_uid": "04A1B2C3",      "schedule_id": 3}   // tap kartu RFID
//	{"member_id": "<uuid>",       "schedule_id": 3}   // absen manual oleh pengurus
//
// 201 success   : presensi tercatat
// 409 duplicate : anggota sudah absen pada tanggal (WIB) yang sama
// 404 invalid   : kode tidak valid / kartu belum terdaftar
// 403 invalid   : anggota nonaktif, atau di luar cakupan pengajian (reason: inactive | scope)
// 400 error     : payload atau jadwal tidak valid
func (h *Handler) ScanAPI(c *gin.Context) {
	var req scanRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ScheduleID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Kode dan jadwal wajib diisi"})
		return
	}
	method, cred := req.method()
	if strings.TrimSpace(cred) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Kode dan jadwal wajib diisi"})
		return
	}

	res, err := h.Scan.Scan(c.Request.Context(), method, cred, req.ScheduleID, userOf(c).ID)

	switch {
	case err == nil:
		c.JSON(http.StatusCreated, gin.H{
			"status":     "success",
			"message":    "Kehadiran tercatat",
			"method":     method,
			"member":     memberPayload(res.Member),
			"scanned_at": res.ScannedAt.In(timeutil.WIB).Format("15:04:05"),
			"scan_date":  res.ScanDate,
		})
	case errors.Is(err, service.ErrDuplicate):
		c.JSON(http.StatusConflict, gin.H{
			"status":     "duplicate",
			"message":    "Sudah absen hari ini",
			"method":     method,
			"member":     memberPayload(res.Member),
			"scanned_at": res.ScannedAt.In(timeutil.WIB).Format("15:04:05"),
			"scan_date":  res.ScanDate,
		})
	case errors.Is(err, service.ErrInvalidToken), errors.Is(err, service.ErrMemberNotFound):
		c.JSON(http.StatusNotFound, gin.H{"status": "invalid", "message": err.Error()})
	case errors.Is(err, service.ErrOutOfScope):
		c.JSON(http.StatusForbidden, gin.H{"status": "invalid", "reason": "scope",
			"message": "Pengajian ini khusus " + res.Scope, "member": memberPayload(res.Member)})
	case errors.Is(err, service.ErrMemberInactive):
		c.JSON(http.StatusForbidden, gin.H{"status": "invalid", "reason": "inactive",
			"message": err.Error(), "member": memberPayload(res.Member)})
	case errors.Is(err, service.ErrScheduleInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
	default:
		log.Printf("scan: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gangguan server, coba lagi"})
	}
}

// MemberSearchAPI  GET /api/v1/members/search?q=nama  (untuk absen manual)
func (h *Handler) MemberSearchAPI(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if len([]rune(q)) < 2 {
		c.JSON(http.StatusOK, gin.H{"members": []gin.H{}})
		return
	}
	list, err := h.Store.ListMembers(c.Request.Context(), store.MemberFilter{Q: q, Limit: 20})
	if err != nil {
		log.Printf("search: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"members": []gin.H{}})
		return
	}
	out := make([]gin.H, 0, 8)
	for i := range list {
		m := &list[i]
		if !m.IsActive || len(out) >= 8 {
			continue
		}
		p := memberPayload(m)
		p["id"] = m.ID
		out = append(out, p)
	}
	c.JSON(http.StatusOK, gin.H{"members": out})
}

func (h *Handler) ScannerPage(c *gin.Context) {
	schedules, err := h.Store.ActiveSchedules(c.Request.Context(), timeutil.TodayStr())
	if err != nil {
		serverErr(c, "/", err)
		return
	}
	h.render(c, "scanner.html", gin.H{"Title": "Scanner", "Active": "scanner", "Schedules": schedules})
}
