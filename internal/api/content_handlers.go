package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/content"
)

// handleContentCatalog serves the content items, the kids' preset starter
// sets and the Deep Scan amount names, so the app shares one list.
func (s *Server) handleContentCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"groups":  content.Groups,
		"presets": map[string][]string{"strict": content.StrictPreset, "young": content.YoungPreset},
		"amounts": content.AmountLabels,
	})
}

// invalidRule returns the first key that isn't a content hide rule, or "".
func invalidRule(keys []string) string {
	for _, k := range keys {
		if !content.ValidRule(k) {
			return k
		}
	}
	return ""
}
