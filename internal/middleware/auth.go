package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"presensi-qr/internal/auth"
	"presensi-qr/internal/models"
	"presensi-qr/internal/store"
)

const (
	CookieName = "session"
	userKey    = "user"
)

// CurrentUser mengambil akun yang sedang login (hanya valid setelah RequireAuth).
func CurrentUser(c *gin.Context) *models.Admin {
	if v, ok := c.Get(userKey); ok {
		if u, ok := v.(*models.Admin); ok {
			return u
		}
	}
	return nil
}

func ClearSession(c *gin.Context, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieName, "", -1, "/", "", secure, true)
}

// RequireAuth memeriksa cookie sesi dan memastikan akun masih aktif di database.
func RequireAuth(st *store.Store, secret string, secure bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		deny := func() {
			ClearSession(c, secure)
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"status": "error", "message": "Sesi berakhir, silakan login ulang",
				})
				return
			}
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
		}
		tok, err := c.Cookie(CookieName)
		if err != nil {
			deny()
			return
		}
		id, err := auth.Parse(secret, tok)
		if err != nil {
			deny()
			return
		}
		u, err := st.AdminByID(c.Request.Context(), id)
		if err != nil || !u.IsActive {
			deny()
			return
		}
		c.Set(userKey, u)
		c.Next()
	}
}

// RequireAdmin: hanya role "admin" (bukan pengurus biasa).
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if u := CurrentUser(c); u == nil || !u.IsAdmin() {
			c.String(http.StatusForbidden, "Halaman ini khusus admin")
			c.Abort()
			return
		}
		c.Next()
	}
}
