package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"presensi-qr/internal/auth"
	"presensi-qr/internal/middleware"
)

func (h *Handler) LoginPage(c *gin.Context) {
	h.render(c, "login.html", gin.H{"Title": "Masuk"})
}

func (h *Handler) Login(c *gin.Context) {
	ip := c.ClientIP()
	if !h.Limiter.Allow(ip) {
		fail(c, "/login", "Terlalu banyak percobaan. Tunggu beberapa menit lalu coba lagi.")
		return
	}
	username := strings.ToLower(strings.TrimSpace(c.PostForm("username")))
	password := c.PostForm("password")

	u, err := h.Store.AdminByUsername(c.Request.Context(), username)
	if err != nil || !u.IsActive || !auth.CheckPassword(u.PasswordHash, password) {
		fail(c, "/login", "Username atau password salah.")
		return
	}
	tok, err := auth.Sign(h.Cfg.JWTSecret, u.ID)
	if err != nil {
		serverErr(c, "/login", err)
		return
	}
	h.Limiter.Reset(ip)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.CookieName, tok, int(auth.SessionTTL.Seconds()), "/", "", h.Cfg.CookieSecure, true)
	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handler) Logout(c *gin.Context) {
	middleware.ClearSession(c, h.Cfg.CookieSecure)
	c.Redirect(http.StatusSeeOther, "/login")
}
