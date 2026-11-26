// Package storage menyimpan foto anggota: ke Supabase Storage bila dikonfigurasi,
// atau ke folder lokal sebagai cadangan.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"presensi-qr/internal/config"
)

const maxPhoto = 2 << 20 // 2 MB

type Storage struct {
	url, key, bucket, dir string
	client                *http.Client
}

func New(c config.Config) *Storage {
	return &Storage{
		url:    strings.TrimRight(c.SupabaseURL, "/"),
		key:    c.SupabaseKey,
		bucket: c.SupabaseBucket,
		dir:    c.UploadDir,
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *Storage) remote() bool { return s.url != "" && s.key != "" }

func (s *Storage) LocalDir() string { return s.dir }

// SavePhoto memvalidasi (ukuran + isi file sebenarnya) lalu menyimpan, mengembalikan URL publik.
func (s *Storage) SavePhoto(ctx context.Context, fh *multipart.FileHeader) (string, error) {
	if fh.Size > maxPhoto {
		return "", errors.New("ukuran foto maksimal 2 MB")
	}
	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxPhoto+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxPhoto {
		return "", errors.New("ukuran foto maksimal 2 MB")
	}

	ct := http.DetectContentType(data)
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[ct]
	if ext == "" {
		return "", errors.New("format foto harus JPG, PNG, atau WebP")
	}
	name := uuid.NewString() + ext

	if s.remote() {
		return s.upload(ctx, name, ct, data)
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(s.dir, name), data, 0o644); err != nil {
		return "", err
	}
	return "/uploads/" + name, nil
}

func (s *Storage) upload(ctx context.Context, name, ct string, data []byte) (string, error) {
	endpoint := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.url, s.bucket, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("apikey", s.key)
	req.Header.Set("Content-Type", ct)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return "", fmt.Errorf("upload ke Supabase gagal (%d): %s", resp.StatusCode, body)
	}
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.url, s.bucket, name), nil
}
