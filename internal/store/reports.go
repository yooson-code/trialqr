package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"presensi-qr/internal/models"
	"presensi-qr/internal/timeutil"
)

const reportSelect = `
	select r.id, r.period, coalesce(r.author_id, 0), r.author_name, r.bidang, r.level, r.village_id, r.group_id,
	       coalesce(v.name, gv.name, ''), coalesce(g.name, ''),
	       r.summary, r.items::text, r.attendance_note, r.evaluation, r.budget_needs, r.topics,
	       r.status, r.submitted_at, r.created_at, r.updated_at
	from monthly_reports r
	left join villages v  on v.id  = r.village_id
	left join groups   g  on g.id  = r.group_id
	left join villages gv on gv.id = g.village_id`

func scanReport(r scanner) (*models.Report, error) {
	var x models.Report
	var items string
	err := r.Scan(&x.ID, &x.Period, &x.AuthorID, &x.AuthorName, &x.Bidang, &x.Level, &x.VillageID, &x.GroupID,
		&x.VillageName, &x.GroupName, &x.Summary, &items, &x.AttendanceNote, &x.Evaluation, &x.BudgetNeeds, &x.Topics,
		&x.Status, &x.SubmittedAt, &x.CreatedAt, &x.UpdatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	_ = json.Unmarshal([]byte(items), &x.Items)
	x.FillScope()
	return &x, nil
}

func (s *Store) GetReport(ctx context.Context, id int64) (*models.Report, error) {
	return scanReport(s.db.QueryRow(ctx, reportSelect+` where r.id = $1`, id))
}

// ReportFilter: Period berformat YYYY-MM-DD (tanggal 1) atau kosong untuk semua periode.
// Laporan draft hanya terlihat oleh penulisnya dan admin.
type ReportFilter struct {
	Period      string
	ViewerID    int64
	ViewerAdmin bool
	OnlySubmitted bool
}

func (s *Store) ListReports(ctx context.Context, f ReportFilter) ([]models.Report, error) {
	var conds []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if f.Period != "" {
		add("r.period = ?::date", f.Period)
	}
	switch {
	case f.OnlySubmitted:
		conds = append(conds, "r.status = 'submitted'")
	case !f.ViewerAdmin:
		add("(r.status = 'submitted' or r.author_id = ?)", f.ViewerID)
	}
	q := reportSelect
	if len(conds) > 0 {
		q += " where " + strings.Join(conds, " and ")
	}
	q += " order by r.period desc, lower(r.bidang), lower(r.author_name) limit 500"

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Report
	for rows.Next() {
		r, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// ReportOf mengembalikan (id, status) laporan milik pengurus pada periode; id 0 bila belum ada.
func (s *Store) ReportOf(ctx context.Context, authorID int64, period string) (int64, string, error) {
	var id int64
	var status string
	err := s.db.QueryRow(ctx,
		`select id, status from monthly_reports where author_id = $1 and period = $2::date`, authorID, period).Scan(&id, &status)
	if err != nil {
		if e := mapErr(err); e == ErrNotFound {
			return 0, "", nil
		}
		return 0, "", err
	}
	return id, status, nil
}

func reportScopeArgs(r *models.Report) (vid, gid any) {
	switch r.Level {
	case models.LevelDesa:
		if r.VillageID != nil {
			vid = nullID(*r.VillageID)
		}
	case models.LevelKelompok:
		if r.GroupID != nil {
			gid = nullID(*r.GroupID)
		}
	}
	return
}

func (s *Store) CreateReport(ctx context.Context, r *models.Report) (int64, error) {
	items, _ := json.Marshal(r.Items)
	vid, gid := reportScopeArgs(r)
	var id int64
	err := s.db.QueryRow(ctx, `
		insert into monthly_reports (period, author_id, author_name, bidang, level, village_id, group_id,
		       summary, items, attendance_note, evaluation, budget_needs, topics, status, submitted_at)
		values ($1::date, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, $12, $13, $14,
		        case when $14 = 'submitted' then now() else null end)
		returning id`,
		r.Period.Format(timeutil.DateLayout), r.AuthorID, r.AuthorName, r.Bidang, r.Level, vid, gid,
		r.Summary, string(items), r.AttendanceNote, r.Evaluation, r.BudgetNeeds, r.Topics, r.Status).Scan(&id)
	return id, err
}

// UpdateReport mengubah isi laporan (tidak mengubah status; pakai SetReportStatus).
func (s *Store) UpdateReport(ctx context.Context, r *models.Report) error {
	items, _ := json.Marshal(r.Items)
	vid, gid := reportScopeArgs(r)
	_, err := s.db.Exec(ctx, `
		update monthly_reports set period=$2::date, bidang=$3, level=$4, village_id=$5, group_id=$6,
		       summary=$7, items=$8::jsonb, attendance_note=$9, evaluation=$10, budget_needs=$11, topics=$12,
		       updated_at=now()
		where id = $1`,
		r.ID, r.Period.Format(timeutil.DateLayout), r.Bidang, r.Level, vid, gid,
		r.Summary, string(items), r.AttendanceNote, r.Evaluation, r.BudgetNeeds, r.Topics)
	return err
}

func (s *Store) SetReportStatus(ctx context.Context, id int64, status string) error {
	_, err := s.db.Exec(ctx, `
		update monthly_reports set status = $2,
		       submitted_at = case when $2 = 'submitted' then now() else null end,
		       updated_at = now()
		where id = $1`, id, status)
	return err
}

func (s *Store) DeleteReport(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `delete from monthly_reports where id = $1`, id)
	return err
}

// ---------- Rekap kehadiran otomatis untuk laporan ----------

// scopeSQL menambahkan kondisi desa/kelompok (alias v dan g) dan mengembalikan args baru.
func scopeSQL(level string, villageID, groupID int64, args []any) (string, []any) {
	switch level {
	case models.LevelDesa:
		if villageID > 0 {
			args = append(args, villageID)
			return fmt.Sprintf(" and v.id = $%d", len(args)), args
		}
	case models.LevelKelompok:
		if groupID > 0 {
			args = append(args, groupID)
			return fmt.Sprintf(" and g.id = $%d", len(args)), args
		}
	}
	return "", args
}

const memberJoin = ` join groups g on g.id = m.group_id join villages v on v.id = g.village_id`

// AttendanceSummary merangkum kehadiran pada rentang [from, to] (YYYY-MM-DD) untuk cakupan tertentu.
func (s *Store) AttendanceSummary(ctx context.Context, from, to, level string, villageID, groupID int64) (models.AttendanceSummary, error) {
	var out models.AttendanceSummary
	now := timeutil.Now()

	// 1. total kehadiran, anggota unik, jumlah pengajian
	args := []any{from, to}
	sc, args := scopeSQL(level, villageID, groupID, args)
	var sessions int
	err := s.db.QueryRow(ctx, `
		select count(a.id), count(distinct a.member_id), count(distinct a.schedule_id)
		from attendances a join members m on m.id = a.member_id`+memberJoin+`
		where a.scan_date between $1::date and $2::date`+sc, args...).
		Scan(&out.TotalAttendances, &out.UniquePresent, &sessions)
	if err != nil {
		return out, err
	}
	out.Sessions = sessions
	if sessions > 0 {
		out.AvgPerSession = float64(out.TotalAttendances) / float64(sessions)
	}

	// 2. anggota aktif per kategori (berdasarkan tanggal lahir)
	active := map[string]int{}
	args2 := []any{}
	sc2, args2 := scopeSQL(level, villageID, groupID, args2)
	rows, err := s.db.Query(ctx, `
		select m.birth_date, count(*) from members m`+memberJoin+`
		where m.is_active`+sc2+` group by m.birth_date`, args2...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var b *time.Time
		var n int
		if err := rows.Scan(&b, &n); err != nil {
			rows.Close()
			return out, err
		}
		_, _, k := models.Classify(b, now)
		active[k] += n
		out.ActiveMembers += n
	}
	rows.Close()

	// 3. anggota unik yang hadir per kategori
	present := map[string]int{}
	args3 := []any{from, to}
	sc3, args3 := scopeSQL(level, villageID, groupID, args3)
	rows, err = s.db.Query(ctx, `
		select m.birth_date, count(distinct m.id)
		from attendances a join members m on m.id = a.member_id`+memberJoin+`
		where a.scan_date between $1::date and $2::date`+sc3+` group by m.birth_date`, args3...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var b *time.Time
		var n int
		if err := rows.Scan(&b, &n); err != nil {
			rows.Close()
			return out, err
		}
		_, _, k := models.Classify(b, now)
		present[k] += n
	}
	rows.Close()

	addCat := func(key, label string) {
		a, p := active[key], present[key]
		if a == 0 && p == 0 {
			return
		}
		pct := 0
		if a > 0 {
			pct = p * 100 / a
			if pct > 100 {
				pct = 100
			}
		}
		out.ByCategory = append(out.ByCategory, models.CategoryStat{Key: key, Label: label, Active: a, Present: p, Pct: pct})
	}
	for _, c := range models.Categories {
		addCat(c.Key, c.Label)
	}
	addCat(models.CategoryUnknown, "Belum diisi")

	if out.ActiveMembers > 0 {
		out.PresentPct = out.UniquePresent * 100 / out.ActiveMembers
		if out.PresentPct > 100 {
			out.PresentPct = 100
		}
	}

	// 4. anggota aktif yang belum pernah hadir pada periode
	args4 := []any{}
	sc4, args4 := scopeSQL(level, villageID, groupID, args4)
	args4 = append(args4, from, to)
	n := len(args4)
	err = s.db.QueryRow(ctx, fmt.Sprintf(`
		select count(*) from members m`+memberJoin+`
		where m.is_active`+sc4+` and not exists (
		  select 1 from attendances a where a.member_id = m.id
		  and a.scan_date between $%d::date and $%d::date)`, n-1, n), args4...).Scan(&out.NeverPresent)
	if err != nil {
		return out, err
	}

	// 5. rincian per pengajian
	args5 := []any{from, to}
	sc5, args5 := scopeSQL(level, villageID, groupID, args5)
	rows, err = s.db.Query(ctx, `
		select sc.title, sc.event_date, sc.level, coalesce(sv.name, gv.name, ''), coalesce(sg.name, ''), count(a.id)
		from attendances a
		join members m on m.id = a.member_id`+memberJoin+`
		join schedules sc on sc.id = a.schedule_id
		left join villages sv on sv.id = sc.village_id
		left join groups   sg on sg.id = sc.group_id
		left join villages gv on gv.id = sg.village_id
		where a.scan_date between $1::date and $2::date`+sc5+`
		group by sc.id, sc.title, sc.event_date, sc.level, sv.name, gv.name, sg.name
		order by sc.event_date, sc.id`, args5...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var st models.SessionStat
		var lvl, vn, gn string
		if err := rows.Scan(&st.Title, &st.Date, &lvl, &vn, &gn, &st.Count); err != nil {
			return out, err
		}
		_, st.Scope = models.ScopeLabel(lvl, vn, gn)
		out.BySession = append(out.BySession, st)
	}
	return out, rows.Err()
}
