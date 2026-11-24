package api

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Admin → Library → "Remembered ratings and covers": what NovelCheck keeps about books that left every library, list
// and event (a rating, a found cover) so they are not rated or searched for again; a switch to stop remembering, and
// buttons to forget ratings (all, or older than some days) and to delete the covers of books that are gone.

var bookCoverFile = regexp.MustCompile(`^book-(\d+)\.jpg(\.miss|\.none)?$`)

// orphanCovers lists the stored cover files of books that no longer exist (the cover finder's files are named by the
// book's id); with remove they are deleted too. It says how many files and how many bytes.
func (s *Server) orphanCovers(remove bool) (files int, bytes int64, kept int, keptBytes int64) {
	entries, err := os.ReadDir(s.Covers.Dir)
	if err != nil || s.Covers.Dir == "" {
		return
	}
	exist := s.Store.BookIDSet()
	for _, e := range entries {
		m := bookCoverFile.FindStringSubmatch(e.Name())
		if m == nil || e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		id, _ := strconv.ParseInt(m[1], 10, 64)
		if exist[id] {
			kept++
			keptBytes += info.Size()
			continue
		}
		if remove && os.Remove(filepath.Join(s.Covers.Dir, e.Name())) != nil {
			continue
		}
		files++
		bytes += info.Size()
	}
	return
}

func (s *Server) memoryState() map[string]any {
	n, oldest := s.Store.MemoryStats()
	files, bytes, kept, keptBytes := s.orphanCovers(false)
	out := map[string]any{"remember": s.Store.SettingBool(store.KeyRememberRatings), "ratings": n,
		"covers_kept": kept, "covers_kept_bytes": keptBytes, "covers_orphans": files, "covers_orphan_bytes": bytes}
	if !oldest.IsZero() {
		out["ratings_oldest"] = oldest.Format("2006-01-02")
	}
	return out
}

// handleMemory: GET /api/admin/memory.
func (s *Server) handleMemory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.memoryState())
}

// handleMemorySave: PUT /api/admin/memory {"remember": bool}.
func (s *Server) handleMemorySave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Remember bool `json:"remember"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	v := "false"
	if body.Remember {
		v = "true"
	}
	if err := s.Store.SetSetting(store.KeyRememberRatings, v); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.memoryState())
}

// handleMemoryPurge: POST /api/admin/memory/purge {"what": "ratings" | "covers", "days": 0}. Ratings: all of them, or
// those saved over days days ago. Covers: the stored covers of books that no longer exist.
func (s *Server) handleMemoryPurge(w http.ResponseWriter, r *http.Request) {
	var body struct {
		What string `json:"what"`
		Days int    `json:"days"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	out := map[string]any{}
	switch body.What {
	case "ratings":
		var before time.Time
		if body.Days > 0 {
			before = time.Now().AddDate(0, 0, -body.Days)
		}
		out["forgotten"] = s.Store.ForgetRatings(before)
	case "covers":
		files, bytes, _, _ := s.orphanCovers(true)
		out["deleted"], out["bytes"] = files, bytes
	default:
		writeErr(w, http.StatusBadRequest, `what must be "ratings" or "covers"`)
		return
	}
	out["state"] = s.memoryState()
	writeJSON(w, http.StatusOK, out)
}
