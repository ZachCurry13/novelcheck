package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// adminRoutes are the admins' routes, each part behind the area the main
// admin gives (internal/store/areas.go). The settings are split per key
// (settingArea), and the main admin's own tools check Owner themselves.
func (s *Server) adminRoutes(r chi.Router) {
	r.Use(auth.RequireAdmin)

	r.Get("/admin/settings", s.handleGetSettings) // filtered to the admin's areas
	r.Put("/admin/settings", s.handlePutSettings) // refused outside them
	r.Put("/admin/users/{id}/access", s.handleUserAccess)
	r.Post("/admin/users/{id}/owner", s.handleMakeOwner)

	r.Group(func(r chi.Router) { // 🤖 AI settings
		r.Use(area(store.AreaAI))
		r.Get("/admin/provider-guide", s.handleProviderGuide)
		r.Get("/admin/aitools", s.handleAIToolsStatus)
		r.Post("/admin/aitools/check-updates", s.handleCheckModelUpdates)
		r.Post("/admin/aitools/updated", s.handleModelUpdated)
		r.Post("/admin/aitools/bench", s.handleStartBench)
		r.Post("/admin/flags", s.handleAddFlag)
		r.Put("/admin/flags/{id}", s.handleUpdateFlag)
		r.Delete("/admin/flags/{id}", s.handleDeleteFlag)
		r.Post("/admin/wipe-queue", s.handleWipeQueue)
		r.Get("/admin/genres", s.handleGenreStatus)
		r.Post("/admin/genres/fill", s.handleGenreFill)
		r.Post("/admin/genres/stop", s.handleGenreStop)
		r.Get("/admin/ollama/find", s.handleOllamaFind)
		r.Post("/admin/ollama/pull", s.handleOllamaPull)
		r.Get("/admin/ollama/pull", s.handleOllamaPullStatus)
		r.Post("/admin/ollama/use", s.handleOllamaUse)
		r.Get("/admin/ollama/gpu", s.handleOllamaGPU)
		r.Get("/admin/ollama/models", s.handleOllamaModels)
		r.Post("/admin/ollama/delete", s.handleOllamaDelete)
	})

	r.Group(func(r chi.Router) { // 🧬 Deep Scans
		r.Use(area(store.AreaDeep))
		r.Get("/admin/deep-scans", s.handleDeepScans)
		r.Get("/admin/deep-scans/next", s.handleDeepScanNext)
		r.Post("/admin/deep-scans/next", s.handleDeepScanNext)
		r.Post("/admin/deep-scans/keep-all", s.handleKeepAllDeepScans)
		r.Post("/admin/deep-scans/accept-all", s.handleAcceptAllDeepScans)
		r.Put("/admin/deep-scans/order", s.handleOrderDeepScans)
		r.Post("/admin/deep-scans/{id}/{action}", s.handleDecideDeepScan)
	})

	r.Group(func(r chi.Router) { // 📚 Calibre & cleanup
		r.Use(area(store.AreaCalibre))
		r.Get("/admin/calibre/browse", s.handleCalibreBrowse)
		r.Get("/admin/calibre/find", s.handleCalibreFind)
		r.Get("/admin/calibre/removal", s.handleCalibreRemoval)
		r.Post("/admin/calibre/remove", s.handleCalibreRemove)
		r.Post("/admin/calibre/duplicates/remove", s.handleRemoveDuplicates)
		r.Get("/admin/calibre/server", s.handleCalibreServerGet)
		r.Put("/admin/calibre/server", s.handleCalibreServerSave)
		r.Put("/admin/calibre/library", s.handleSetCalibreLibrary)
		r.Get("/admin/formats", s.handleFormats)
		r.Post("/admin/formats/remove", s.handleRemoveFormats)
		r.Post("/admin/formats/undo", s.handleUndoFormats)
		r.Post("/admin/formats/convert", s.handleConvertFormats)
		r.Get("/admin/formats/job", s.handleFormatJob)
		r.Post("/admin/formats/job/stop", s.handleStopFormatJob)
		r.Get("/admin/title-fixes", s.handleTitleFixes)
		r.Post("/admin/title-fixes", s.handleFixTitles)
		r.Post("/books/{id}/calibre-title", s.handleSaveCalibreTitle)
		r.Get("/admin/cover-reports", s.handleCoverReports)
		r.Post("/admin/cover-reports/{id}/{action}", s.handleCloseCoverReport)
		r.Get("/admin/delete-requests", s.handleDeleteRequests)
		r.Post("/admin/delete-requests/decide", s.handleDecideDeletes)
	})

	r.Group(func(r chi.Router) { // ✉️ Email & Discover
		r.Use(area(store.AreaServices))
		r.Post("/admin/smtp-test", s.handleSMTPTest)
		r.Get("/admin/discover", s.handleDiscoverStatus)
		r.Post("/admin/discover/refresh", s.handleDiscoverRefresh)
		r.Post("/admin/discover/test", s.handleDiscoverTest)
	})

	r.Group(func(r chi.Router) { // 🖥️ System
		r.Use(area(store.AreaSystem))
		r.Post("/admin/safe-mode/leave", s.handleLeaveSafeMode)
		r.Get("/admin/problems", s.handleProblemReports)
		r.Post("/admin/problems/{id}/done", s.handleCloseProblemReport)
		r.Get("/admin/backup", s.handleBackup)
		r.Get("/admin/tunnel", s.handleTunnelStatus)
		r.Put("/admin/tunnel", s.handleTunnelSave)
		r.Get("/admin/system", s.handleSystem)
		r.Post("/admin/health", s.handleHealthChecks)
		r.Get("/admin/diagnostics", s.handleDiagnostics)
		r.Post("/admin/diagnose", s.handleDiagnose)
	})
}
