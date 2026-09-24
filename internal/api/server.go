// Package api wires the HTTP routes for the JSON API and embedded web app.
package api

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/calibre"
	"github.com/zachcurry13/novelcheck/internal/config"
	"github.com/zachcurry13/novelcheck/internal/ollama"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/tunnel"
	"github.com/zachcurry13/novelcheck/internal/updates"
)

type Server struct {
	Cfg     config.Config
	Store   *store.Store
	Auth    *auth.Manager
	Worker  *analyzer.Worker
	Syncer  *calibre.Syncer
	Updates *updates.Checker
	Tunnel  *tunnel.Manager
	Pulls   ollama.Puller // Ollama model downloads
	Web     fs.FS         // embedded static assets
	logins  *loginLimiter
}

func (s *Server) Router() http.Handler {
	s.logins = newLoginLimiter()
	r := chi.NewRouter()
	r.Use(realIP(s.Cfg.TrustProxy))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(securityHeaders)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	r.Route("/api", func(r chi.Router) {
		r.Use(noStore, cors(s.Cfg.CORSOrigins), csrfGuard)
		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/logout", s.handleLogout)
		r.Get("/setup", s.handleSetupStatus)
		r.Post("/setup", s.handleSetup)

		r.Group(func(r chi.Router) {
			r.Use(s.Auth.RequireUser)
			r.Get("/me", s.handleMe)
			r.Put("/me/password", s.handleChangePassword)
			r.Put("/me/delivery", s.handleUpdateDelivery)
			r.Put("/me/guide-seen", s.handleGuideSeen)

			r.Get("/books", s.handleListBooks)
			r.Get("/books/{id}", s.handleGetBook)
			r.Get("/books/{id}/download", s.handleDownload)
			r.Get("/catalogs", s.handleListCatalogs)
			r.Get("/updates", s.handleUpdates)

			r.Get("/queue", s.handleListQueue)
			r.Post("/queue", s.handleEnqueue)
			r.Put("/queue/order", s.handleReorderQueue)
			r.Delete("/queue/{id}", s.handleDequeue)
			r.Post("/queue/{id}/start", s.handleStartReading)
			r.Post("/queue/{id}/finish", s.handleFinishReading)

			// Editors and admins: day-to-day management. Handlers further
			// restrict editors to kid accounts (see users_handlers.go).
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireManager)
				r.Post("/catalogs", s.handleCreateCatalog)
				r.Patch("/catalogs/{id}", s.handleRenameCatalog)
				r.Post("/import/drive", s.handleImportDrive)
				r.Post("/books/{id}/analyze", s.handleAnalyzeBook)
				r.Put("/books/{id}/verdict", s.handleSetVerdict)
				r.Put("/books/{id}/approval", s.handleSetApproval)

				r.Get("/admin/status", s.handleAdminStatus)
				r.Post("/admin/analyze-batch", s.handleAnalyzeBatch)
				r.Post("/admin/calibre-sync", s.handleCalibreSync)

				r.Get("/admin/users", s.handleListUsers)
				r.Post("/admin/users", s.handleCreateUser)
				r.Put("/admin/users/{id}", s.handleUpdateUser)
				r.Put("/admin/users/{id}/password", s.handleResetPassword)
				r.Delete("/admin/users/{id}", s.handleDeleteUser)
			})

			// Admins only: technical settings, secrets, destructive actions.
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAdmin)
				r.Delete("/catalogs/{id}", s.handleDeleteCatalog)
				r.Get("/admin/settings", s.handleGetSettings)
				r.Get("/admin/provider-guide", s.handleProviderGuide)
				r.Put("/admin/settings", s.handlePutSettings)
				r.Post("/admin/wipe-queue", s.handleWipeQueue)
				r.Get("/admin/calibre/browse", s.handleCalibreBrowse)
				r.Get("/admin/calibre/find", s.handleCalibreFind)
				r.Get("/admin/calibre/removal", s.handleCalibreRemoval)
				r.Put("/admin/calibre/library", s.handleSetCalibreLibrary)
				r.Post("/admin/smtp-test", s.handleSMTPTest)
				r.Get("/admin/backup", s.handleBackup)
				r.Get("/admin/tunnel", s.handleTunnelStatus)
				r.Put("/admin/tunnel", s.handleTunnelSave)
				r.Get("/admin/ollama/find", s.handleOllamaFind)
				r.Post("/admin/ollama/pull", s.handleOllamaPull)
				r.Get("/admin/ollama/pull", s.handleOllamaPullStatus)
				r.Post("/admin/ollama/use", s.handleOllamaUse)
			})
		})
	})

	r.NotFound(s.serveStatic)
	return r
}
