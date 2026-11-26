package store

import (
	"context"

	"presensi-qr/internal/models"
)

const scheduleSelect = `
	select s.id, s.title, s.event_date, coalesce(to_char(s.start_time,'HH24:MI'),''),
	       s.location, s.is_active,
	       (select count(*) from attendances a where a.schedule_id = s.id),
	       s.level, s.village_id, s.group_id,
	       coalesce(v.name, gv.name, ''), coalesce(g.name, '')
	from schedules s
	left join villages v  on v.id  = s.village_id
	left join groups   g  on g.id  = s.group_id
	left join villages gv on gv.id = g.village_id`

func scanSchedule(r scanner) (*models.Schedule, error) {
	var x models.Schedule
	err := r.Scan(&x.ID, &x.Title, &x.EventDate, &x.StartTime, &x.Location, &x.IsActive, &x.AttendCount,
		&x.Level, &x.VillageID, &x.GroupID, &x.VillageName, &x.GroupName)
	if err != nil {
		return nil, mapErr(err)
	}
	x.FillScope()
	return &x, nil
}

func (s *Store) listSchedules(ctx context.Context, where, order string, args ...any) ([]models.Schedule, error) {
	rows, err := s.db.Query(ctx, scheduleSelect+" "+where+" "+order, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Schedule
	for rows.Next() {
		x, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}

func (s *Store) ListSchedules(ctx context.Context) ([]models.Schedule, error) {
	return s.listSchedules(ctx, "", "order by s.event_date desc, s.id desc limit 200")
}

// ActiveSchedules: jadwal aktif, yang terdekat dengan hari ini (WIB) tampil lebih dulu.
func (s *Store) ActiveSchedules(ctx context.Context, today string) ([]models.Schedule, error) {
	return s.listSchedules(ctx, "where s.is_active",
		"order by abs(s.event_date - $1::date), s.start_time nulls last, s.id limit 50", today)
}

func (s *Store) GetSchedule(ctx context.Context, id int64) (*models.Schedule, error) {
	return scanSchedule(s.db.QueryRow(ctx, scheduleSelect+` where s.id = $1`, id))
}

// NewSchedule: villageID dipakai bila Level=desa, groupID bila Level=kelompok.
type NewSchedule struct {
	Title, Date, StartTime, Location, Level string
	VillageID, GroupID                      int64
	CreatedBy                               int64
}

func nullID(n int64) any {
	if n <= 0 {
		return nil
	}
	return n
}

func (s *Store) CreateSchedule(ctx context.Context, n NewSchedule) error {
	var st any
	if n.StartTime != "" {
		st = n.StartTime
	}
	var vid, gid any
	switch n.Level {
	case models.LevelDesa:
		vid = nullID(n.VillageID)
	case models.LevelKelompok:
		gid = nullID(n.GroupID)
	}
	_, err := s.db.Exec(ctx, `
		insert into schedules (title, event_date, start_time, location, level, village_id, group_id, created_by)
		values ($1, $2::date, $3::time, $4, $5, $6, $7, $8)`,
		n.Title, n.Date, st, n.Location, n.Level, vid, gid, n.CreatedBy)
	return err
}

func (s *Store) SetScheduleActive(ctx context.Context, id int64, active bool) error {
	_, err := s.db.Exec(ctx, `update schedules set is_active = $2 where id = $1`, id, active)
	return err
}

func (s *Store) DeleteSchedule(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `delete from schedules where id = $1`, id)
	return err
}
