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
	"log"
	"mime/multipart"
	"net/url"
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

// baseURL mengambil hanya skema + host dari SUPABASE_URL. Path tambahan yang sering terselip
// (mis. "/rest/v1", "/storage/v1") membuat permintaan nyasar ke API database (galat PGRST125).
func baseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimRight(raw, "/")
	}
	return u.Scheme + "://" + u.Host
}

func New(c config.Config) *Storage {
	return &Storage{
		url:    baseURL(c.SupabaseURL),
		key:    strings.TrimSpace(c.SupabaseKey),
		bucket: strings.Trim(strings.TrimSpace(c.SupabaseBucket), "/ "),
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

func (s *Storage) put(ctx context.Context, name, ct string, data []byte) (int, string, error) {
	endpoint := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.url, url.PathEscape(s.bucket), name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("apikey", s.key)
	req.Header.Set("Content-Type", ct)
	return s.do(req)
}

func (s *Storage) do(req *http.Request) (int, string, error) {
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("tidak bisa menghubungi Supabase (%s): %w", s.url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
	return resp.StatusCode, string(body), nil
}

// createBucket membuat bucket publik bila belum ada (butuh service_role key).
func (s *Storage) createBucket(ctx context.Context) error {
	payload := fmt.Sprintf(`{"id":%q,"name":%q,"public":true}`, s.bucket, s.bucket)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url+"/storage/v1/bucket", strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("apikey", s.key)
	req.Header.Set("Content-Type", "application/json")
	status, body, err := s.do(req)
	if err != nil {
		return err
	}
	if status >= 300 && status != http.StatusConflict {
		return fmt.Errorf("bucket %q belum ada dan gagal dibuat otomatis (%d): %s", s.bucket, status, body)
	}
	return nil
}

func explain(status int, body string) error {
	lower := strings.ToLower(body)
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return errors.New("kunci Supabase ditolak. Pakai kunci \"service_role\" (bukan \"anon\") untuk SUPABASE_SERVICE_KEY")
	case strings.Contains(lower, "pgrst125") || strings.Contains(lower, "invalid path"):
		return errors.New("SUPABASE_URL tidak benar. Isi hanya alamat proyek, contoh https://abcdxyz.supabase.co")
	default:
		return fmt.Errorf("upload ke Supabase gagal (%d): %s", status, body)
	}
}

func (s *Storage) upload(ctx context.Context, name, ct string, data []byte) (string, error) {
	status, body, err := s.put(ctx, name, ct, data)
	if err != nil {
		return "", err
	}
	if (status == http.StatusNotFound || status == http.StatusBadRequest) && strings.Contains(strings.ToLower(body), "bucket not found") {
		if err := s.createBucket(ctx); err != nil {
			return "", err
		}
		status, body, err = s.put(ctx, name, ct, data)
		if err != nil {
			return "", err
		}
	}
	if status >= 300 {
		log.Printf("storage: %d %s (url=%s bucket=%s)", status, body, s.url, s.bucket)
		return "", explain(status, body)
	}
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.url, url.PathEscape(s.bucket), name), nil
}
