// Package api wires the HTTP routes for the JSON API and embedded web app.
package api

import (
	"io/fs"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/zachcurry13/novelcheck/internal/aitools"
	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/calibre"
	"github.com/zachcurry13/novelcheck/internal/collections"
	"github.com/zachcurry13/novelcheck/internal/config"
	"github.com/zachcurry13/novelcheck/internal/covers"
	"github.com/zachcurry13/novelcheck/internal/deepread"
	"github.com/zachcurry13/novelcheck/internal/discover"
	"github.com/zachcurry13/novelcheck/internal/genrefill"
	"github.com/zachcurry13/novelcheck/internal/libby"
	"github.com/zachcurry13/novelcheck/internal/ollama"
	"github.com/zachcurry13/novelcheck/internal/push"
	"github.com/zachcurry13/novelcheck/internal/safemode"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/suggest"
	"github.com/zachcurry13/novelcheck/internal/sysinfo"
	"github.com/zachcurry13/novelcheck/internal/tunnel"
	"github.com/zachcurry13/novelcheck/internal/updates"
)

type Server struct {
	Cfg         config.Config
	Store       *store.Store
	Auth        *auth.Manager
	Worker      *analyzer.Worker
	Syncer      *calibre.Syncer
	Updates     *updates.Checker
	Tunnel      *tunnel.Manager
	Pulls       ollama.Puller // Ollama model downloads
	SysInfo     *sysinfo.Sampler
	Push        *push.Service        // phone notifications; nil turns them off
	Deep        *deepread.Runner     // Deep Scans (full-text reading)
	Covers      covers.Cache         // shrunk book covers under /data/covers
	Genres      *genrefill.Filler    // AI genres for books without Calibre tags
	Suggest     *suggest.Service     // Suggested Reads under Nightstand
	Discover    *discover.Service    // the Discover tab's lists; nil in tests that don't need it
	Collections *collections.Service // AI-filled collections and weekly ideas
	AITools     *aitools.Service     // model updates and the speed test
	Safe        *safemode.State      // safe mode: background work off until an admin leaves it
	Web         fs.FS                // embedded static assets
	logins      *loginLimiter
	hoursSeen   sync.Map        // user id -> the stored statistics file was read once this run
	hoursTried  sync.Map        // user id -> when the stored statistics file was last read for hours (readingstats_handlers.go)
	resolvedAt  sync.Map        // user id -> when their unmatched KOReader books were last looked up (koresolve.go)
	codeGuesses *loginLimiter   // wrong sync codes (KOReader, WebDAV, plugin)
	formats     formatJob       // format cleanup's background job
	refreshMu   sync.Mutex      // one Calibre re-read from Library cleanup at a time
	ratingBusy  atomic.Bool     // a Hardcover look-up round started by hand is running
	cleanup     cleanupCache    // the Library cleanup summary, kept while the library is unchanged
	trial       trialJob        // "try the new rules on 20 books" (trial_handlers.go)
	missing     missingCache    // the books with no cover, kept a minute (coverfinder.go)
	GoogleBase  string          // Google's OpenAI-compatible address; blank = the real one (tests point it at a fake)
	slowStat    slowStatusCache // the slow part of the Admin status, kept in memory (statusslow.go)
	integ       integState      // Integrations: the last connection checks
	hcjob       hcJob           // Hardcover cleanup: the scan or apply running in the background
	rcache      respCache       // short-lived copies of the heavy list pages (respcache.go)
	later       laterSync       // Calibre re-reads after a save, in the background (synclater.go)
	covers      coverJob        // the cover finder (coverfinder.go)
	trash       trashCache      // Calibre's recycle bin, read in the background (caltrash_cache.go)
	titleMu     sync.Mutex      // one title save at a time
	dav         davState        // KOReader statistics folders
	pins        pinTries        // wrong PINs on family devices
	Libby       libby.Client    // library availability (kept a few hours)
	etags       sync.Map        // static file name → ETag
	buildOnce   sync.Once       // buildID, computed once
	build       string
	indexOnce   sync.Once // index.html with versioned addresses
	index       []byte
}

func (s *Server) Router() http.Handler {
	s.logins = newLoginLimiter()
	s.codeGuesses = newGuessLimiter()
	r := chi.NewRouter()
	r.Use(realIP(s.Cfg.TrustProxy))
	r.Use(s.errorRecorder)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(securityHeaders)

	r.Get("/bugs/{token}", s.handleBugLink)
	r.Post("/bugs/{token}/resolve", s.handleBugLinkResolve)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	// KOReader's OPDS catalog: the private token in the address stands in for signing in.
	r.Get("/opds", s.handleOPDSFeed) // short address: Basic login with name + sync code
	r.Get("/opds/book/{id}/{name}", s.handleOPDSBook)
	r.Get("/opds/{token}", s.handleOPDSFeed)
	r.Get("/opds/{token}/book/{id}/{name}", s.handleOPDSBook)
	// KOReader progress sync: signed in with the NovelCheck name and a sync code.
	r.Route("/kosync", s.kosyncRoutes)
	// KOReader's reading statistics (Cloud sync): a private WebDAV folder, same sign-in.
	r.Handle("/dav", http.HandlerFunc(s.handleDAV))
	r.Handle("/dav/*", http.HandlerFunc(s.handleDAV))

	r.Route("/api", func(r chi.Router) {
		r.Use(noStore, cors(s.Cfg.CORSOrigins), csrfGuard)
		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/logout", s.handleLogout)
		r.Get("/setup", s.handleSetupStatus)
		// The app's version, for the "new version ready" bar. (index.html's ETag says the same, but Cloudflare's
		// tunnel drops ETags, so phones reached through it never saw an update.)
		r.Get("/build", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"build": s.buildID()})
		})
		r.Post("/setup", s.handleSetup)
		// "Who's reading?": only answers on a device a parent marked.
		r.Get("/family", s.handleFamily)
		r.Post("/family/switch", s.handleFamilySwitch)

		r.Group(func(r chi.Router) {
			r.Use(s.Auth.RequireUser)
			r.Use(friendGuard) // friends' accounts: only the parts listed in friend.go
			s.userRoutes(r)
			r.Group(func(r chi.Router) {
				r.Use(s.requireModule(store.KeyModuleQueue, "The Nightstand"))
				r.Get("/queue", s.handleListQueue)
				r.Post("/queue", s.handleEnqueue)
				r.Put("/queue/order", s.handleReorderQueue)
				r.Delete("/queue/{id}", s.handleDequeue)
				r.Post("/queue/{id}/hide", s.handleHideQueueItem)
				r.Post("/queue/{id}/start", s.handleStartReading)
				r.Post("/queue/{id}/finish", s.handleFinishReading)
				r.Post("/queue/{id}/pause", s.handlePauseReading)
				r.Group(func(r chi.Router) {
					r.Use(s.requireModule(store.KeyModuleSuggest, "Suggested Reads"))
					r.With(s.cached).Get("/suggestions", s.handleSuggestions) // ~4 s to work out, kept for a short while
					r.Post("/suggestions/vote", s.handleSuggestionVote)
					r.Delete("/suggestions/votes", s.handleClearSuggestionVotes)
					r.Post("/suggestions/wish", s.handleWishSuggestion)
					r.With(s.requireModule(store.KeyModuleTaste, "The taste profile")).Get("/taste", s.handleTasteBooks)
					r.With(s.requireModule(store.KeyModuleTaste, "The taste profile")).Post("/taste", s.handleTasteMark)
				})
			})

			// Editors and admins: day-to-day management. Handlers further
			// restrict editors to kid accounts (see users_handlers.go).
			r.Group(func(r chi.Router) {
				r.With(managerOrFriend).Post("/check", s.handleCheck)
				r.With(managerOrFriend).Get("/check/jobs/{id}", s.handleCheckJob)
				r.Use(auth.RequireManager)
				r.Get("/admin/deep-scans/{id}/passage", s.handleDeepPassage) // parents read a scanned part
				r.Get("/activity", s.handleActivity)
				r.Put("/books/{id}/name", s.handleRenameBook) // fix a looked-up or listed book's title and author
				r.Post("/collections", s.handleCreateCollection)
				r.Patch("/collections/{id}", s.handleUpdateCollection)
				r.Delete("/collections/{id}", s.handleDeleteCollection)
				r.Post("/collections/{id}/keep", s.handleKeepIdea)
				r.Post("/collections/{id}/books", s.handleAddToCollection)
				r.Delete("/collections/{id}/books/{book}", s.handleRemoveFromCollection)
				r.Post("/collections/ai", s.handleStartFill)
				r.Post("/shelves/reject", s.handleRejectFromShelf)
				r.Post("/shelves/check", s.handleCheckShelf)
				r.Post("/events/preview", s.handlePreviewEvent)
				r.Post("/events", s.handleCreateEvent)
				r.Put("/events/group", s.handleGroupEvents) // a Linktree's lists as one event, or renamed
				r.Patch("/events/{id}", s.handleUpdateEvent)
				r.Post("/events/{id}/pin", s.handlePinEvent)
				r.Post("/events/{id}/archive", s.handleArchiveEvent)
				r.Delete("/events/{id}", s.handleDeleteEvent)
				r.Post("/events/{id}/books/{book}/claim", s.handleClaimEventBook)
				r.Get("/collections/ai/{job}", s.handleFillJob)
				r.Post("/wishlist/{id}/{action}", s.handleDecideWish)
				r.Get("/admin/users/{id}/opds", s.handleUserOPDS)
				r.Get("/admin/users/{id}/kosync", s.handleUserKosync)
				r.Post("/admin/users/{id}/kosync", s.handleUserKosync)
				r.Get("/admin/users/{id}/koplugin.zip", s.handleUserPluginZip)
				r.Get("/admin/users/{id}/kindle-app.zip", s.handleUserKindleAppZip)
				r.Get("/admin/kids-sharing", s.handleKidsSharing)
				r.Put("/admin/users/{id}/sharing", s.handleKidSharing)
				r.Get("/admin/users/{id}/reading", s.handleUserReading)
				r.Put("/admin/users/{id}/pin", s.handleUserPIN)
				r.Post("/me/family-device", s.handleMakeFamilyDevice)
				r.Delete("/me/family-device", s.handleMakeFamilyDevice)
				r.Get("/admin/family-devices", s.handleFamilyDevices)
				r.Delete("/admin/family-devices/{id}", s.handleFamilyDevices)
				r.Post("/catalogs", s.handleCreateCatalog)
				r.Patch("/catalogs/{id}", s.handleUpdateCatalog)
				r.Delete("/catalogs/{id}", s.handleDeleteCatalog)
				r.Delete("/catalogs/{id}/books/{book}", s.handleRemoveFromCatalog)
				r.With(s.requireModule(store.KeyModuleImport, "Importing books")).Post("/import/drive", s.handleImportDrive)
				r.With(s.requireModule(store.KeyModuleImport, "Paper books")).Post("/shelves", s.handleCreateShelf)
				r.With(s.requireModule(store.KeyModuleImport, "Paper books")).Post("/shelves/{id}/books", s.handleAddToShelf)
				r.Post("/books/{id}/analyze", s.handleAnalyzeBook)
				r.Post("/books/{id}/same-as", s.handleSameAs)
				r.Get("/box-sets", s.handleBoxSets)
				r.Get("/box-sets/{id}/match", s.handleBoxMatch)
				r.Post("/box-sets/{id}/ask", s.handleAskBoxSet)
				r.Post("/box-sets/{id}/split", s.handleSplitBoxSet)
				r.Post("/box-sets/{id}/not", func(w http.ResponseWriter, r *http.Request) { s.handleBoxSetDecision(w, r, "not") })
				r.Post("/box-sets/{id}/undo", func(w http.ResponseWriter, r *http.Request) { s.handleBoxSetDecision(w, r, "undo") })
				r.Post("/books/{id}/rerate-big", s.handleRerateBig)
				r.Put("/books/{id}/verdict", s.handleSetVerdict)
				r.Put("/books/{id}/approval", s.handleSetApproval)
				r.Put("/books/{id}/age", s.handleSetAge)
				r.Post("/books/{id}/notes", s.handleAddNote)
				r.Put("/notes/{id}", s.handleUpdateNote)
				r.Delete("/notes/{id}", s.handleDeleteNote)

				r.With(s.cached).Get("/admin/status", s.handleAdminStatus)
				r.Get("/admin/calibre/duplicates", s.handleDuplicates)
				r.Get("/admin/calibre/duplicates/details", s.handleDuplicateDetails)
				r.With(s.cached).Get("/admin/same-books", s.handleSamePairs)
				r.Post("/admin/same-books/merge", s.handleSameAction(true))
				r.Post("/admin/same-books/not", s.handleSameAction(false))
				r.Post("/admin/rerate", s.handleRerate)
				r.Get("/notifications", s.handleNotifications)
				r.Post("/notifications/read", s.handleNotificationsRead)
				r.Post("/notifications/dismiss", s.handleNotificationsDismiss)
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

			// Admins: each part behind the area the main admin gives (routes_admin.go).
			r.Group(s.adminRoutes)
		})
	})

	r.NotFound(s.serveStatic)
	return r
}
