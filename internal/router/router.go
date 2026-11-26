package router

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"presensi-qr/internal/config"
	"presensi-qr/internal/handler"
	"presensi-qr/internal/middleware"
	"presensi-qr/internal/storage"
	"presensi-qr/internal/store"
	"presensi-qr/web"
)

func New(cfg config.Config, st *store.Store, files *storage.Storage) (*gin.Engine, error) {
	h, err := handler.New(st, cfg, files, middleware.NewLoginLimiter(8, 5*time.Minute))
	if err != nil {
		return nil, err
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	_ = r.SetTrustedProxies(nil) // pakai RemoteAddr
	if cfg.TrustedIPHeader != "" { // di belakang CDN/proxy (mis. Vercel): percaya header IP dari platform
		r.TrustedPlatform = cfg.TrustedIPHeader
	}
	r.MaxMultipartMemory = 4 << 20

	r.Use(func(c *gin.Context) { // header keamanan dasar
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "same-origin")
		c.Header("Permissions-Policy", "camera=(self)")
		c.Next()
	})

	static, _ := fs.Sub(web.FS, "static")
	r.StaticFS("/static", http.FS(static))
	r.Static("/uploads", files.LocalDir())

	r.GET("/login", h.LoginPage)
	r.POST("/login", h.Login)
	r.POST("/logout", h.Logout)

	// ---- area terproteksi ----
	app := r.Group("/", middleware.RequireAuth(st, cfg.JWTSecret, cfg.CookieSecure))
	{
		app.GET("/", h.Dashboard)
		app.GET("/scanner", h.ScannerPage)

		app.GET("/members", h.MembersPage)
		app.POST("/members", h.MemberCreate)
		app.GET("/members/cards", h.MemberCards)
		app.GET("/members/export", h.MembersExport)
		app.POST("/members/:id", h.MemberUpdate)
		app.POST("/members/:id/delete", h.MemberDelete)
		app.GET("/members/:id/qr.png", h.MemberQR)
		app.GET("/members/:id/card", h.MemberCard)

		app.GET("/schedules", h.SchedulesPage)
		app.POST("/schedules", h.ScheduleCreate)
		app.POST("/schedules/:id/toggle", h.ScheduleToggle)
		app.POST("/schedules/:id/delete", h.ScheduleDelete)

		app.GET("/master", h.MasterPage)
		app.POST("/master/villages", h.VillageCreate)
		app.POST("/master/villages/:id", h.VillageUpdate)
		app.POST("/master/villages/:id/delete", h.VillageDelete)
		app.POST("/master/groups", h.GroupCreate)
		app.POST("/master/groups/:id", h.GroupUpdate)
		app.POST("/master/groups/:id/delete", h.GroupDelete)

		app.GET("/reports", h.ReportsPage)
		app.GET("/reports/new", h.ReportNew)
		app.GET("/reports/compile", h.ReportsCompile)
		app.GET("/reports/export", h.ReportsExport)
		app.POST("/reports", h.ReportCreate)
		app.GET("/reports/:id", h.ReportView)
		app.GET("/reports/:id/edit", h.ReportEdit)
		app.POST("/reports/:id", h.ReportUpdate)
		app.POST("/reports/:id/submit", h.ReportSubmit)
		app.POST("/reports/:id/reopen", h.ReportReopen)
		app.POST("/reports/:id/delete", h.ReportDelete)

		app.GET("/attendance", h.AttendancePage)
		app.GET("/attendance/export", h.AttendanceExport)
		app.POST("/attendance/:id/delete", h.AttendanceDelete)

		// khusus admin
		adm := app.Group("/accounts", middleware.RequireAdmin())
		adm.GET("", h.AccountsPage)
		adm.POST("", h.AccountCreate)
		adm.POST("/:id/toggle", h.AccountToggle)
		adm.POST("/:id/password", h.AccountPassword)
		adm.POST("/:id/delete", h.AccountDelete)

		app.POST("/api/v1/scan", h.ScanAPI)
		app.GET("/api/v1/members/search", h.MemberSearchAPI)
	}
	return r, nil
}
