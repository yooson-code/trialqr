package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"presensi-qr/internal/models"
	"presensi-qr/internal/timeutil"
)

// InsertAttendance mencatat presensi. Pencegahan double-scan dilakukan atomik oleh
// database lewat UNIQUE (member_id, scan_date) + ON CONFLICT DO NOTHING, sehingga
// aman terhadap dua scan yang masuk bersamaan (QR, RFID, maupun manual).
func (s *Store) InsertAttendance(ctx context.Context, memberID string, scheduleID, adminID int64,
	scannedAt time.Time, scanDate, method string) (inserted bool, err error) {

	var id int64
	err = s.db.QueryRow(ctx, `
		insert into attendances (member_id, schedule_id, scanned_by, scanned_at, scan_date, method)
		values ($1::uuid, $2, $3, $4, $5::date, $6)
		on conflict (member_id, scan_date) do nothing
		returning id`, memberID, scheduleID, adminID, scannedAt, scanDate, method).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) AttendanceTime(ctx context.Context, memberID, scanDate string) (time.Time, error) {
	var t time.Time
	err := s.db.QueryRow(ctx,
		`select scanned_at from attendances where member_id = $1::uuid and scan_date = $2::date`,
		memberID, scanDate).Scan(&t)
	return t, mapErr(err)
}

func (s *Store) DeleteAttendance(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `delete from attendances where id = $1`, id)
	return err
}

type AttFilter struct {
	From, To   string // YYYY-MM-DD
	VillageID  int64
	GroupID    int64
	ScheduleID int64
	Category   string
}

func (s *Store) Attendances(ctx context.Context, f AttFilter, limit int) ([]models.Attendance, error) {
	var conds []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if f.From != "" {
		add("a.scan_date >= ?::date", f.From)
	}
	if f.To != "" {
		add("a.scan_date <= ?::date", f.To)
	}
	if f.VillageID > 0 {
		add("v.id = ?", f.VillageID)
	}
	if f.GroupID > 0 {
		add("g.id = ?", f.GroupID)
	}
	if f.ScheduleID > 0 {
		add("a.schedule_id = ?", f.ScheduleID)
	}
	if c := categoryCond(f.Category); c != "" {
		conds = append(conds, c)
	}
	q := `
		select a.id, a.scanned_at, a.scan_date, m.full_name, m.phone, g.name, v.name,
		       coalesce(sc.title,'-'), coalesce(ad.full_name,'-'), a.method, m.birth_date
		from attendances a
		join members m  on m.id = a.member_id
		join groups g   on g.id = m.group_id
		join villages v on v.id = g.village_id
		left join schedules sc on sc.id = a.schedule_id
		left join admins ad    on ad.id = a.scanned_by`
	if len(conds) > 0 {
		q += " where " + strings.Join(conds, " and ")
	}
	if limit <= 0 {
		limit = 5000
	}
	q += fmt.Sprintf(" order by a.scanned_at desc limit %d", limit)

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := timeutil.Now()
	var out []models.Attendance
	for rows.Next() {
		var a models.Attendance
		var birth *time.Time
		if err := rows.Scan(&a.ID, &a.ScannedAt, &a.ScanDate, &a.MemberName, &a.Phone,
			&a.GroupName, &a.VillageName, &a.ScheduleTitle, &a.ScannedBy, &a.Method, &birth); err != nil {
			return nil, err
		}
		_, a.Category, a.CategoryKey = models.Classify(birth, now)
		out = append(out, a)
	}
	return out, rows.Err()
}

// MemberSummaries: jumlah kehadiran tiap anggota aktif pada rentang filter
// (anggota yang tidak pernah hadir tetap muncul dengan total 0).
func (s *Store) MemberSummaries(ctx context.Context, f AttFilter) ([]models.MemberSummary, error) {
	var args []any
	var join, where []string
	addTo := func(dst *[]string, cond string, v any) {
		args = append(args, v)
		*dst = append(*dst, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if f.From != "" {
		addTo(&join, "a.scan_date >= ?::date", f.From)
	}
	if f.To != "" {
		addTo(&join, "a.scan_date <= ?::date", f.To)
	}
	if f.ScheduleID > 0 {
		addTo(&join, "a.schedule_id = ?", f.ScheduleID)
	}
	if f.ScheduleID > 0 { // jangan hitung "tidak hadir" untuk anggota di luar cakupan pengajian
		addTo(&where, `exists (select 1 from schedules sc where sc.id = ? and (sc.level = 'daerah'
			or (sc.level = 'desa' and sc.village_id = v.id) or (sc.level = 'kelompok' and sc.group_id = g.id)))`, f.ScheduleID)
	}
	if f.VillageID > 0 {
		addTo(&where, "v.id = ?", f.VillageID)
	}
	if f.GroupID > 0 {
		addTo(&where, "g.id = ?", f.GroupID)
	}
	if c := categoryCond(f.Category); c != "" {
		where = append(where, c)
	}
	q := `
		select m.id::text, m.full_name, g.name, v.name, count(a.id),
		       m.birth_date, m.parent_name, m.parent_phone
		from members m
		join groups g   on g.id = m.group_id
		join villages v on v.id = g.village_id
		left join attendances a on a.member_id = m.id`
	for _, j := range join {
		q += " and " + j
	}
	q += " where m.is_active"
	for _, w := range where {
		q += " and " + w
	}
	q += " group by m.id, g.name, v.name order by count(a.id) desc, lower(m.full_name) limit 2000"

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := timeutil.Now()
	var out []models.MemberSummary
	for rows.Next() {
		var m models.MemberSummary
		var birth *time.Time
		if err := rows.Scan(&m.ID, &m.FullName, &m.GroupName, &m.VillageName, &m.Total,
			&birth, &m.ParentName, &m.ParentPhone); err != nil {
			return nil, err
		}
		_, m.Category, m.CategoryKey = models.Classify(birth, now)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) Stats(ctx context.Context, today, monthStart string) (models.Stats, error) {
	var st models.Stats
	err := s.db.QueryRow(ctx, `
		select
		  (select count(*) from members   where is_active),
		  (select count(*) from attendances where scan_date = $1::date),
		  (select count(*) from attendances where scan_date >= $2::date and scan_date <= $1::date),
		  (select count(*) from schedules where is_active)`, today, monthStart).
		Scan(&st.ActiveMembers, &st.TodayCount, &st.MonthCount, &st.ActiveSchedules)
	return st, err
}

// DailyCounts: jumlah hadir per tanggal sejak `from` (key: YYYY-MM-DD).
func (s *Store) DailyCounts(ctx context.Context, from string) (map[string]int, error) {
	rows, err := s.db.Query(ctx, `
		select scan_date::text, count(*) from attendances
		where scan_date >= $1::date group by scan_date`, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var d string
		var n int
		if err := rows.Scan(&d, &n); err != nil {
			return nil, err
		}
		out[d] = n
	}
	return out, rows.Err()
}

// CategoryStats: per kategori -> [jumlah anggota aktif, jumlah hadir pada `today`].
func (s *Store) CategoryStats(ctx context.Context, today string) (map[string][2]int, error) {
	var b strings.Builder
	b.WriteString("case when m.birth_date is null then 'belum'")
	for i := len(models.Categories) - 1; i >= 0; i-- {
		c := models.Categories[i]
		fmt.Fprintf(&b, " when %s >= %d then '%s'", ageExpr, c.Min, c.Key)
	}
	b.WriteString(" else 'belum' end")

	rows, err := s.db.Query(ctx, `
		with x as (select m.id, `+b.String()+` as cat from members m where m.is_active)
		select x.cat, count(distinct x.id), count(distinct a.id)
		from x left join attendances a on a.member_id = x.id and a.scan_date = $1::date
		group by x.cat`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][2]int{}
	for rows.Next() {
		var k string
		var total, present int
		if err := rows.Scan(&k, &total, &present); err != nil {
			return nil, err
		}
		out[k] = [2]int{total, present}
	}
	return out, rows.Err()
}
