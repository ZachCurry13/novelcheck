package api

import (
	"net/http"
	"strings"

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
// models. The fast "flash-lite" model comes first (it answers in seconds; the bigger Flash models think for a minute
// and can run out the health check's time), the bigger one is the second try. Prompts sent to the free tier may be
// used by Google to improve its products. The key is Google AI Studio's; it is taken from the box, else from the
// backup or main AI when that already points at Google.
const googleBase = "https://generativelanguage.googleapis.com/v1beta/openai"

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
	if err := s.Store.SetSettings(map[string]string{
		store.KeyBackupEnabled: "true", store.KeyBackupProvider: "openai", store.KeyBackupBaseURL: googleBase,
		store.KeyBackupAPIKey: key, store.KeyBackupModel: "gemini-2.5-flash-lite,gemini-2.5-flash", store.KeyBackupJSONMode: "false",
		store.KeyBackupPriceIn: "0", store.KeyBackupPriceOut: "0", store.KeyBackupWatts: "0",
	}); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_url": googleBase, "models": "gemini-2.5-flash-lite,gemini-2.5-flash"})
}
