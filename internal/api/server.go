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
	"github.com/zachcurry13/novelcheck/internal/push"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/sysinfo"
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
	SysInfo *sysinfo.Sampler
	Push    *push.Service // phone notifications; nil turns them off
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
			r.Post("/books/{id}/delete-request", s.handleRequestDelete)
			r.Delete("/books/{id}/delete-request", s.handleCancelDelete)
			r.Get("/catalogs", s.handleListCatalogs)
			r.Get("/age-groups", s.handleAgeGroups)
			r.Get("/updates", s.handleUpdates)
			r.Get("/flags", s.handleListFlags)
			r.Get("/push", s.handlePushStatus)
			r.Post("/push/subscribe", s.handlePushSubscribe)
			r.Post("/push/unsubscribe", s.handlePushUnsubscribe)
			r.Post("/push/test", s.handlePushTest)

			r.Group(func(r chi.Router) {
				r.Use(s.requireModule(store.KeyModuleQueue, "The reading queue"))
				r.Get("/queue", s.handleListQueue)
				r.Post("/queue", s.handleEnqueue)
				r.Put("/queue/order", s.handleReorderQueue)
				r.Delete("/queue/{id}", s.handleDequeue)
				r.Post("/queue/{id}/start", s.handleStartReading)
				r.Post("/queue/{id}/finish", s.handleFinishReading)
			})

			// Editors and admins: day-to-day management. Handlers further
			// restrict editors to kid accounts (see users_handlers.go).
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireManager)
				r.Post("/check", s.handleCheck)
				r.Post("/catalogs", s.handleCreateCatalog)
				r.Patch("/catalogs/{id}", s.handleRenameCatalog)
				r.With(s.requireModule(store.KeyModuleImport, "Importing books")).Post("/import/drive", s.handleImportDrive)
				r.Post("/books/{id}/analyze", s.handleAnalyzeBook)
				r.Put("/books/{id}/verdict", s.handleSetVerdict)
				r.Put("/books/{id}/approval", s.handleSetApproval)
				r.Put("/books/{id}/age", s.handleSetAge)
				r.Post("/books/{id}/notes", s.handleAddNote)
				r.Put("/notes/{id}", s.handleUpdateNote)
				r.Delete("/notes/{id}", s.handleDeleteNote)

				r.Get("/admin/status", s.handleAdminStatus)
				r.Get("/admin/calibre/duplicates", s.handleDuplicates)
				r.Post("/admin/rerate", s.handleRerate)
				r.Get("/notifications", s.handleNotifications)
				r.Post("/notifications/read", s.handleNotificationsRead)
				r.Delete("/notifications", s.handleNotificationsClear)
				r.Post("/admin/analyze-batch", s.handleAnalyzeBatch)
				r.Get("/admin/errors", s.handleErrors)
				r.Post("/admin/retry-errors", s.handleRetryErrors)
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
				r.Post("/admin/flags", s.handleAddFlag)
				r.Put("/admin/flags/{id}", s.handleUpdateFlag)
				r.Delete("/admin/flags/{id}", s.handleDeleteFlag)
				r.Get("/admin/settings", s.handleGetSettings)
				r.Get("/admin/provider-guide", s.handleProviderGuide)
				r.Put("/admin/settings", s.handlePutSettings)
				r.Post("/admin/wipe-queue", s.handleWipeQueue)
				r.Get("/admin/calibre/browse", s.handleCalibreBrowse)
				r.Get("/admin/calibre/find", s.handleCalibreFind)
				r.Get("/admin/calibre/removal", s.handleCalibreRemoval)
				r.Post("/admin/calibre/remove", s.handleCalibreRemove)
				r.Post("/admin/calibre/duplicates/remove", s.handleRemoveDuplicates)
				r.Get("/admin/delete-requests", s.handleDeleteRequests)
				r.Post("/admin/delete-requests/decide", s.handleDecideDeletes)
				r.Get("/admin/calibre/server", s.handleCalibreServerGet)
				r.Put("/admin/calibre/server", s.handleCalibreServerSave)
				r.Put("/admin/calibre/library", s.handleSetCalibreLibrary)
				r.Post("/admin/smtp-test", s.handleSMTPTest)
				r.Get("/admin/backup", s.handleBackup)
				r.Get("/admin/tunnel", s.handleTunnelStatus)
				r.Get("/admin/system", s.handleSystem)
				r.Post("/admin/health", s.handleHealthChecks)
				r.Get("/admin/diagnostics", s.handleDiagnostics)
				r.Post("/admin/diagnose", s.handleDiagnose)
				r.Put("/admin/tunnel", s.handleTunnelSave)
				r.Get("/admin/ollama/find", s.handleOllamaFind)
				r.Post("/admin/ollama/pull", s.handleOllamaPull)
				r.Get("/admin/ollama/pull", s.handleOllamaPullStatus)
				r.Post("/admin/ollama/use", s.handleOllamaUse)
				r.Get("/admin/ollama/gpu", s.handleOllamaGPU)
			})
		})
	})

	r.NotFound(s.serveStatic)
	return r
}
