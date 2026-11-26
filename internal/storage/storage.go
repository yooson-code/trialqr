// Package storage menyimpan foto anggota: ke Supabase Storage bila dikonfigurasi,
// atau ke folder lokal sebagai cadangan.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // mendaftarkan decoder PNG
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // mendaftarkan decoder WebP

	"presensi-qr/internal/config"
)

const (
	maxUpload = 10 << 20 // foto mentah dari kamera HP boleh sampai 10 MB; hasil akhir jauh lebih kecil
	maxDim    = 800      // sisi terpanjang hasil kompres (px)
	jpegQ     = 80
	maxPixels = 25_000_000
)

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

// compress menormalkan foto: ubah ukuran (maks. 800 px), simpan sebagai JPEG kualitas 80,
// dan otomatis membuang metadata EXIF (termasuk lokasi GPS). PNG transparan diberi latar putih.
func compress(data []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("foto tidak bisa dibaca, coba foto lain")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return nil, errors.New("resolusi foto terlalu besar")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("foto tidak bisa dibaca, coba foto lain")
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if longest := max(w, h); longest > maxDim {
		w, h = max(1, w*maxDim/longest), max(1, h*maxDim/longest)
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, xdraw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQ}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SavePhoto memvalidasi isi file sebenarnya, mengompresnya, lalu menyimpan. Mengembalikan URL publik.
func (s *Storage) SavePhoto(ctx context.Context, fh *multipart.FileHeader) (string, error) {
	if fh.Size > maxUpload {
		return "", errors.New("ukuran foto maksimal 10 MB")
	}
	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxUpload+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxUpload {
		return "", errors.New("ukuran foto maksimal 10 MB")
	}
	switch http.DetectContentType(data) {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return "", errors.New("format foto harus JPG, PNG, atau WebP")
	}

	small, err := compress(data)
	if err != nil {
		return "", err
	}
	name := uuid.NewString() + ".jpg"

	if s.remote() {
		return s.upload(ctx, name, "image/jpeg", small)
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(s.dir, name), small, 0o644); err != nil {
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
