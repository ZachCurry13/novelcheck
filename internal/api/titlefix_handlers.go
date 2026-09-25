package api

// Tidying titles in Calibre. NovelCheck already shows "01 - Dune" as "Dune"
// (book 1); these let an admin make Calibre match, through its Content
// server. NovelCheck's own access to the library stays read-only.

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

type titlePush struct {
	Book   int64
	Title  string
	Series string
	Index  float64
}

// handleTitleFixes lists the titles that could be tidied in Calibre.
func (s *Server) handleTitleFixes(w http.ResponseWriter, r *http.Request) {
	fixes, err := s.Store.TitleFixes()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"fixes": fixes,
		"server": s.Store.Setting(store.KeyCalibreSrvURL) != "", "calibre_web_url": s.Store.Setting(store.KeyCalibreWebURL)})
}

// handleFixTitles tidies the chosen titles in Calibre, as NovelCheck shows them.
func (s *Server) handleFixTitles(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if !readJSON(w, r, &body, 64<<10) {
		return
	}
	fixes, err := s.Store.TitleFixes()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	chosen := map[int64]bool{}
	for _, id := range body.IDs {
		chosen[id] = true
	}
	var pushes []titlePush
	for _, f := range fixes {
		if chosen[f.ID] {
			pushes = append(pushes, titlePush{f.ID, f.Title, f.Series, f.SeriesIndex})
		}
	}
	if len(pushes) == 0 {
		writeErr(w, http.StatusBadRequest, "choose at least one title")
		return
	}
	if fixed, ok := s.pushTitles(w, r, pushes); ok {
		writeJSON(w, http.StatusOK, map[string]int{"fixed": fixed})
	}
}

// handleSaveCalibreTitle saves one book's title and series in Calibre, as
// the admin typed them.
func (s *Server) handleSaveCalibreTitle(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Title       string  `json:"title"`
		Series      string  `json:"series"`
		SeriesIndex float64 `json:"series_index"`
	}
	if !readJSON(w, r, &body, 8<<10) {
		return
	}
	p := titlePush{id, strings.TrimSpace(body.Title), strings.TrimSpace(body.Series), body.SeriesIndex}
	switch {
	case p.Title == "" || len(p.Title) > 300:
		writeErr(w, http.StatusBadRequest, "type a title (up to 300 characters)")
		return
	case len(p.Series) > 200 || p.Index < 0 || p.Index > 9999:
		writeErr(w, http.StatusBadRequest, "the series name or number doesn't look right")
		return
	}
	if _, ok := s.pushTitles(w, r, []titlePush{p}); !ok {
		return
	}
	b, err := s.Store.BookByID(id, nil)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"book": b})
}

// pushTitles checks every Calibre entry against the Content server first (so
// the wrong library is never changed), saves the titles, then re-reads the
// library. On failure it writes the error and returns ok=false.
func (s *Server) pushTitles(w http.ResponseWriter, r *http.Request, pushes []titlePush) (fixed int, ok bool) {
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		writeErr(w, http.StatusBadRequest, "set up the calibre Content server connection first (Admin → Delivery & Services → Calibre Library)")
		return 0, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	c := s.calibreClient()
	entries := map[int64][]int{}
	want := map[int]string{}
	var all []int
	for _, p := range pushes {
		b, err := s.Store.BookByID(p.Book, nil)
		if err != nil {
			continue
		}
		for _, cid := range s.Store.CalibreIDs(p.Book) {
			entries[p.Book] = append(entries[p.Book], cid)
			want[cid] = b.Title
			all = append(all, cid)
		}
	}
	if len(all) == 0 {
		writeErr(w, http.StatusBadRequest, "those books aren't in Calibre")
		return 0, false
	}
	have, err := c.Titles(ctx, all)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return 0, false
	}
	for _, cid := range all {
		// Compared like NovelCheck matches books: ignoring case, punctuation and numbering.
		if title, found := have[cid]; found && store.NormKey(title, "") != store.NormKey(want[cid], "") {
			writeErr(w, http.StatusConflict, "stopped: calibre's book #"+strconv.Itoa(cid)+" is \""+title+
				"\" but NovelCheck expected \""+want[cid]+"\". Is the Content server using the same library? (If you renamed it in Calibre, run a sync first.) Nothing was changed.")
			return 0, false
		}
	}
	var accept []int64 // rated books whose only change will be ours
	for _, p := range pushes {
		unchanged := !s.Store.ChangedInCalibre(p.Book)
		saved := false
		for _, cid := range entries[p.Book] {
			if _, found := have[cid]; !found {
				continue // already gone from calibre
			}
			if _, err := c.SetMetadata(ctx, cid, p.Title, p.Series, p.Index); err != nil {
				writeErr(w, http.StatusBadGateway, "tidied "+strconv.Itoa(fixed)+" titles, then: "+err.Error())
				return fixed, false
			}
			saved = true
		}
		if !saved {
			continue
		}
		if err := s.Store.SavedToCalibre(p.Book, p.Title, p.Series, p.Index); err != nil {
			writeStoreErr(w, err)
			return fixed, false
		}
		fixed++
		if unchanged {
			accept = append(accept, p.Book)
		}
	}
	log.Printf("calibre: tidied %d titles via Content server (requested by admin)", fixed)
	// Calibre moved the renamed books' files: re-read the library so downloads keep working.
	if s.Syncer.Available() {
		if _, err := s.Syncer.Run(); err != nil {
			log.Printf("calibre sync after tidying titles failed: %v", err)
		} else if err := s.Store.AcceptModified(accept); err != nil {
			log.Printf("keeping ratings after tidying titles: %v", err)
		}
	}
	return fixed, true
}
