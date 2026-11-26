package store

import (
	"context"

	"presensi-qr/internal/models"
)

const adminCols = `id, username, full_name, password_hash, role, is_active, created_at`

func scanAdmin(r scanner) (*models.Admin, error) {
	var a models.Admin
	if err := r.Scan(&a.ID, &a.Username, &a.FullName, &a.PasswordHash, &a.Role, &a.IsActive, &a.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	return &a, nil
}

func (s *Store) CountAdmins(ctx context.Context) (n int, err error) {
	err = s.db.QueryRow(ctx, `select count(*) from admins`).Scan(&n)
	return
}

func (s *Store) CreateAdmin(ctx context.Context, username, fullName, hash, role string) error {
	_, err := s.db.Exec(ctx,
		`insert into admins (username, full_name, password_hash, role) values ($1,$2,$3,$4)`,
		username, fullName, hash, role)
	return err
}

func (s *Store) AdminByUsername(ctx context.Context, username string) (*models.Admin, error) {
	return scanAdmin(s.db.QueryRow(ctx, `select `+adminCols+` from admins where username = $1`, username))
}

func (s *Store) AdminByID(ctx context.Context, id int64) (*models.Admin, error) {
	return scanAdmin(s.db.QueryRow(ctx, `select `+adminCols+` from admins where id = $1`, id))
}

func (s *Store) ListAdmins(ctx context.Context) ([]models.Admin, error) {
	rows, err := s.db.Query(ctx, `select `+adminCols+` from admins order by role, full_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Admin
	for rows.Next() {
		a, err := scanAdmin(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (s *Store) SetAdminActive(ctx context.Context, id int64, active bool) error {
	_, err := s.db.Exec(ctx, `update admins set is_active = $2 where id = $1`, id, active)
	return err
}

func (s *Store) SetAdminPassword(ctx context.Context, id int64, hash string) error {
	_, err := s.db.Exec(ctx, `update admins set password_hash = $2 where id = $1`, id, hash)
	return err
}

func (s *Store) DeleteAdmin(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `delete from admins where id = $1`, id)
	return err
}
