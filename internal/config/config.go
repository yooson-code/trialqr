package config

import (
	"errors"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	CookieSecure   bool
	AdminUsername  string
	AdminPassword  string
	AdminName      string
	SupabaseURL    string
	SupabaseKey    string
	SupabaseBucket string
	UploadDir      string
	DBMaxConns     int32  // koneksi maks. per instance (di Vercel pakai kecil, mis. 3)
	TrustedIPHeader string // mis. "X-Real-Ip" di Vercel, agar rate-limit memakai IP pengunjung asli
}

func Load() (Config, error) {
	_ = godotenv.Load() // .env opsional

	c := Config{
		Port:           get("PORT", "8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AdminUsername:  get("ADMIN_USERNAME", "admin"),
		AdminPassword:  os.Getenv("ADMIN_PASSWORD"),
		AdminName:      get("ADMIN_NAME", "Administrator"),
		SupabaseURL:    os.Getenv("SUPABASE_URL"),
		SupabaseKey:    os.Getenv("SUPABASE_SERVICE_KEY"),
		SupabaseBucket: get("SUPABASE_BUCKET", "member-photos"),
		UploadDir:      get("UPLOAD_DIR", "uploads"),
	}
	c.CookieSecure, _ = strconv.ParseBool(os.Getenv("COOKIE_SECURE"))
	c.TrustedIPHeader = os.Getenv("TRUSTED_IP_HEADER")
	c.DBMaxConns = 10
	if n, err := strconv.Atoi(os.Getenv("DB_MAX_CONNS")); err == nil && n > 0 && n <= 50 {
		c.DBMaxConns = int32(n)
	}

	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL belum diisi")
	}
	if len(c.JWTSecret) < 16 {
		return c, errors.New("JWT_SECRET minimal 16 karakter")
	}
	return c, nil
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
