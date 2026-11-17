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

// "Use Google's free Gemini as the backup AI": Google's free tier has limits that differ by account and model (shown
// in AI Studio, not published as fixed numbers). Google retires model names for new accounts (gemini-2.5-flash was one), so the models are not guessed: the
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

// pickGemini chooses the models, newest first: the two newest "flash-lite" models (each has its own daily allowance
// on the free tier: 500 requests a day each in the owner's account), then the biggest Gemma 4 (a much larger daily
// allowance but a small per-minute token limit), and a "flash" only when no flash-lite exists.
func pickGemini(all []string) []string {
	bad := []string{"preview", "image", "tts", "embed", "live", "audio", "exp", "thinking", "robotics", "computer"}
	ok := func(n string) bool {
		if !strings.HasPrefix(n, "gemini-") && !strings.HasPrefix(n, "gemma-") {
			return false
		}
		for _, b := range bad {
			if strings.Contains(n, b) {
				return false
			}
		}
		return true
	}
	var lites []string
	var flash, gemma string
	for _, n := range all {
		switch {
		case !ok(n):
		case strings.HasPrefix(n, "gemini-") && strings.Contains(n, "flash-lite") && len(lites) < 2:
			lites = append(lites, n)
		case strings.HasPrefix(n, "gemini-") && strings.Contains(n, "flash") && !strings.Contains(n, "lite") && flash == "":
			flash = n
		case strings.HasPrefix(n, "gemma-4") && (gemma == "" || (strings.Contains(n, "31b") && !strings.Contains(gemma, "31b"))):
			gemma = n
		}
	}
	out := lites
	if len(out) == 0 && flash != "" {
		out = append(out, flash)
	}
	if gemma != "" {
		out = append(out, gemma)
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

// "Make Google the main AI, and my own AI the backup" (Admin → AI & Scans): the free Gemini models first, and the AI
// that ran before (usually the home machine) stays as the safety net. The pause between books becomes 4 seconds
// (Google's free tier allows 15 requests a minute). Nothing already rated changes.

// handleMainUseGoogle: POST /api/admin/main-ai/use-google {"key"}.
func (s *Server) handleMainUseGoogle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	key := strings.TrimSpace(body.Key)
	base := googleBase
	if s.GoogleBase != "" {
		base = s.GoogleBase
	}
	isGoogle := func(url string) bool {
		return url == base || strings.Contains(url, "generativelanguage.googleapis.com")
	}
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
	all, err := googleModels(r.Context(), base, key)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "Couldn't ask Google which models this key can use: "+err.Error())
		return
	}
	models := pickGemini(all)
	if len(models) == 0 {
		writeErr(w, http.StatusBadGateway, "Google lists no usable Gemini model for this key")
		return
	}
	set := map[string]string{
		store.KeyLLMProvider: "openai", store.KeyLLMBaseURL: base, store.KeyLLMAPIKey: key, store.KeyLLMModel: models[0],
		store.KeyLLMFallbackModel: strings.Join(models[1:], ","), store.KeyLLMJSONMode: "false",
		store.KeyPriceInputPerM: "0", store.KeyPriceOutputPerM: "0", store.KeyMainWatts: "0", store.KeyScanDelaySeconds: "4",
	}
	// the AI that was the main one becomes the backup, unless it already was Google
	if prev := s.Store.Setting(store.KeyLLMBaseURL); !isGoogle(prev) && strings.TrimSpace(prev) != "" && len(s.Store.LLMModels()) > 0 {
		json := "false"
		if s.Store.SettingBool(store.KeyLLMJSONMode) {
			json = "true"
		}
		set[store.KeyBackupEnabled], set[store.KeyBackupProvider], set[store.KeyBackupBaseURL] = "true", s.Store.Setting(store.KeyLLMProvider), prev
		set[store.KeyBackupAPIKey], set[store.KeyBackupModel] = s.Store.Setting(store.KeyLLMAPIKey), strings.Join(s.Store.LLMModels(), ",")
		set[store.KeyBackupJSONMode] = json
		set[store.KeyBackupPriceIn], set[store.KeyBackupPriceOut] = s.Store.Setting(store.KeyPriceInputPerM), s.Store.Setting(store.KeyPriceOutputPerM)
		set[store.KeyBackupWatts] = s.Store.Setting(store.KeyMainWatts)
	}
	if err := s.Store.SetSettings(set); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"models": strings.Join(models, ","), "backup": set[store.KeyBackupBaseURL]})
}
