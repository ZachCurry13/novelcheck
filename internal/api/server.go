// Package api wires the HTTP routes for the JSON API and embedded web app.
package api

import (
	"io/fs"
	"net/http"
	"sync"

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
	Suggest     *suggest.Service     // Suggested Reads under Up Next
	Discover    *discover.Service    // the Discover tab's lists; nil in tests that don't need it
	Collections *collections.Service // AI-filled collections and weekly ideas
	AITools     *aitools.Service     // model updates and the speed test
	Safe        *safemode.State      // safe mode: background work off until an admin leaves it
	Web         fs.FS                // embedded static assets
	logins      *loginLimiter
	formats     formatJob // format cleanup's background job
	dav         davState  // KOReader statistics folders
	pins        pinTries  // wrong PINs on family devices
	etags       sync.Map  // static file name → ETag
	buildOnce   sync.Once // buildID, computed once
	build       string
	indexOnce   sync.Once // index.html with versioned addresses
	index       []byte
}

func (s *Server) Router() http.Handler {
	s.logins = newLoginLimiter()
	r := chi.NewRouter()
	r.Use(realIP(s.Cfg.TrustProxy))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(securityHeaders)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	// KOReader's OPDS catalog: the private token in the address stands in for signing in.
	r.Get("/opds/{token}", s.handleOPDSFeed)
	r.Get("/opds/{token}/book/{id}/{name}", s.handleOPDSBook)
	// KOReader progress sync: signed in with the NovelCheck name and a sync code.
	r.Route("/kosync", func(r chi.Router) {
		r.Use(noStore)
		r.Post("/users/create", s.handleKosyncCreate)
		r.Get("/users/auth", s.handleKosyncAuth)
		r.Put("/syncs/progress", s.handleKosyncPut)
		r.Get("/syncs/progress/{document}", s.handleKosyncGet)
	})
	// KOReader's reading statistics (Cloud sync): a private WebDAV folder, same sign-in.
	r.Handle("/dav", http.HandlerFunc(s.handleDAV))
	r.Handle("/dav/*", http.HandlerFunc(s.handleDAV))

	r.Route("/api", func(r chi.Router) {
		r.Use(noStore, cors(s.Cfg.CORSOrigins), csrfGuard)
		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/logout", s.handleLogout)
		r.Get("/setup", s.handleSetupStatus)
		r.Post("/setup", s.handleSetup)
		// "Who's reading?": only answers on a device a parent marked.
		r.Get("/family", s.handleFamily)
		r.Post("/family/switch", s.handleFamilySwitch)

		r.Group(func(r chi.Router) {
			r.Use(s.Auth.RequireUser)
			r.Get("/me", s.handleMe)
			r.Put("/me/pin", s.handleMyPIN)
			r.Put("/me/password", s.handleChangePassword)
			r.Put("/me/delivery", s.handleUpdateDelivery)
			r.Put("/me/guide-seen", s.handleGuideSeen)
			r.Put("/me/start-page", s.handleStartPage)
			r.Put("/me/appearance", s.handleAppearance)
			r.Get("/me/searches", s.handleSearches)
			r.Post("/me/searches", s.handleAddSearch)
			r.Delete("/me/searches", s.handleClearSearches)
			r.Get("/me/opds", s.handleMyOPDS)
			r.Post("/me/opds/reset", s.handleResetMyOPDS)
			r.Get("/me/kosync", s.handleMyKosync)
			r.Post("/me/kosync", s.handleMyKosync)
			r.Get("/me/koreader-books", s.handleMyKOReaderBooks)

			r.Get("/books", s.handleListBooks)
			r.Get("/books/facets", s.handleBookFacets)
			r.Get("/books/states", s.handleBookStates)
			r.Get("/collections", s.handleListCollections)
			r.Get("/series", s.handleSeriesList)
			r.Get("/series/one", s.handleSeries)
			r.Get("/collections/{id}", s.handleGetCollection)
			r.Get("/events", s.handleListEvents)
			r.Get("/events/{id}", s.handleGetEvent)
			r.Post("/books/bulk", s.handleBulkBooks)
			r.Get("/books/{id}", s.handleGetBook)
			r.Get("/books/{id}/download", s.handleDownload)
			r.Get("/books/{id}/cover", s.handleCover)
			r.Post("/books/{id}/cover-report", s.handleReportCover)
			r.Post("/books/{id}/delete-request", s.handleRequestDelete)
			r.Delete("/books/{id}/delete-request", s.handleCancelDelete)
			r.Get("/catalogs", s.handleListCatalogs)
			r.Get("/age-groups", s.handleAgeGroups)
			r.Get("/updates", s.handleUpdates)
			r.Get("/flags", s.handleListFlags)
			r.Get("/content", s.handleContentCatalog)
			r.With(s.requireModule(store.KeyModuleDiscover, "Discover")).Get("/discover", s.handleDiscover)
			r.Post("/books/{id}/wish", s.handleAddWish)
			r.Delete("/books/{id}/wish", s.handleRemoveWish)
			r.Get("/wishlist", s.handleWishlist)
			r.Get("/books/{id}/deep-scan", s.handleBookDeepScan)
			r.Post("/books/{id}/deep-scan", s.handleStartDeepScan)
			r.Get("/push", s.handlePushStatus)
			r.Post("/push/subscribe", s.handlePushSubscribe)
			r.Post("/push/unsubscribe", s.handlePushUnsubscribe)
			r.Post("/push/test", s.handlePushTest)
			r.Post("/problems", s.handleReportProblem)

			r.Group(func(r chi.Router) {
				r.Use(s.requireModule(store.KeyModuleQueue, "The reading queue"))
				r.Get("/queue", s.handleListQueue)
				r.Post("/queue", s.handleEnqueue)
				r.Put("/queue/order", s.handleReorderQueue)
				r.Delete("/queue/{id}", s.handleDequeue)
				r.Post("/queue/{id}/start", s.handleStartReading)
				r.Post("/queue/{id}/finish", s.handleFinishReading)
				r.Group(func(r chi.Router) {
					r.Use(s.requireModule(store.KeyModuleSuggest, "Suggested Reads"))
					r.Get("/suggestions", s.handleSuggestions)
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
				r.Use(auth.RequireManager)
				r.Post("/check", s.handleCheck)
				r.Get("/admin/deep-scans/{id}/passage", s.handleDeepPassage) // parents read a scanned part
				r.Get("/activity", s.handleActivity)
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

				r.Get("/admin/status", s.handleAdminStatus)
				r.Get("/admin/calibre/duplicates", s.handleDuplicates)
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
