// Package handler adalah entry point serverless function untuk Vercel.
// Semua request diarahkan ke sini oleh vercel.json, lalu diteruskan ke router Gin.
package handler

import (
	"context"
	"log"
	"net/http"
	"sync"

	"presensi-qr/internal/app"
)

var (
	mu     sync.Mutex
	engine http.Handler
)

// ready merakit aplikasi sekali per instance (dipakai ulang antar request).
// Bila gagal (mis. env belum diisi), akan dicoba lagi pada request berikutnya.
func ready() (http.Handler, error) {
	mu.Lock()
	defer mu.Unlock()
	if engine != nil {
		return engine, nil
	}
	e, _, _, err := app.New(context.Background())
	if err != nil {
		return nil, err
	}
	engine = e
	return engine, nil
}

// Handler adalah fungsi yang dipanggil Vercel.
func Handler(w http.ResponseWriter, r *http.Request) {
	h, err := ready()
	if err != nil {
		log.Printf("init gagal: %v", err)
		http.Error(w, "Layanan belum siap. Periksa Environment Variables dan log di dashboard Vercel.", http.StatusInternalServerError)
		return
	}
	h.ServeHTTP(w, r)
}
