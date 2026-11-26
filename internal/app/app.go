// Package app merakit seluruh aplikasi (config, database, router) agar bisa dipakai
// oleh server biasa (cmd/server) maupun serverless function (api/index.go di Vercel).
package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"presensi-qr/internal/auth"
	"presensi-qr/internal/config"
	"presensi-qr/internal/router"
	"presensi-qr/internal/storage"
	"presensi-qr/internal/store"
)

// New membuat engine HTTP siap pakai. Panggil closeFn saat berhenti.
func New(ctx context.Context) (engine *gin.Engine, cfg config.Config, closeFn func(), err error) {
	cfg, err = config.Load()
	if err != nil {
		return nil, cfg, nil, fmt.Errorf("konfigurasi: %w", err)
	}
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	st, err := store.New(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		return nil, cfg, nil, fmt.Errorf("koneksi database: %w", err)
	}
	if err := seedAdmin(ctx, st, cfg); err != nil {
		st.Close()
		return nil, cfg, nil, err
	}
	engine, err = router.New(cfg, st, storage.New(cfg))
	if err != nil {
		st.Close()
		return nil, cfg, nil, fmt.Errorf("router: %w", err)
	}
	return engine, cfg, st.Close, nil
}

// seedAdmin membuat akun admin pertama bila tabel admins masih kosong.
func seedAdmin(ctx context.Context, st *store.Store, cfg config.Config) error {
	n, err := st.CountAdmins(ctx)
	if err != nil {
		return fmt.Errorf("cek admin: %w (sudah menjalankan migrations/*.sql?)", err)
	}
	if n > 0 {
		return nil
	}
	if len(cfg.AdminPassword) < 8 {
		return fmt.Errorf("belum ada akun admin: isi ADMIN_USERNAME dan ADMIN_PASSWORD (min. 8 karakter)")
	}
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	username := strings.ToLower(strings.TrimSpace(cfg.AdminUsername))
	if err := st.CreateAdmin(ctx, username, cfg.AdminName, hash, "admin"); err != nil {
		return fmt.Errorf("buat admin: %w", err)
	}
	return nil
}
