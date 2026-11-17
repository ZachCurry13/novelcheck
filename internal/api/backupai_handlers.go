package api

import (
	"net/http"

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
