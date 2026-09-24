package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/delivery"
	"github.com/zachcurry13/novelcheck/internal/store"
)

const secretMask = "********"

// defaultTokensPerBook seeds the cost projection before any calls are made.
const defaultTokensPerBook = 900

var numericKeys = map[string]bool{
	store.KeyPriceInputPerM: true, store.KeyPriceOutputPerM: true, store.KeyBatchSize: true,
	store.KeyTokensPerHour: true, store.KeyScanDelaySeconds: true, store.KeySMTPPort: true,
	store.KeyCalibrePollHours: true, store.KeySessionDays: true,
}

// editableKeys are the settings the admin panel may change.
var editableKeys = func() map[string]bool {
	m := map[string]bool{}
	for k := range store.Defaults {
		m[k] = true
	}
	for k := range store.SecretKeys {
		m[k] = true
	}
	for _, k := range []string{store.KeySMTPHost, store.KeySMTPUser, store.KeySMTPFrom} {
		m[k] = true
	}
	delete(m, store.KeyCalibreLastSync)
	delete(m, store.KeyCalibreLastResult)
	delete(m, store.KeyCalibreLibraryPath) // set via PUT /admin/calibre/library (validated)
	return m
}()

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
	spent := float64(usage.TotalPrompt)*pin/1e6 + float64(usage.TotalCompletion)*pout/1e6
	perBook := usage.AvgPerBook
	if perBook == 0 {
		perBook = defaultTokensPerBook
	}
	remaining := counts["pending"] + counts["queued"] + counts["error"]
	// Prompts dominate: assume ~80% input / 20% output tokens per call.
	projected := float64(remaining) * perBook * (0.8*pin + 0.2*pout) / 1e6
	var lastSync any
	_ = json.Unmarshal([]byte(s.Store.Setting(store.KeyCalibreLastResult)), &lastSync)
	writeJSON(w, http.StatusOK, map[string]any{
		"counts":            counts,
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

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	all, err := s.Store.AllSettings()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	out := map[string]string{}
	for k := range editableKeys {
		v := all[k]
		if store.SecretKeys[k] && v != "" {
			v = secretMask
		}
		out[k] = v
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if !readJSON(w, r, &body, 64<<10) {
		return
	}
	for k, v := range body {
		if !editableKeys[k] {
			writeErr(w, http.StatusBadRequest, "unknown setting "+k)
			return
		}
		v = strings.TrimSpace(v)
		if numericKeys[k] {
			if f, err := strconv.ParseFloat(v, 64); err != nil || f < 0 {
				writeErr(w, http.StatusBadRequest, k+" must be a non-negative number")
				return
			}
		}
		if k == store.KeyLLMProvider && v != "openai" && v != "anthropic" {
			writeErr(w, http.StatusBadRequest, "llm_provider must be openai or anthropic")
			return
		}
		if k == store.KeySessionDays {
			if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 365 {
				writeErr(w, http.StatusBadRequest, "stay signed in must be between 1 and 365 days")
				return
			}
		}
		if k == store.KeyLLMJSONMode || k == store.KeyCheckUpdates {
			if _, err := strconv.ParseBool(v); err != nil {
				writeErr(w, http.StatusBadRequest, k+" must be true or false")
				return
			}
		}
		body[k] = v
	}
	for k, v := range body {
		if store.SecretKeys[k] && v == secretMask {
			continue // unchanged secret
		}
		if err := s.Store.SetSetting(k, v); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAnalyzeBatch(w http.ResponseWriter, r *http.Request) {
	n := queryInt(r, "size")
	if n <= 0 {
		n = s.Store.SettingInt(store.KeyBatchSize)
	}
	n = min(max(n, 1), 500)
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
	go func() {
		if _, err := s.Syncer.Run(); err != nil {
			log.Printf("manual calibre sync failed: %v", err)
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": true})
}

func (s *Server) handleSMTPTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		To string `json:"to"`
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
	cfg := delivery.SMTPConfig{
		Host: s.Store.Setting(store.KeySMTPHost), Port: s.Store.Setting(store.KeySMTPPort),
		Username: s.Store.Setting(store.KeySMTPUser), Password: s.Store.Setting(store.KeySMTPPassword),
		From: s.Store.Setting(store.KeySMTPFrom),
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
