package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/ollama"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// modelsInUse are the models NovelCheck is set to use (main, fallbacks,
// backup, Deep Scan), so they aren't deleted by accident.
func (s *Server) modelsInUse() map[string]bool {
	used := map[string]bool{}
	for _, ai := range s.Store.AIConfigs() {
		for _, m := range ai.Models {
			used[m] = true
		}
	}
	if m := strings.TrimSpace(s.Store.Setting(store.KeyDeepModel)); m != "" {
		used[m] = true
	}
	return used
}

// handleOllamaModels lists the installed models with their disk space.
func (s *Server) handleOllamaModels(w http.ResponseWriter, r *http.Request) {
	base, err := ollama.Normalize(r.URL.Query().Get("url"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	models, err := ollama.Installed(ctx, base)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "couldn't reach Ollama at "+base+": "+err.Error())
		return
	}
	used := s.modelsInUse()
	type row struct {
		ollama.Model
		InUse bool `json:"in_use"`
	}
	out := make([]row, 0, len(models))
	var total int64
	for _, m := range models {
		out = append(out, row{m, used[m.Name]})
		total += m.Size
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": out, "total": total})
}

// handleOllamaDelete removes a model, unless NovelCheck is set to use it.
func (s *Server) handleOllamaDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL   string `json:"url"`
		Model string `json:"model"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	base, err := ollama.Normalize(body.URL)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.modelsInUse()[body.Model] {
		writeErr(w, http.StatusConflict, "NovelCheck is set to use "+body.Model+". Choose another model under AI & Scans first.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := ollama.Delete(ctx, base, body.Model); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
