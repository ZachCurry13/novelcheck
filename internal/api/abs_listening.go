package api

import (
	"context"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/audiobookshelf"
	"github.com/zachcurry13/novelcheck/internal/auth"
)

// audioCover returns the cover Audiobookshelf has for an item, kept after the first fetch.
func (s *Server) audioCover(ctx context.Context, item string) (string, os.FileInfo, bool) {
	name := "abs-" + strings.ToLower(strings.ReplaceAll(item, "_", "-"))
	if p, info, ok := s.Covers.Stored(name); ok {
		return p, info, true
	}
	c, ok := audiobookshelf.FromSettings(s.Store)
	if !ok {
		return "", nil, false
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	jpg, err := c.Cover(ctx, item)
	if err != nil {
		return "", nil, false
	}
	return s.Covers.Keep(name, jpg)
}

// listeningBook is one line of "Continue listening".
type listeningBook struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Narrator    string  `json:"narrator"`
	DurationSec int     `json:"duration_sec"`
	SpiceLevel  *int    `json:"spice_level"`
	At          float64 `json:"at"`
	Fraction    float64 `json:"progress"`
	Updated     int64   `json:"updated"`
}

// handleListening: GET /api/listening, the audiobooks the person is partway through, latest first.
func (s *Server) handleListening(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	c, ok := s.absFor(u)
	out := []listeningBook{}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false, "books": out})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	prog, _, err := c.Me(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": true, "error": err.Error(), "books": out})
		return
	}
	ids := s.Store.AudioBookIDs()
	for item, p := range prog {
		bid, found := ids[item]
		if !found || p.Finished || p.CurrentTime < 5 {
			continue
		}
		b, err := s.Store.BookByID(bid, u) // kids' rules apply
		if err != nil {
			continue
		}
		out = append(out, listeningBook{ID: b.ID, Title: b.Title, Author: b.Author, Narrator: b.Narrator, DurationSec: b.DurationSec,
			SpiceLevel: b.SpiceLevel, At: p.CurrentTime, Fraction: p.Fraction, Updated: p.Updated})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated > out[j].Updated })
	if len(out) > 12 {
		out = out[:12]
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "books": out})
}
