package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// "Use my Deep Scan machine as the backup AI" (Admin → AI & Scans → Backup AI): the second machine already runs
// Ollama on the home network, so it can take over when the main one fails: free, no key, no daily limit. One press
// copies the Deep Scan machine's address and models into the backup AI (replacing what was there), prices at 0.

// handleBackupUseDeep: POST /api/admin/backup-ai/use-deep.
func (s *Server) handleBackupUseDeep(w http.ResponseWriter, r *http.Request) {
	ai, ok := s.Store.DeepAI()
	if !ok {
		writeErr(w, http.StatusBadRequest, "Set up the Deep Scan machine first (Admin → AI & Scans → Deep Scans)")
		return
	}
	json := "false"
	if ai.JSONMode {
		json = "true"
	}
	if err := s.Store.SetSettings(map[string]string{
		store.KeyBackupEnabled: "true", store.KeyBackupProvider: ai.Provider, store.KeyBackupBaseURL: ai.BaseURL,
		store.KeyBackupAPIKey: ai.APIKey, store.KeyBackupModel: s.Store.Setting(store.KeyDeepModels), store.KeyBackupJSONMode: json,
		store.KeyBackupPriceIn: "0", store.KeyBackupPriceOut: "0", store.KeyBackupWatts: s.Store.Setting(store.KeyDeepWatts),
	}); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_url": ai.BaseURL, "models": s.Store.Setting(store.KeyDeepModels)})
}

// "Use Google's free Gemini as the backup AI": Google's free tier allows a few hundred calls a day on its small
// models. Google retires model names for new accounts (gemini-2.5-flash was one), so the models are not guessed: the
// key is used to ask Google which models this account can use, and the newest "flash-lite" one (a fast model that
// answers in seconds) goes first, the newest "flash" second. Prompts sent to the free tier may be used by Google to
// improve its products. The key is Google AI Studio's; it is taken from the box, else from the backup or main AI
// when that already points at Google.
const googleBase = "https://generativelanguage.googleapis.com/v1beta/openai"

// googleModels lists the usable models of a key, newest first, as bare names.
func googleModels(ctx context.Context, base, key string) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Google answered %d: check the key", res.StatusCode)
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&body); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range body.Data {
		out = append(out, strings.TrimPrefix(m.ID, "models/"))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out, nil
}

// pickGemini chooses the fast model first and the bigger one second from a list (newest first).
func pickGemini(all []string) []string {
	bad := []string{"preview", "image", "tts", "embed", "live", "audio", "exp", "thinking", "robotics", "computer"}
	ok := func(n string) bool {
		if !strings.HasPrefix(n, "gemini-") {
			return false
		}
		for _, b := range bad {
			if strings.Contains(n, b) {
				return false
			}
		}
		return true
	}
	var lite, flash string
	for _, n := range all {
		switch {
		case ok(n) && strings.Contains(n, "flash-lite") && lite == "":
			lite = n
		case ok(n) && strings.Contains(n, "flash") && !strings.Contains(n, "lite") && flash == "":
			flash = n
		}
	}
	var out []string
	for _, n := range []string{lite, flash} {
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}

// handleBackupUseGoogle: POST /api/admin/backup-ai/use-google {"key"}.
func (s *Server) handleBackupUseGoogle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	key := strings.TrimSpace(body.Key)
	isGoogle := func(url string) bool { return strings.Contains(url, "generativelanguage.googleapis.com") }
	if key == "" || strings.Trim(key, "*") == "" {
		switch {
		case isGoogle(s.Store.Setting(store.KeyBackupBaseURL)) && s.Store.Setting(store.KeyBackupAPIKey) != "":
			key = s.Store.Setting(store.KeyBackupAPIKey)
		case isGoogle(s.Store.Setting(store.KeyLLMBaseURL)) && s.Store.Setting(store.KeyLLMAPIKey) != "":
			key = s.Store.Setting(store.KeyLLMAPIKey)
		default:
			writeErr(w, http.StatusBadRequest, "Paste a Google AI Studio key first (aistudio.google.com → Get API key)")
			return
		}
	}
	base := googleBase
	if s.GoogleBase != "" {
		base = s.GoogleBase
	}
	all, err := googleModels(r.Context(), base, key)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "Couldn't ask Google which models this key can use: "+err.Error())
		return
	}
	models := pickGemini(all)
	if len(models) == 0 {
		writeErr(w, http.StatusBadGateway, "Google lists no fast Gemini model for this key")
		return
	}
	list := strings.Join(models, ",")
	if err := s.Store.SetSettings(map[string]string{
		store.KeyBackupEnabled: "true", store.KeyBackupProvider: "openai", store.KeyBackupBaseURL: base,
		store.KeyBackupAPIKey: key, store.KeyBackupModel: list, store.KeyBackupJSONMode: "false",
		store.KeyBackupPriceIn: "0", store.KeyBackupPriceOut: "0", store.KeyBackupWatts: "0",
	}); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_url": base, "models": list})
}
