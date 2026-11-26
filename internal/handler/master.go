package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"presensi-qr/internal/store"
)

func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

func (h *Handler) MasterPage(c *gin.Context) {
	views, err := h.Store.VillageViews(c.Request.Context())
	if err != nil {
		serverErr(c, "/", err)
		return
	}
	h.render(c, "master.html", gin.H{"Title": "Desa & kelompok", "Active": "master", "Views": views})
}

func (h *Handler) saveErr(c *gin.Context, err error, what string) {
	switch {
	case store.IsUnique(err):
		fail(c, "/master", what+" dengan nama tersebut sudah ada.")
	default:
		serverErr(c, "/master", err)
	}
}

func (h *Handler) VillageCreate(c *gin.Context) {
	name := clean(c.PostForm("name"))
	if name == "" {
		fail(c, "/master", "Nama desa wajib diisi.")
		return
	}
	if err := h.Store.CreateVillage(c.Request.Context(), name); err != nil {
		h.saveErr(c, err, "Desa")
		return
	}
	success(c, "/master", "Desa "+name+" ditambahkan.")
}

func (h *Handler) VillageUpdate(c *gin.Context) {
	name := clean(c.PostForm("name"))
	if name == "" {
		fail(c, "/master", "Nama desa wajib diisi.")
		return
	}
	if err := h.Store.UpdateVillage(c.Request.Context(), idParam(c), name); err != nil {
		h.saveErr(c, err, "Desa")
		return
	}
	success(c, "/master", "Nama desa diperbarui.")
}

func (h *Handler) VillageDelete(c *gin.Context) {
	if err := h.Store.DeleteVillage(c.Request.Context(), idParam(c)); err != nil {
		if store.IsInUse(err) {
			fail(c, "/master", "Desa masih dipakai (punya kelompok, jadwal, atau laporan). Hapus atau pindahkan dulu.")
			return
		}
		serverErr(c, "/master", err)
		return
	}
	success(c, "/master", "Desa dihapus.")
}

func (h *Handler) GroupCreate(c *gin.Context) {
	name := clean(c.PostForm("name"))
	vid := atoi64(c.PostForm("village_id"))
	if name == "" || vid <= 0 {
		fail(c, "/master", "Nama kelompok wajib diisi.")
		return
	}
	if err := h.Store.CreateGroup(c.Request.Context(), vid, name); err != nil {
		h.saveErr(c, err, "Kelompok")
		return
	}
	success(c, "/master", "Kelompok "+name+" ditambahkan.")
}

func (h *Handler) GroupUpdate(c *gin.Context) {
	name := clean(c.PostForm("name"))
	if name == "" {
		fail(c, "/master", "Nama kelompok wajib diisi.")
		return
	}
	if err := h.Store.UpdateGroup(c.Request.Context(), idParam(c), name); err != nil {
		h.saveErr(c, err, "Kelompok")
		return
	}
	success(c, "/master", "Nama kelompok diperbarui.")
}

func (h *Handler) GroupDelete(c *gin.Context) {
	if err := h.Store.DeleteGroup(c.Request.Context(), idParam(c)); err != nil {
		if store.IsInUse(err) {
			fail(c, "/master", "Kelompok masih dipakai (punya anggota, jadwal, atau laporan). Pindahkan atau hapus dulu.")
			return
		}
		serverErr(c, "/master", err)
		return
	}
	success(c, "/master", "Kelompok dihapus.")
}
