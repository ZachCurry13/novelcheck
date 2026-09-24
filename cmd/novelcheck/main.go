// Command novelcheck serves the NovelCheck web app and background workers.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/calibre"
	"github.com/zachcurry13/novelcheck/internal/config"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/updates"
	"github.com/zachcurry13/novelcheck/internal/version"
	"github.com/zachcurry13/novelcheck/web"
)

func main() {
	cfg := config.Load()
	database, err := db.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()
	st := store.New(database)

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
	go worker.Run(ctx)
	syncer := &calibre.Syncer{Store: st, Dir: cfg.CalibreDir}
	go syncer.Loop(ctx)

	srv := &api.Server{
		Cfg:     cfg,
		Store:   st,
		Auth:    &auth.Manager{Store: st, SessionDays: cfg.SessionDays},
		Worker:  worker,
		Syncer:  syncer,
		Updates: updates.New(),
		Web:     web.FS(),
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
}
