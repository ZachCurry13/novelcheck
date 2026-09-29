// Command novelcheck serves the NovelCheck web app and background workers.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/zachcurry13/novelcheck/internal/aitools"
	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/calibre"
	"github.com/zachcurry13/novelcheck/internal/cli"
	"github.com/zachcurry13/novelcheck/internal/collections"
	"github.com/zachcurry13/novelcheck/internal/config"
	"github.com/zachcurry13/novelcheck/internal/covers"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/deepread"
	"github.com/zachcurry13/novelcheck/internal/discover"
	"github.com/zachcurry13/novelcheck/internal/genrefill"
	"github.com/zachcurry13/novelcheck/internal/push"
	"github.com/zachcurry13/novelcheck/internal/safemode"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/suggest"
	"github.com/zachcurry13/novelcheck/internal/sysinfo"
	"github.com/zachcurry13/novelcheck/internal/tunnel"
	"github.com/zachcurry13/novelcheck/internal/updates"
	"github.com/zachcurry13/novelcheck/internal/version"
	"github.com/zachcurry13/novelcheck/web"
)

func main() {
	cfg := config.Load()
	// Maintenance commands (novelcheck help) run and exit instead of the web app.
	if code, handled := cli.Run(os.Args[1:], cfg.DataDir, os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	safeReason := safemode.Starting(cfg.DataDir, time.Now())
	database, err := db.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()
	st := store.New(database)
	// NOVELCHECK_SESSION_DAYS is the default until an admin changes it in the app.
	store.Defaults[store.KeySessionDays] = strconv.Itoa(cfg.SessionDays)

	if err := auth.Bootstrap(st, cfg.AdminUser, cfg.AdminPassword); err != nil {
		log.Fatalf("bootstrap admin: %v", err)
	}
	// Books left mid-analysis by a previous shutdown go back to Pending.
	if n, err := st.ResetQueued(); err == nil && n > 0 {
		log.Printf("reset %d interrupted analyses to pending", n)
	}
	_ = st.PurgeExpiredSessions()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	worker := analyzer.New(st)
	// AI collections: fills run in the background; weekly ideas only while
	// the AI is idle and within automatic rating's hours.
	collectionsSvc := collections.New(st)
	collectionsSvc.Local = func() bool { return analyzer.LocalAI(st) }
	collectionsSvc.Allowed = func() bool {
		return worker.Status().State == "idle" && analyzer.RateHoursOpen(st, time.Now())
	}
	aiTools := aitools.New(st)
	syncer := &calibre.Syncer{Store: st, Dir: cfg.CalibreDir}
	syncer.NewBooks = func() bool {
		worker.Kick()
		return analyzer.AutoRateOn(st)
	}

	sampler := sysinfo.New(cfg.DataDir)
	go sampler.Loop(ctx)

	// Built-in Cloudflare Tunnel for access from outside the home network.
	tun := tunnel.New()
	tun.OnChange = func(state, lastErr string) {
		switch state {
		case tunnel.StateConnected:
			st.Resolve("tunnel")
		case tunnel.StateRetrying:
			st.Notify("error", "tunnel", "Remote access lost its connection to Cloudflare: "+lastErr, "#/admin")
		}
	}
	if st.SettingBool(store.KeyTunnelEnabled) {
		if err := tun.Apply(true, st.Setting(store.KeyTunnelToken)); err != nil {
			log.Printf("remote access not started: %v", err)
		}
	}
	defer tun.Stop()

	deep := deepread.New(st, cfg.CalibreDir)
	discoverSvc := discover.New(st, func(ids ...int64) { worker.Enqueue(false, ids...) })

	// Background work, unless safe mode is on (then an admin starts it by
	// leaving safe mode).
	bg := &background{st: st, worker: worker, collections: collectionsSvc, aiTools: aiTools, syncer: syncer, deep: deep, discover: discoverSvc}
	safe := safemode.NewState(cfg.DataDir, safeReason, sync.OnceFunc(func() { bg.start(ctx) }))
	if safeReason == "" {
		bg.start(ctx)
	} else {
		log.Printf("SAFE MODE (%s): background work is off until an admin leaves safe mode", safeReason)
		st.Notify("warning", "safe-mode", "NovelCheck started in safe mode: Calibre sync, rating and Deep Scans are paused. Leave safe mode from the banner at the top.", "#/admin")
	}
	// A start that runs a while (or shuts down normally) wasn't a crash.
	go func() {
		select {
		case <-ctx.Done():
		case <-time.After(safemode.HealthyAt):
			safemode.Clean(cfg.DataDir)
		}
	}()

	// Phone notifications: every new 🔔 notice also goes to subscribed devices.
	pusher := push.New(st)
	st.OnNotify = pusher.FromNotice

	srv := &api.Server{
		Cfg:         cfg,
		Store:       st,
		Auth:        &auth.Manager{Store: st, SessionDays: cfg.SessionDays},
		Worker:      worker,
		Syncer:      syncer,
		Updates:     updates.New(),
		Tunnel:      tun,
		SysInfo:     sampler,
		Push:        pusher,
		Deep:        deep,
		Suggest:     suggest.New(st),
		Covers:      covers.Cache{Dir: filepath.Join(cfg.DataDir, "covers")},
		Genres:      &genrefill.Filler{Store: st},
		Discover:    discoverSvc,
		Collections: collectionsSvc,
		AITools:     aiTools,
		Safe:        safe,
		Web:         web.FS(),
	}
	srv.Pulls.OnError = func(model string, err error) {
		st.Notify("warning", "ollama", "Downloading "+model+" in Ollama failed: "+err.Error(), "#/admin")
	}
	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdown)
	}()
	log.Printf("NovelCheck %s listening on %s (data: %s, calibre: %s)", version.Version, cfg.Addr, cfg.DataDir, cfg.CalibreDir)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	safemode.Clean(cfg.DataDir) // a normal shutdown
}
