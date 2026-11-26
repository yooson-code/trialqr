package models

import "time"

// Category adalah kelas anggota berdasarkan umur (batas inklusif).
type Category struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Min   int    `json:"min"`
	Max   int    `json:"max"`
}

// Urutan menaik menurut umur. Ubah batas di sini bila aturan kelas berubah;
// seluruh aplikasi (tampilan, filter, rekap) mengikuti daftar ini.
var Categories = []Category{
	{"anak", "Di bawah 11", 0, 10},
	{"pra-remaja", "Pra Remaja", 11, 15},
	{"remaja", "Remaja", 16, 18},
	{"mandiri", "Usia Mandiri", 19, 30},
	{"dewasa", "Dewasa", 31, 150},
}

const (
	CategoryUnknown = "belum"
	CategoryMandiri = "mandiri"

	MaritalSingle  = "belum"
	MaritalMarried = "menikah"
)

// MaritalLabel: keterangan status pernikahan (hanya ditampilkan untuk Usia Mandiri).
func MaritalLabel(status string) string {
	switch status {
	case MaritalSingle:
		return "Belum menikah"
	case MaritalMarried:
		return "Menikah"
	}
	return ""
}

// AgeOn menghitung umur penuh (tahun) pada tanggal `now`.
func AgeOn(birth, now time.Time) int {
	y := now.Year() - birth.Year()
	if now.Month() < birth.Month() || (now.Month() == birth.Month() && now.Day() < birth.Day()) {
		y--
	}
	return y
}

// Classify mengembalikan umur, label, dan key kategori.
func Classify(birth *time.Time, now time.Time) (age *int, label, key string) {
	if birth == nil {
		return nil, "Belum diisi", CategoryUnknown
	}
	a := AgeOn(*birth, now)
	for _, c := range Categories {
		if a >= c.Min && a <= c.Max {
			return &a, c.Label, c.Key
		}
	}
	return &a, "Belum diisi", CategoryUnknown
}

// Fill mengisi field turunan (Age, Category) dari BirthDate.
func (m *Member) Fill(now time.Time) {
	m.Age, m.Category, m.CategoryKey = Classify(m.BirthDate, now)
	m.MaritalLabel = ""
	if m.CategoryKey == CategoryMandiri && m.MaritalStatus != nil {
		m.MaritalLabel = MaritalLabel(*m.MaritalStatus)
	}
}
