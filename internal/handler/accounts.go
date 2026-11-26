package handler

import (
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"presensi-qr/internal/auth"
	"presensi-qr/internal/store"
)

var usernameRe = regexp.MustCompile(`^[a-z0-9._-]{3,30}$`)

func (h *Handler) AccountsPage(c *gin.Context) {
	list, err := h.Store.ListAdmins(c.Request.Context())
	if err != nil {
		serverErr(c, "/", err)
		return
	}
	h.render(c, "accounts.html", gin.H{"Title": "Akun pengurus", "Active": "accounts", "Admins": list})
}

func (h *Handler) AccountCreate(c *gin.Context) {
	username := strings.ToLower(strings.TrimSpace(c.PostForm("username")))
	name := clean(c.PostForm("full_name"))
	pass := c.PostForm("password")
	role := c.PostForm("role")

	switch {
	case name == "":
		fail(c, "/accounts", "Nama lengkap wajib diisi.")
		return
	case !usernameRe.MatchString(username):
		fail(c, "/accounts", "Username 3-30 karakter: huruf kecil, angka, titik, strip, atau garis bawah.")
		return
	case len(pass) < 8:
		fail(c, "/accounts", "Password minimal 8 karakter.")
		return
	case role != "admin" && role != "pengurus":
		role = "pengurus"
	}
	hash, err := auth.HashPassword(pass)
	if err != nil {
		serverErr(c, "/accounts", err)
		return
	}
	if err := h.Store.CreateAdmin(c.Request.Context(), username, name, hash, role); err != nil {
		if store.IsUnique(err) {
			fail(c, "/accounts", "Username \""+username+"\" sudah dipakai.")
			return
		}
		serverErr(c, "/accounts", err)
		return
	}
	success(c, "/accounts", "Akun "+name+" dibuat.")
}

func (h *Handler) AccountToggle(c *gin.Context) {
	id := idParam(c)
	if id == userOf(c).ID {
		fail(c, "/accounts", "Anda tidak bisa menonaktifkan akun sendiri.")
		return
	}
	if err := h.Store.SetAdminActive(c.Request.Context(), id, c.PostForm("active") == "1"); err != nil {
		serverErr(c, "/accounts", err)
		return
	}
	success(c, "/accounts", "Status akun diperbarui.")
}

func (h *Handler) AccountPassword(c *gin.Context) {
	pass := c.PostForm("password")
	if len(pass) < 8 {
		fail(c, "/accounts", "Password baru minimal 8 karakter.")
		return
	}
	hash, err := auth.HashPassword(pass)
	if err != nil {
		serverErr(c, "/accounts", err)
		return
	}
	if err := h.Store.SetAdminPassword(c.Request.Context(), idParam(c), hash); err != nil {
		serverErr(c, "/accounts", err)
		return
	}
	success(c, "/accounts", "Password berhasil diganti.")
}

func (h *Handler) AccountDelete(c *gin.Context) {
	id := idParam(c)
	if id == userOf(c).ID {
		fail(c, "/accounts", "Anda tidak bisa menghapus akun sendiri.")
		return
	}
	if err := h.Store.DeleteAdmin(c.Request.Context(), id); err != nil {
		serverErr(c, "/accounts", err)
		return
	}
	success(c, "/accounts", "Akun dihapus.")
}
