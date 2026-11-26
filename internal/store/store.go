package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("data tidak ditemukan")

type Store struct{ db *pgxpool.Pool }

func New(ctx context.Context, dsn string, maxConns int32) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	// Simple protocol kompatibel dengan Supabase pooler (PgBouncer, mode transaction).
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	if maxConns <= 0 {
		maxConns = 10
	}
	cfg.MaxConns = maxConns

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{db: pool}, nil
}

func (s *Store) Close() { s.db.Close() }

func pgCode(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// IsUnique: pelanggaran UNIQUE constraint.
func IsUnique(err error) bool { return pgCode(err) == "23505" }

// IsInUse: baris masih direferensikan tabel lain (foreign key).
func IsInUse(err error) bool { return pgCode(err) == "23503" }

func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

type scanner interface{ Scan(dest ...any) error }
