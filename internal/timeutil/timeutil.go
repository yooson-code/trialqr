// Package timeutil memusatkan seluruh penanganan zona waktu WIB (UTC+7).
package timeutil

import (
	"time"
	_ "time/tzdata" // tetap jalan di server/container tanpa database zona waktu
)

const DateLayout = "2006-01-02"

var WIB *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	WIB = loc
}

func Now() time.Time { return time.Now().In(WIB) }

// TodayStr mengembalikan tanggal hari ini menurut WIB, format YYYY-MM-DD.
func TodayStr() string { return Now().Format(DateLayout) }
