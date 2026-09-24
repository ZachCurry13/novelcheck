package api

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/ollama"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleOllamaFind looks for Ollama servers (optionally at ?url=) and lists
// their models.
func (s *Server) handleOllamaFind(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	var extra []string
	if u := r.URL.Query().Get("url"); u != "" {
		n, err := ollama.Normalize(u)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		extra = append(extra, n)
	}
	if cur := s.Store.Setting(store.KeyLLMBaseURL); ollamaBase(cur) != "" {
		extra = append(extra, cur)
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": ollama.Discover(ctx, extra...)})
}

type ollamaModelReq struct {
	URL    string   `json:"url"`
	Model  string   `json:"model"`
	Models []string `json:"models"` // "use": in order, main model first
	Target string   `json:"target"` // "use": "" = main AI, "backup" = backup AI
}

func (s *Server) handleOllamaPull(w http.ResponseWriter, r *http.Request) {
	var body ollamaModelReq
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	base, err := ollama.Normalize(body.URL)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Pulls.Start(base, strings.TrimSpace(body.Model)); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, s.Pulls.Status())
}

func (s *Server) handleOllamaPullStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Pulls.Status())
}

// handleOllamaUse points the LLM settings at an Ollama server and model.
func (s *Server) handleOllamaUse(w http.ResponseWriter, r *http.Request) {
	var body ollamaModelReq
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	base, err := ollama.Normalize(body.URL)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	srv, err := ollama.Probe(ctx, base)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "can't reach Ollama at "+base)
		return
	}
	// Models in order: the first is the main one, the rest are fallbacks.
	models := body.Models
	if len(models) == 0 {
		models = []string{body.Model}
	}
	var chosen []string
	for _, m := range models {
		m = strings.TrimSpace(m)
		if m == "" || slices.Contains(chosen, m) {
			continue
		}
		if !slices.Contains(srv.Models, m) && !slices.Contains(srv.Models, m+":latest") {
			writeErr(w, http.StatusBadRequest, m+" isn't downloaded on this Ollama server yet")
			return
		}
		chosen = append(chosen, m)
	}
	if len(chosen) == 0 {
		writeErr(w, http.StatusBadRequest, "pick at least one model")
		return
	}
	model, fallbacks := chosen[0], strings.Join(chosen[1:], ",")
	if body.Target == "backup" {
		for k, v := range map[string]string{
			store.KeyBackupEnabled: "true", store.KeyBackupProvider: "openai", store.KeyBackupBaseURL: base + "/v1",
			store.KeyBackupAPIKey: "", store.KeyBackupModel: strings.Join(chosen, ","), store.KeyBackupJSONMode: "true",
			store.KeyBackupPriceIn: "0", store.KeyBackupPriceOut: "0",
		} {
			if err := s.Store.SetSetting(k, v); err != nil {
				writeStoreErr(w, err)
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"base_url": base + "/v1", "model": strings.Join(chosen, ","), "target": "backup"})
		return
	}
	for k, v := range map[string]string{
		store.KeyLLMProvider: "openai", store.KeyLLMBaseURL: base + "/v1", store.KeyLLMAPIKey: "",
		store.KeyLLMModel: model, store.KeyLLMFallbackModel: fallbacks, store.KeyLLMJSONMode: "true",
		store.KeyPriceInputPerM: "0", store.KeyPriceOutputPerM: "0",
	} {
		if err := s.Store.SetSetting(k, v); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_url": base + "/v1", "model": model, "fallback": fallbacks})
}
