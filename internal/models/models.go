package models

import "time"

type Admin struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	FullName     string    `json:"full_name"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

func (a Admin) IsAdmin() bool { return a.Role == "admin" }

type Village struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	GroupCount int    `json:"group_count"`
}

type Group struct {
	ID          int64  `json:"id"`
	VillageID   int64  `json:"village_id"`
	Name        string `json:"name"`
	VillageName string `json:"village_name"`
	MemberCount int    `json:"member_count"`
}

// VillageView dipakai untuk menampilkan desa beserta kelompok-kelompoknya.
type VillageView struct {
	Village
	Groups []Group `json:"groups"`
}

type Member struct {
	ID          string    `json:"id"`
	FullName    string    `json:"full_name"`
	Phone       string    `json:"phone"`
	GroupID     int64     `json:"group_id"`
	GroupName   string    `json:"group_name"`
	VillageID   int64     `json:"village_id"`
	VillageName string    `json:"village_name"`
	PhotoURL    *string   `json:"photo_url"`
	Token       string    `json:"token"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`

	BirthDate   *time.Time `json:"birth_date"`
	ParentName  string     `json:"parent_name"`
	ParentPhone string     `json:"parent_phone"`
	RFID        *string    `json:"rfid_uid"`

	// khusus kategori Usia Mandiri (19-30): "belum" | "menikah"
	MaritalStatus *string `json:"marital_status"`
	MaritalLabel  string  `json:"marital_label"`

	// turunan dari BirthDate, diisi lewat Fill()
	Age         *int   `json:"age"`
	Category    string `json:"category"`
	CategoryKey string `json:"category_key"`
}

// Cakupan pengajian.
const (
	LevelDaerah   = "daerah"
	LevelDesa     = "desa"
	LevelKelompok = "kelompok"
)

type Schedule struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	EventDate   time.Time `json:"event_date"`
	StartTime   string    `json:"start_time"`
	Location    string    `json:"location"`
	IsActive    bool      `json:"is_active"`
	AttendCount int       `json:"attend_count"`

	Level       string `json:"level"`
	VillageID   *int64 `json:"village_id"`
	GroupID     *int64 `json:"group_id"`
	VillageName string `json:"village_name"`
	GroupName   string `json:"group_name"`
	Scope       string `json:"scope"` // "Daerah", "Desa X", "Kelompok Y, Desa X"
}

// ScopeLabel menormalkan level dan menyusun label cakupan yang mudah dibaca.
func ScopeLabel(level, village, group string) (string, string) {
	switch level {
	case LevelDesa:
		return level, "Desa " + village
	case LevelKelompok:
		return level, "Kelompok " + group + ", Desa " + village
	}
	return LevelDaerah, "Daerah"
}

func (s *Schedule) FillScope() {
	s.Level, s.Scope = ScopeLabel(s.Level, s.VillageName, s.GroupName)
}

// Covers: apakah anggota termasuk cakupan pengajian ini.
func (s *Schedule) Covers(m *Member) bool {
	switch s.Level {
	case LevelDesa:
		return s.VillageID != nil && *s.VillageID == m.VillageID
	case LevelKelompok:
		return s.GroupID != nil && *s.GroupID == m.GroupID
	}
	return true
}

type Attendance struct {
	ID            int64
	ScannedAt     time.Time
	ScanDate      time.Time
	MemberName    string
	Phone         string
	GroupName     string
	VillageName   string
	ScheduleTitle string
	ScannedBy     string
	Method        string // qr | rfid | manual
	Category      string
	CategoryKey   string
}

type MemberSummary struct {
	ID          string
	FullName    string
	GroupName   string
	VillageName string
	Total       int
	Category    string
	CategoryKey string
	ParentName  string
	ParentPhone string
}

type Stats struct {
	ActiveMembers   int
	TodayCount      int
	MonthCount      int
	ActiveSchedules int
}

// ---------- Laporan pra-musyawarah bulanan ----------

const (
	ReportDraft     = "draft"
	ReportSubmitted = "submitted"
)

// Status kegiatan pada laporan.
var ItemStatuses = []struct{ Key, Label string }{
	{"selesai", "Selesai"},
	{"berjalan", "Berjalan"},
	{"tertunda", "Tertunda"},
	{"belum", "Belum dimulai"},
}

func ItemStatusLabel(key string) string {
	for _, s := range ItemStatuses {
		if s.Key == key {
			return s.Label
		}
	}
	return "Berjalan"
}

type ReportItem struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Achievement string `json:"achievement"`
	Obstacle    string `json:"obstacle"`
	Followup    string `json:"followup"`
}

type Report struct {
	ID          int64
	Period      time.Time // tanggal 1 bulan yang dilaporkan
	AuthorID    int64
	AuthorName  string
	Bidang      string
	Level       string
	VillageID   *int64
	GroupID     *int64
	VillageName string
	GroupName   string
	Scope       string
	Summary     string
	Items       []ReportItem
	AttendanceNote string
	Evaluation     string
	BudgetNeeds    string
	Topics         string
	Status      string
	SubmittedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r *Report) FillScope() {
	r.Level, r.Scope = ScopeLabel(r.Level, r.VillageName, r.GroupName)
}

func (r Report) IsSubmitted() bool { return r.Status == ReportSubmitted }

// ItemsDone: jumlah kegiatan berstatus selesai.
func (r Report) ItemsDone() int {
	n := 0
	for _, it := range r.Items {
		if it.Status == "selesai" {
			n++
		}
	}
	return n
}

// ItemsPct: persentase kegiatan selesai (0-100).
func (r Report) ItemsPct() int {
	if len(r.Items) == 0 {
		return 0
	}
	return r.ItemsDone() * 100 / len(r.Items)
}

// Rekap kehadiran otomatis untuk satu periode (dan cakupan) laporan.
type SessionStat struct {
	Title string
	Date  time.Time
	Scope string
	Count int
}

type CategoryStat struct {
	Key, Label     string
	Active, Present int
	Pct            int
}

type AttendanceSummary struct {
	ActiveMembers    int
	TotalAttendances int
	UniquePresent    int
	PresentPct       int
	Sessions         int
	AvgPerSession    float64
	NeverPresent     int
	BySession        []SessionStat
	ByCategory       []CategoryStat
}
