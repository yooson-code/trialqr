# Hadir: Presensi QR Muda-Mudi

Aplikasi web presensi pengajian berbasis QR Code. Backend Go (Gin), database Supabase PostgreSQL,
frontend server-rendered (`html/template`) + Tailwind CSS + Alpine.js, scanner kamera dengan `html5-qrcode`.

## Menjalankan

1. **Database.** Buka Supabase > SQL Editor, jalankan berurutan `migrations/001_init.sql`, lalu `002_profil_rfid.sql`, lalu `003_cakupan_jadwal.sql`, lalu `004_laporan_musyawarah.sql`.
   (Sudah pernah menjalankan 001? Cukup jalankan 002; isinya aman diulang.)
2. **Konfigurasi.** `cp .env.example .env`, lalu isi `DATABASE_URL`, `JWT_SECRET`, `ADMIN_USERNAME`, `ADMIN_PASSWORD`.
   `DATABASE_URL` ada di Supabase > Project Settings > Database > Connection string (URI).
3. **Dependensi dan jalankan** (perlu Go 1.22+ dan internet sekali untuk mengunduh modul):
   ```bash
   go mod tidy
   go run ./cmd/server
   ```
4. Buka http://localhost:8080, masuk dengan admin dari `.env`. Akun admin pertama dibuat otomatis saat tabel `admins` masih kosong.

Alur pemakaian: **Desa & kelompok** > **Anggota** (QR otomatis dibuat) > **Jadwal pengajian** > **Scanner**.

> Kamera browser hanya jalan di `https://` atau `localhost`. Untuk dipakai di HP saat pengajian,
> deploy di belakang HTTPS (Fly.io, Railway, VPS + Caddy, dsb.) dan set `COOKIE_SECURE=true`.

## Umur, kategori, dan data orang tua

Form anggota meminta **tanggal lahir** (bukan umur), jadi umur dan kategori selalu mutakhir tanpa perlu diedit tiap tahun.

| Umur       | Kategori                                          |
|------------|---------------------------------------------------|
| < 11       | Di bawah 11                                       |
| 11 - 15    | Pra Remaja                                        |
| 16 - 18    | Remaja                                            |
| 19 - 30    | Usia Mandiri (+ status: belum menikah / menikah)  |
| 31 ke atas | Dewasa                                            |

Status pernikahan hanya muncul di form dan ditampilkan untuk anggota Usia Mandiri. Bila anggota berpindah ke kategori
Dewasa, kolom status tidak lagi dipakai (dan dikosongkan saat data anggota disimpan ulang). Daftar anggota bisa difilter
berdasarkan status pernikahan.

Batas ini ada di satu tempat: `internal/models/category.go`. Kategori muncul di daftar anggota, kartu, hasil scan,
rekap, dan dashboard, serta bisa dipakai sebagai filter. Nama dan nomor HP orang tua/wali tampil di daftar anggota
(dengan tautan WhatsApp) dan di tab "Per anggota" pada rekap, supaya mudah menghubungi orang tua anggota yang jarang hadir.

## Cakupan pengajian

Setiap jadwal punya cakupan:

| Cakupan  | Siapa yang boleh absen                 |
|----------|-----------------------------------------|
| Daerah   | semua anggota                           |
| Desa     | hanya anggota dari desa yang dipilih    |
| Kelompok | hanya anggota dari kelompok yang dipilih |

Di scanner, tiap jadwal menampilkan cakupannya, dan anggota di luar cakupan ditolak dengan pesan
"Pengajian ini khusus ..." (kode 403, `reason: "scope"`). Pada rekap "Per anggota", bila filter pengajian dipilih,
hanya anggota dalam cakupan yang dihitung, sehingga anggota desa lain tidak tampak "tidak hadir".
Jadwal lama otomatis menjadi cakupan Daerah.

## Laporan musyawarah bulanan

Menu **Laporan musyawarah** untuk pengurus mengisi laporan sebelum musyawarah (satu laporan per pengurus per bulan):

1. **Identitas:** periode, bidang/jabatan, dan cakupan (daerah / desa / kelompok).
2. **Ringkasan kinerja:** ringkasan umum dan daftar kegiatan, masing-masing dengan status, capaian, kendala, dan rencana tindak lanjut.
3. **Rekap kehadiran:** dihitung otomatis dari data absensi pada periode dan cakupan laporan (total hadir, anggota yang pernah hadir,
   per kategori, per pengajian, dan jumlah anggota yang belum pernah hadir). Pengurus bisa menambah catatan pelengkap.
4. **Catatan untuk musyawarah:** poin evaluasi, kebutuhan anggaran/logistik, dan topik pembahasan.

Laporan disimpan sebagai **draft** (hanya terlihat penulis dan admin) sampai dikirim. Laporan terkirim bisa dibaca semua pengurus.
Penulis dan admin dapat mengubah, menarik kembali menjadi draft, atau menghapus laporan. Dashboard mengingatkan pengurus yang belum mengisi laporan bulan berjalan.

Untuk forum musyawarah, **Ringkasan musyawarah** (tombol di daftar laporan) menyatukan seluruh laporan terkirim pada satu bulan:
rekap kehadiran seluruh daerah, daftar usulan topik, kebutuhan anggaran, serta kendala beserta tindak lanjutnya, lalu laporan tiap pengurus.
Halaman ini siap dicetak atau disimpan sebagai PDF lewat dialog cetak browser, dan datanya bisa diekspor ke Excel (dua sheet: Laporan dan Kegiatan).

## Aksesibilitas

- Pengaturan **Ukuran teks** (normal / besar / sangat besar) di menu samping, tersimpan di perangkat.
- Area sentuh minimal 44 px dan font input 16 px di ponsel, kontras teks sekunder dinaikkan, fokus keyboard terlihat jelas,
  ada tautan "Lewati ke konten", dan animasi dimatikan bila pengguna memilih *reduced motion*.

## Presensi RFID

1. Pakai reader RFID/NFC **USB** yang berperilaku seperti keyboard (HID): umumnya hampir semua reader murah. Colokkan ke laptop yang membuka halaman Scanner.
2. Daftarkan kartu: **Anggota > Ubah**, klik kolom *Kartu RFID*, tempelkan kartu. Nomornya terisi sendiri.
3. Buka **Scanner**, pilih tab **Kartu RFID**, lalu tempelkan kartu. Umpan balik suara dan warnanya sama dengan scan QR.

Catatan: daftarkan dan scan kartu memakai reader yang sama, karena sebagian reader mengeluarkan nomor dalam format
desimal dan sebagian heksadesimal. Nomor dinormalkan otomatis (huruf besar, tanpa spasi/titik dua).

## Fitur tambahan

- **Absen manual** di halaman Scanner (cari nama, tekan *Hadirkan*) untuk anggota yang lupa kartu. Tercatat dengan metode "Manual".
- **Batalkan kehadiran** di rekap harian bila salah scan.
- Kolom **Metode** (QR / RFID / Manual) dan **Kategori** di rekap, CSV, dan Excel.
- **Ekspor daftar anggota** ke CSV mengikuti filter yang sedang aktif.
- Statistik **kehadiran per kategori** di dashboard.

## Deploy ke Vercel

Vercel menjalankan Go sebagai serverless function (`api/index.go`), bukan server yang menyala terus.

1. Push proyek ke GitHub (jalankan `go mod tidy` dulu dan **commit `go.sum`**).
2. Di vercel.com: **Add New > Project**, pilih repo. Framework preset: *Other*. Biarkan build/output kosong.
3. **Settings > Environment Variables**, isi:
   - `DATABASE_URL`: pakai **Transaction pooler** Supabase (port `6543`), bukan koneksi langsung.
   - `JWT_SECRET`, `ADMIN_USERNAME`, `ADMIN_PASSWORD`, `ADMIN_NAME`
   - `COOKIE_SECURE=true`, `DB_MAX_CONNS=3`, `TRUSTED_IP_HEADER=X-Real-Ip`
   - `SUPABASE_URL`, `SUPABASE_SERVICE_KEY`, `SUPABASE_BUCKET=member-photos` (wajib untuk foto)
4. **Deploy**. Buka URL `*.vercel.app` dan login dengan admin dari env.

Catatan: disk Vercel tidak permanen, jadi foto harus lewat Supabase Storage. Pembatas percobaan login berjalan per instance
(bukan global). `vercel.json` memakai region Singapura (`sin1`) agar dekat dengan Supabase dan pengguna di Indonesia.
Alternatif untuk server yang menyala terus (tanpa cold start): Fly.io, Railway, atau VPS dengan `Dockerfile` yang tersedia.

## Deploy ke Render

Render menjalankan aplikasi sebagai server biasa (memakai `Dockerfile`), tanpa cold start pada plan berbayar.

1. Jalankan `go mod tidy`, lalu push proyek ke GitHub (**commit `go.sum`**, karena Dockerfile membutuhkannya).
2. Di dashboard.render.com pilih **New > Blueprint**, hubungkan repo. Render membaca `render.yaml`.
   (Cara manual: **New > Web Service**, Language: *Docker*, Region: *Singapore*, Health Check Path: `/login`.)
3. Isi environment variable yang diminta:
   - `DATABASE_URL`: **Session pooler** Supabase (port `5432`). Hindari koneksi langsung karena memakai IPv6.
   - `ADMIN_PASSWORD` (min. 8 karakter), `SUPABASE_URL`, `SUPABASE_SERVICE_KEY`.
   - `JWT_SECRET` dibuat otomatis, `COOKIE_SECURE` sudah `true`.
4. **Deploy**. Buka URL `*.onrender.com`, login dengan admin dari env. Migrasi 001 sampai 004 harus sudah dijalankan di Supabase.

Catatan:
- **Plan Free tidur setelah 15 menit tanpa trafik** dan butuh sekitar satu menit untuk bangun. Itu mengganggu saat pengajian, jadi pakai **Starter** untuk pemakaian sungguhan.
- Disk Render tidak permanen (kecuali ditambah Persistent Disk), jadi isi `SUPABASE_URL` dan `SUPABASE_SERVICE_KEY` agar foto tersimpan di Supabase Storage.
- Kamera scanner butuh HTTPS, dan Render menyediakannya otomatis.
- Pembatas login membaca IP dari `RemoteAddr`. Bila pengguna cepat saling terkunci karena IP proxy yang sama, set `TRUSTED_IP_HEADER` ke header IP klien dari Render (mis. `CF-Connecting-IP`; cek di log request).

## Struktur proyek

```
cmd/server/main.go          entry point, seed admin, graceful shutdown
internal/
  config/                   baca env
  models/                   struct domain
  store/                    semua query SQL (pgx), satu file per domain
  service/scan.go           logika bisnis scan: validasi token + anti double-scan
  handler/                  HTTP handler + helper template (Gin)
  middleware/               auth JWT-cookie, role admin, rate limit login
  auth/                     JWT + bcrypt
  storage/                  upload foto (Supabase Storage / lokal)
  timeutil/                 zona waktu WIB
  router/                   pemetaan route
web/
  templates/                halaman HTML (di-embed ke binary)
  static/
migrations/001_init.sql     skema Supabase
```

## API scan

`POST /api/v1/scan` (butuh sesi login). Isi salah satu dari tiga identitas:

```json
{"token": "<uuid dari QR>",  "schedule_id": 1}
{"rfid_uid": "04A1B2C3",     "schedule_id": 1}
{"member_id": "<uuid>",      "schedule_id": 1}
```

`GET /api/v1/members/search?q=nama` mengembalikan anggota aktif untuk absen manual.

| HTTP | `status`    | Arti                                           |
|------|-------------|------------------------------------------------|
| 201  | `success`   | Kehadiran tercatat                             |
| 409  | `duplicate` | Sudah absen pada tanggal (WIB) yang sama       |
| 404  | `invalid`   | Kode tidak valid / QR atau kartu belum terdaftar |
| 403  | `invalid`   | Anggota nonaktif                               |
| 400  | `error`     | Payload atau jadwal tidak valid                |

## Catatan desain

- **Double-scan** dicegah di level database (`UNIQUE (member_id, scan_date)` + `ON CONFLICT DO NOTHING`),
  jadi aman walau dua perangkat memindai bersamaan. Sesuai spesifikasi, batasnya satu kali per anggota per hari,
  apa pun jadwalnya.
- **Zona waktu:** `scanned_at` bertipe `timestamptz`; `scan_date` dihitung di Go memakai `Asia/Jakarta`,
  jadi absen jam 00:30 WIB tidak jatuh ke tanggal kemarin (UTC).
- **QR Code** dibuat on-the-fly dari `qr_code_token` (UUID), tidak disimpan sebagai file. Karena deterministik,
  mencetak ulang selalu menghasilkan QR yang sama.
- **Foto** diunggah ke bucket Supabase `member-photos` bila `SUPABASE_URL` dan `SUPABASE_SERVICE_KEY` diisi,
  jika tidak disimpan di folder `uploads/`.
- RLS diaktifkan pada semua tabel tanpa policy, sehingga data tidak bisa dibaca lewat anon key Supabase.
  Aplikasi Go terhubung langsung sebagai role `postgres`.
- Tailwind, Alpine, dan html5-qrcode dimuat dari CDN. Untuk produksi penuh, bisa diganti build lokal.
