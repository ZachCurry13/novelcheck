package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// adminRoutes are the admins' routes, each part behind the area the main
// admin gives (internal/store/areas.go). The settings are split per key
// (settingArea), and the main admin's own tools check Owner themselves.
func (s *Server) adminRoutes(r chi.Router) {
	r.Use(auth.RequireAdmin)

	r.With(ownerOnly).Post("/admin/devices/forget", s.handleForgetDevice) // a Kindle that stopped reporting
	r.Get("/admin/health", s.handleHealth)                                // 2.0: is everything working
	r.Get("/admin/backups", s.handleBackups)                              // 2.0.2: the daily copies, Back up now, the main admin's download
	r.Post("/admin/backups/now", s.handleBackupNow)
	r.Get("/admin/backups/tidy", s.handleTidyPreview) // Tidy backups: dictionaries, and files the plugin's backup holds identically
	r.Post("/admin/backups/tidy", s.handleTidyRun)
	r.Get("/admin/backups/{name}", s.handleBackupFile)
	r.Get("/admin/accuracy", s.handleAccuracy)           // 2.0: how the AI did against people's own levels
	r.Post("/admin/accuracy/{id}/use", s.handleUseLevel) // use a person's level for the book
	r.Get("/admin/settings", s.handleGetSettings)        // filtered to the admin's areas
	r.Put("/admin/settings", s.handlePutSettings)        // refused outside them
	r.Put("/admin/users/{id}/access", s.handleUserAccess)
	r.Get("/admin/activity", s.handleActivityLog)
	r.Put("/admin/activity", s.handleActivityStart)
	r.Delete("/admin/activity", s.handleActivityClear)
	r.Post("/admin/users/{id}/owner", s.handleMakeOwner)
	r.Post("/admin/users/{id}/view-as", s.handleViewAs) // see the app as them, read-only (needs the people area)

	r.Group(func(r chi.Router) { // 🤖 AI settings
		r.Use(area(store.AreaAI))
		r.Get("/admin/provider-guide", s.handleProviderGuide)
		r.With(s.cached).Get("/admin/aitools", s.handleAIToolsStatus) // asks the AI server, 3 s
		r.Post("/admin/aitools/check-updates", s.handleCheckModelUpdates)
		r.Post("/admin/aitools/updated", s.handleModelUpdated)
		r.Post("/admin/aitools/bench", s.handleStartBench)
		r.Post("/admin/flags", s.handleAddFlag)
		r.Put("/admin/flags/{id}", s.handleUpdateFlag)
		r.Delete("/admin/flags/{id}", s.handleDeleteFlag)
		r.Post("/admin/wipe-queue", s.handleWipeQueue)
		r.Post("/admin/backup-ai/use-deep", s.handleBackupUseDeep)
		r.Post("/admin/backup-ai/use-google", s.handleBackupUseGoogle)
		r.Post("/admin/main-ai/use-google", s.handleMainUseGoogle)
		r.Get("/admin/rerate/trial", s.handleTrial)
		r.Post("/admin/rerate/trial", s.handleTrial)
		r.Get("/admin/genres", s.handleGenreStatus)
		r.Post("/admin/genres/fill", s.handleGenreFill)
		r.Post("/admin/genres/hardcover", s.handleGenreHardcover) // genres from Hardcover now (the AI no longer fills them in by itself)
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
		r.Get("/admin/deep-scans/kept-below", s.handleKeptBelow)
		r.Post("/admin/deep-scans/kept-below", s.handleKeptBelow)
		r.Post("/admin/deep-scans/accept-all", s.handleAcceptAllDeepScans)
		r.Put("/admin/deep-scans/order", s.handleOrderDeepScans)
		r.Post("/admin/deep-scans/{id}/{action}", s.handleDecideDeepScan)
	})

	r.Group(func(r chi.Router) { // 📚 Calibre & cleanup
		r.Use(area(store.AreaCalibre), s.cleanupRecorder) // what a cleanup action didn't do goes to the bug link
		r.Post("/admin/server-checks", s.handleServerChecks)
		r.Get("/admin/upload/targets", s.handleUploadTargets)
		r.Post("/admin/upload/ebook", s.handleUploadEbook)
		r.Post("/admin/upload/audiobook", s.handleUploadAudiobook)
		r.Get("/admin/autopilot", s.handleAutopilot)
		r.Put("/admin/autopilot", s.handleAutopilot)
		r.Get("/admin/hc-clean", s.handleHCCleanState)
		r.Post("/admin/hc-clean/scan", s.handleHCCleanScan)
		r.Post("/admin/hc-clean/apply", s.handleHCCleanApply)
		r.Post("/admin/hc-clean/skip", s.handleHCCleanSkip)
		r.Post("/admin/hc-clean/stop", s.handleHCCleanStop)
		r.Get("/admin/calibre/browse", s.handleCalibreBrowse)
		r.Get("/admin/calibre/find", s.handleCalibreFind)
		r.Get("/admin/calibre/removal", s.handleCalibreRemoval)
		r.With(ownerOnly).Post("/admin/calibre/remove", s.handleCalibreRemove) // removes the real files: the main admin only
		r.With(ownerOnly).Post("/admin/calibre/duplicates/remove", s.handleRemoveDuplicates)
		r.With(ownerOnly).Post("/admin/calibre/duplicates/split", s.handleSplitDuplicates)
		r.Get("/admin/calibre/trash", s.handleCalibreTrash)
		r.Post("/admin/calibre/trash/restore", s.handleCalibreTrashRestore)
		r.Get("/admin/calibre/server", s.handleCalibreServerGet)
		r.Put("/admin/calibre/server", s.handleCalibreServerSave)
		r.Put("/admin/calibre/library", s.handleSetCalibreLibrary)
		r.With(s.cached).Get("/admin/covers/missing", s.handleMissingCovers)
		r.Get("/admin/covers/suggest", s.handleCoverSuggest)
		r.Get("/admin/covers/preview", s.handleCoverPreview)
		r.Post("/admin/covers/choose", s.handleCoverChoose)
		r.Post("/admin/covers/leave-out", s.handleLeaveOut)
		r.Post("/admin/covers/leave-out/reset", s.handleShowLeftOut)
		r.Post("/admin/covers/find", s.handleFindCovers)
		r.Post("/admin/covers/stop", s.handleStopCovers)
		r.Get("/admin/memory", s.handleMemory)
		r.Get("/admin/kindle-backup", s.handleKindleBackup)
		r.Get("/admin/kindle-unknown", s.handleKindleUnknown)
		r.Get("/admin/kindle-tasks", s.handleKindleTasks)
		r.Post("/admin/kindle-tasks", s.handleKindleTasks)
		r.Delete("/admin/kindle-tasks/{id}", s.handleKindleTaskCancel)
		r.Post("/admin/kindle-unknown", s.handleKindleUnknown)
		r.Get("/admin/kindle-backup/files", s.handleKindleFiles)
		r.Get("/admin/kindle-backup/file", s.handleKindleFile)
		r.Get("/admin/kindle-backup/zip", s.handleKindleZip)
		r.Put("/admin/memory", s.handleMemorySave)
		r.Post("/admin/memory/purge", s.handleMemoryPurge)
		r.With(s.cached).Get("/admin/cleanup", s.handleCleanup)
		r.Post("/admin/cleanup/refresh", s.handleCleanupRefresh)
		r.Get("/admin/formats", s.handleFormats)
		r.With(ownerOnly).Post("/admin/formats/remove", s.handleRemoveFormats)
		r.Post("/admin/formats/undo", s.handleUndoFormats)
		r.Post("/admin/formats/convert", s.handleConvertFormats)
		r.Post("/admin/formats/convert/as-is", s.handleConvertAsIs)
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
		r.Post("/admin/events-mail/use-smtp", s.handleEventsMailUseSMTP)
		r.Get("/admin/discover", s.handleDiscoverStatus)
		r.Post("/admin/discover/refresh", s.handleDiscoverRefresh)
		r.Post("/admin/discover/test", s.handleDiscoverTest)
		r.Get("/admin/audiobookshelf", s.handleABSStatus)
		r.Post("/admin/audiobookshelf/test", s.handleABSTest)
		r.Post("/admin/audiobookshelf/sync", s.handleABSSync)
		r.Get("/admin/audiobookshelf/cleanup", s.handleABSClean)
		r.Put("/admin/audiobookshelf/cleanup", s.handleABSCleanAuto)
		r.Post("/admin/audiobookshelf/cleanup/run", s.handleABSCleanRun)
		r.Post("/admin/audiobookshelf/fixes/all", s.handleABSFixAll)
		r.Post("/admin/audiobookshelf/fixes/{id}", s.handleABSFix)
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
		r.Get("/admin/integrations", s.handleIntegrations)
		r.Post("/admin/integrations/check", s.handleIntegrationsCheck)
		r.Post("/admin/integrations/db-check", s.handleDatabaseCheck)
		r.Get("/admin/diagnostics", s.handleDiagnostics)
		r.Post("/admin/diagnose", s.handleDiagnose)
		r.Get("/admin/bug-link", s.handleAdminBugLink)
		r.Post("/admin/bug-link", s.handleAdminBugLink)
		r.Delete("/admin/bug-link", s.handleAdminBugLink)
		r.Get("/admin/device-logs", s.handleDeviceLogs)
		r.Delete("/admin/device-logs", s.handleDeleteAllLogs)
		r.Get("/admin/server-errors", s.handleServerErrors)
		r.Delete("/admin/server-errors", s.handleServerErrors)
		r.Get("/admin/device-logs/{id}", s.handleDeviceLog)
		r.Delete("/admin/device-logs/{id}", s.handleDeviceLog)
	})
}

// ownerOnly keeps a route to the main admin: deleting a book's real files from the library is theirs alone.
func ownerOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.UserFrom(r).Owner {
			writeErr(w, http.StatusForbidden, onlyOwnerDeletes)
			return
		}
		next.ServeHTTP(w, r)
	})
}

const onlyOwnerDeletes = "only the main admin can delete a book's files from the library; ask them, or use Request to delete"
