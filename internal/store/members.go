package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"presensi-qr/internal/models"
	"presensi-qr/internal/timeutil"
)

// ageExpr: umur penuh (tahun) berdasarkan tanggal hari ini di WIB.
const ageExpr = `extract(year from age((now() at time zone 'Asia/Jakarta')::date, m.birth_date))`

// categoryCond menghasilkan kondisi SQL untuk satu kategori; "" bila key tidak dikenal.
func categoryCond(key string) string {
	if key == models.CategoryUnknown {
		return "m.birth_date is null"
	}
	for _, c := range models.Categories {
		if c.Key == key {
			return fmt.Sprintf("%s between %d and %d", ageExpr, c.Min, c.Max)
		}
	}
	return ""
}

const memberSelect = `
	select m.id::text, m.full_name, m.phone, m.group_id, g.name, v.id, v.name,
	       m.photo_url, m.qr_code_token::text, m.is_active, m.created_at,
	       m.birth_date, m.parent_name, m.parent_phone, m.rfid_uid, m.marital_status
	from members m
	join groups g   on g.id = m.group_id
	join villages v on v.id = g.village_id`

func scanMember(r scanner) (*models.Member, error) {
	var m models.Member
	err := r.Scan(&m.ID, &m.FullName, &m.Phone, &m.GroupID, &m.GroupName, &m.VillageID,
		&m.VillageName, &m.PhotoURL, &m.Token, &m.IsActive, &m.CreatedAt,
		&m.BirthDate, &m.ParentName, &m.ParentPhone, &m.RFID, &m.MaritalStatus)
	if err != nil {
		return nil, mapErr(err)
	}
	m.Fill(timeutil.Now())
	return &m, nil
}

type MemberFilter struct {
	Q         string
	VillageID int64
	GroupID   int64
	Category  string
	Marital   string // "belum" | "menikah" (relevan untuk Usia Mandiri)
	Limit     int
}

func (s *Store) ListMembers(ctx context.Context, f MemberFilter) ([]models.Member, error) {
	var conds []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		args = append(args, "%"+q+"%")
		n := len(args)
		conds = append(conds, fmt.Sprintf(
			"(m.full_name ilike $%d or m.phone ilike $%d or m.parent_name ilike $%d or m.rfid_uid ilike $%d)", n, n, n, n))
	}
	if f.VillageID > 0 {
		add("v.id = ?", f.VillageID)
	}
	if f.GroupID > 0 {
		add("g.id = ?", f.GroupID)
	}
	if c := categoryCond(f.Category); c != "" {
		conds = append(conds, c)
	}
	if f.Marital == models.MaritalSingle || f.Marital == models.MaritalMarried {
		add("m.marital_status = ?", f.Marital)
	}
	q := memberSelect
	if len(conds) > 0 {
		q += " where " + strings.Join(conds, " and ")
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 500
	}
	q += fmt.Sprintf(" order by lower(m.full_name) limit %d", limit)

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Member
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (s *Store) MemberByID(ctx context.Context, id string) (*models.Member, error) {
	return scanMember(s.db.QueryRow(ctx, memberSelect+` where m.id = $1::uuid`, id))
}

func (s *Store) MemberByToken(ctx context.Context, token string) (*models.Member, error) {
	return scanMember(s.db.QueryRow(ctx, memberSelect+` where m.qr_code_token = $1::uuid`, token))
}

func (s *Store) MemberByRFID(ctx context.Context, uid string) (*models.Member, error) {
	return scanMember(s.db.QueryRow(ctx, memberSelect+` where m.rfid_uid = $1`, uid))
}

func birthArg(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(timeutil.DateLayout)
}

func maritalArg(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func rfidArg(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *Store) CreateMember(ctx context.Context, m *models.Member) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `
		insert into members (full_name, phone, group_id, photo_url, is_active,
		                     birth_date, parent_name, parent_phone, rfid_uid, marital_status)
		values ($1,$2,$3,$4,$5,$6::date,$7,$8,nullif($9,''),nullif($10,''))
		returning id::text`,
		m.FullName, m.Phone, m.GroupID, m.PhotoURL, m.IsActive,
		birthArg(m.BirthDate), m.ParentName, m.ParentPhone, rfidArg(m.RFID), maritalArg(m.MaritalStatus)).Scan(&id)
	return id, err
}

// UpdateMember: photo_url hanya diganti bila m.PhotoURL tidak nil; RFID kosong = dilepas.
func (s *Store) UpdateMember(ctx context.Context, m *models.Member) error {
	_, err := s.db.Exec(ctx, `
		update members set full_name=$2, phone=$3, group_id=$4,
		       photo_url = coalesce($5::text, photo_url), is_active=$6,
		       birth_date=$7::date, parent_name=$8, parent_phone=$9, rfid_uid=nullif($10,''),
		       marital_status=nullif($11,''), updated_at=now()
		where id = $1::uuid`,
		m.ID, m.FullName, m.Phone, m.GroupID, m.PhotoURL, m.IsActive,
		birthArg(m.BirthDate), m.ParentName, m.ParentPhone, rfidArg(m.RFID), maritalArg(m.MaritalStatus))
	return err
}

func (s *Store) DeleteMember(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `delete from members where id = $1::uuid`, id)
	return err
}
