// Package service berisi logika bisnis yang tidak bergantung pada HTTP.
package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"presensi-qr/internal/models"
	"presensi-qr/internal/store"
	"presensi-qr/internal/timeutil"
)

// Cara presensi dicatat.
const (
	MethodQR     = "qr"
	MethodRFID   = "rfid"
	MethodManual = "manual"
)

var (
	ErrInvalidToken    = errors.New("Kode tidak valid")
	ErrMemberNotFound  = errors.New("Kartu atau QR belum terdaftar")
	ErrMemberInactive  = errors.New("Anggota ini berstatus nonaktif")
	ErrScheduleInvalid = errors.New("Jadwal tidak ditemukan atau sudah nonaktif")
	ErrDuplicate       = errors.New("Sudah tercatat hadir hari ini")
	ErrOutOfScope      = errors.New("Anggota di luar cakupan pengajian ini")
)

var rfidRe = regexp.MustCompile(`^[0-9A-Z]{4,24}$`)

// NormalizeRFID menyeragamkan UID dari berbagai reader: huruf besar, tanpa spasi/titik dua/strip.
func NormalizeRFID(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	return strings.NewReplacer(" ", "", ":", "", "-", "").Replace(s)
}

func ValidRFID(s string) bool { return rfidRe.MatchString(s) }

type ScanResult struct {
	Scope     string // cakupan jadwal, mis. "Desa Cibiru"
	Member    *models.Member
	ScannedAt time.Time
	ScanDate  string
}

type ScanService struct{ store *store.Store }

func NewScanService(s *store.Store) *ScanService { return &ScanService{store: s} }

// Scan mencatat presensi untuk satu anggota yang diidentifikasi lewat `credential`:
//
//	MethodQR     -> UUID dari QR Code
//	MethodRFID   -> UID kartu RFID
//	MethodManual -> ID anggota (dipilih pengurus dari pencarian)
//
// Untuk ErrDuplicate dan ErrMemberInactive, result tetap terisi agar UI bisa menampilkan siapa orangnya.
func (s *ScanService) Scan(ctx context.Context, method, credential string, scheduleID, adminID int64) (*ScanResult, error) {
	// 1. Validasi format kredensial sebelum menyentuh DB.
	var lookup func() (*models.Member, error)
	switch method {
	case MethodQR, MethodManual:
		id, err := uuid.Parse(strings.TrimSpace(credential))
		if err != nil {
			return nil, ErrInvalidToken
		}
		if method == MethodQR {
			lookup = func() (*models.Member, error) { return s.store.MemberByToken(ctx, id.String()) }
		} else {
			lookup = func() (*models.Member, error) { return s.store.MemberByID(ctx, id.String()) }
		}
	case MethodRFID:
		uid := NormalizeRFID(credential)
		if !ValidRFID(uid) {
			return nil, ErrInvalidToken
		}
		lookup = func() (*models.Member, error) { return s.store.MemberByRFID(ctx, uid) }
	default:
		return nil, ErrInvalidToken
	}

	// 2. Jadwal harus ada dan aktif.
	sch, err := s.store.GetSchedule(ctx, scheduleID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && !sch.IsActive) {
		return nil, ErrScheduleInvalid
	}
	if err != nil {
		return nil, err
	}

	// 3. Anggota harus terdaftar & aktif.
	m, err := lookup()
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	if !m.IsActive {
		return &ScanResult{Member: m}, ErrMemberInactive
	}
	// Pengajian desa/kelompok hanya untuk anggota dari desa/kelompok tersebut.
	if !sch.Covers(m) {
		return &ScanResult{Member: m, Scope: sch.Scope}, ErrOutOfScope
	}

	// 4. Catat. Tanggal dihitung dalam WIB (UTC+7), bukan UTC server.
	now := timeutil.Now()
	date := now.Format(timeutil.DateLayout)

	inserted, err := s.store.InsertAttendance(ctx, m.ID, scheduleID, adminID, now, date, method)
	if err != nil {
		return nil, err
	}
	if !inserted { // (member_id, scan_date) sudah ada -> double scan
		at, _ := s.store.AttendanceTime(ctx, m.ID, date)
		return &ScanResult{Member: m, ScannedAt: at, ScanDate: date}, ErrDuplicate
	}
	return &ScanResult{Member: m, ScannedAt: now, ScanDate: date}, nil
}
