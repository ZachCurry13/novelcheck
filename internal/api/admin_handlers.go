package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/delivery"
	"github.com/zachcurry13/novelcheck/internal/store"
)

const secretMask = "********"

// defaultTokensPerBook seeds the cost projection before any calls are made.
const defaultTokensPerBook = 900

func (s *Server) handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	counts, err := s.Store.StatusCounts()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	usage, err := s.Store.Usage()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	pin, pout := s.Store.SettingFloat(store.KeyPriceInputPerM), s.Store.SettingFloat(store.KeyPriceOutputPerM)
	spent := s.Store.SpentUSD()
	perBook := usage.AvgPerBook
	if perBook == 0 {
		perBook = defaultTokensPerBook
	}
	remaining := counts["pending"] + counts["queued"] + counts["error"]
	// Prompts dominate: assume ~80% input / 20% output tokens per call.
	projected := float64(remaining) * perBook * (0.8*pin + 0.2*pout) / 1e6
	rerate, _ := s.Store.RerateCandidates()
	aiRated, _ := s.Store.AIRatedIDs()
	var lastSync any
	_ = json.Unmarshal([]byte(s.Store.Setting(store.KeyCalibreLastResult)), &lastSync)
	writeJSON(w, http.StatusOK, map[string]any{
		"counts":            counts,
		"rerate_candidates": len(rerate),
		"ai_rated":          len(aiRated),
		"changed_books":     s.Store.ChangedSinceRated(),
		"title_fixes":       s.Store.CountTitleFixes(),
		"cover_reports":     s.Store.CountCoverReports(),
		"genres":            s.Genres.Status(),
		"non_english":       s.nonEnglishCount(),
		"pending_deletes":   s.Store.PendingDeleteCount(),
		"pending_deep":      s.Store.PendingDeepRequests(),
		"usage":             usage,
		"tokens_per_hour":   s.Store.SettingInt(store.KeyTokensPerHour),
		"cost_spent":        spent,
		"cost_projected":    projected,
		"tokens_projected":  int(float64(remaining) * perBook),
		"worker":            s.Worker.Status(),
		"calibre_available": s.Syncer.Available(),
		"calibre_library":   s.Syncer.LibraryDir(),
		"calibre_last_sync": lastSync,
	})
}

// maxBatch caps one batch, even when the batch size is 0 ("all waiting").
const maxBatch = 500

func (s *Server) handleAnalyzeBatch(w http.ResponseWriter, r *http.Request) {
	n := queryInt(r, "size")
	if n <= 0 {
		n = s.Store.SettingInt(store.KeyBatchSize) // blank or unreadable reads as 0
	}
	if n <= 0 || n > maxBatch {
		n = maxBatch // 0 means every waiting book (up to the cap)
	}
	ids, err := s.Store.QueueForAnalysis(n)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	s.Worker.Enqueue(false, ids...)
	writeJSON(w, http.StatusAccepted, map[string]int{"queued": len(ids)})
}

func (s *Server) handleWipeQueue(w http.ResponseWriter, r *http.Request) {
	n, err := s.Worker.Wipe()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"reset": n})
}

// handleCalibreSync starts a manual sync in the background.
func (s *Server) handleCalibreSync(w http.ResponseWriter, r *http.Request) {
	if !s.Syncer.Available() {
		writeErr(w, http.StatusNotFound, "no Calibre library (metadata.db) at "+s.Syncer.LibraryDir()+"; choose the library folder under Calibre Library")
		return
	}
	// ?wait=1 syncs before answering (used by "Check again" on Duplicates).
	if r.URL.Query().Get("wait") == "1" {
		res, err := s.Syncer.Run()
		if err != nil {
			writeErr(w, http.StatusBadGateway, "Calibre sync failed: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"result": res, "at": s.Store.Setting(store.KeyCalibreLastSync)})
		return
	}
	go func() {
		if _, err := s.Syncer.Run(); err != nil {
			log.Printf("manual calibre sync failed: %v", err)
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": true})
}

func (s *Server) handleSMTPTest(w http.ResponseWriter, r *http.Request) {
	// The SMTP fields are what's on screen (maybe not saved yet); blanks and
	// the masked password fall back to the saved settings.
	var body struct {
		To       string `json:"to"`
		Host     string `json:"smtp_host"`
		Port     string `json:"smtp_port"`
		Username string `json:"smtp_username"`
		Password string `json:"smtp_password"`
		From     string `json:"smtp_from"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	if body.To == "" {
		body.To = auth.UserFrom(r).KindleEmail
	}
	tmp, err := os.CreateTemp("", "novelcheck-test-*.txt")
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	defer os.Remove(tmp.Name())
	fmt.Fprintf(tmp, "NovelCheck SMTP test sent %s\n", time.Now().Format(time.RFC1123))
	tmp.Close()
	cfg := s.smtpConfig()
	pick := func(onScreen string, saved *string) {
		if v := strings.TrimSpace(onScreen); v != "" && v != secretMask {
			*saved = v
		}
	}
	pick(body.Host, &cfg.Host)
	pick(body.Port, &cfg.Port)
	pick(body.Username, &cfg.Username)
	pick(body.From, &cfg.From)
	if body.Password != "" && body.Password != secretMask {
		cfg.Password = body.Password
	}
	if err := delivery.SendFile(cfg, body.To, tmp.Name()); err != nil {
		writeErr(w, http.StatusBadGateway, "SMTP test failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"sent_to": body.To})
}

// handleBackup snapshots the live database with VACUUM INTO and streams it.
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	dir, err := os.MkdirTemp("", "novelcheck-backup-")
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	defer os.RemoveAll(dir)
	snap := filepath.Join(dir, "novelcheck.db")
	if _, err := s.Store.DB.Exec(`VACUUM INTO ?`, snap); err != nil {
		writeStoreErr(w, err)
		return
	}
	f, err := os.Open(snap)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	name := "novelcheck-" + time.Now().Format("20060102-150405") + ".db"
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeContent(w, r, "", st.ModTime(), f)
}

// handleErrors lists books that failed to rate, grouped by the reason.
func (s *Server) handleErrors(w http.ResponseWriter, r *http.Request) {
	groups, err := s.Store.ErrorGroups()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

// handleRetryErrors queues failed books again: every one, or only those that
// failed with {"message": "..."}.
func (s *Server) handleRetryErrors(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Message string `json:"message"`
	}
	if r.ContentLength > 0 && !readJSON(w, r, &body, 16<<10) {
		return
	}
	ids, err := s.Store.ErrorBookIDs(body.Message)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	s.Worker.Enqueue(false, ids...)
	s.Store.Resolve("analysis")
	writeJSON(w, http.StatusOK, map[string]int{"queued": len(ids)})
}
