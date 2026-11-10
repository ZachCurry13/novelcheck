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
	if fixed, skipped, ok := s.pushTitles(w, r, pushes); ok {
		writeJSON(w, http.StatusOK, map[string]any{"fixed": fixed, "skipped": skipped})
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
	fixed, skipped, ok := s.pushTitles(w, r, []titlePush{p})
	if !ok {
		return
	}
	if fixed == 0 { // saying "Saved" when nothing was saved is what hid the problem
		writeErr(w, http.StatusConflict, "nothing was saved: "+strings.Join(skipped, "; "))
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
func (s *Server) pushTitles(w http.ResponseWriter, r *http.Request, pushes []titlePush) (fixed int, skipped []string, ok bool) {
	if !s.titleMu.TryLock() { // saves go one at a time; pressing again only piled up more work
		writeErr(w, http.StatusConflict, "a title is already being saved in Calibre; wait for it to finish")
		return 0, nil, false
	}
	defer s.titleMu.Unlock()
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		writeErr(w, http.StatusBadRequest, "set up the calibre Content server connection first (Admin → Delivery & Services → Calibre Library)")
		return 0, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	c := s.calibreClient()
	entries := map[int64][]int{}
	want := map[int]string{}
	known := map[int][]string{} // each number -> the titles NovelCheck knows it by (tidy, and as read from Calibre)
	seen := map[int64]string{}  // book -> the (untidy) title NovelCheck read from its Calibre library
	var all []int
	for _, p := range pushes {
		b, err := s.Store.BookByID(p.Book, nil)
		if err != nil {
			continue
		}
		seen[p.Book] = b.TitleFix
		for _, cid := range s.Store.CalibreIDs(p.Book) {
			entries[p.Book] = append(entries[p.Book], cid)
			want[cid] = b.Title
			known[cid] = []string{b.Title, b.TitleFix}
			all = append(all, cid)
		}
	}
	if len(all) == 0 {
		writeErr(w, http.StatusBadRequest, "those books aren't in Calibre")
		return 0, nil, false
	}
	serverNote := ""
	have, err := c.Titles(ctx, all)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return 0, nil, false
	}
	if len(have) == 0 || len(have)*2 < len(all) { // few or none of the books exist there: wrong library?
		if lib, report := s.matchServerLibrary(ctx, known); lib != "" && lib != c.Library {
			log.Printf("calibre: the Content server's library was %q; switching to %q, where the book numbers match", c.Library, lib)
			s.useServerLibrary(lib)
			c = s.calibreClient()
			if have, err = c.Titles(ctx, all); err != nil {
				writeErr(w, http.StatusBadGateway, err.Error())
				return 0, nil, false
			}
		} else if len(report) > 0 {
			serverNote = " What the Content server serves: " + joinLines(report) + "."
		}
	}
	for _, cid := range all {
		// Compared like NovelCheck matches books: ignoring case, punctuation and numbering.
		if title, found := have[cid]; found && store.NormKey(title, "") != store.NormKey(want[cid], "") {
			writeErr(w, http.StatusConflict, "stopped: calibre's book #"+strconv.Itoa(cid)+" is \""+title+
				"\" but NovelCheck expected \""+want[cid]+"\". Is the Content server using the same library? (If you renamed it in Calibre, run a sync first.) Nothing was changed.")
			return 0, nil, false
		}
	}
	var accept []int64 // rated books whose only change will be ours
	for _, p := range pushes {
		unchanged := !s.Store.ChangedInCalibre(p.Book)
		saved := false
		var gone, other []string
		for _, cid := range entries[p.Book] {
			if t, found := have[cid]; found && t == p.Title && seen[p.Book] != "" && seen[p.Book] != p.Title {
				// The Content server already has the tidy title, yet the library NovelCheck reads still has the old one.
				other = append(other, "#"+strconv.Itoa(cid))
				continue
			}
			if _, found := have[cid]; !found {
				gone = append(gone, "#"+strconv.Itoa(cid)) // not in the library the Content server serves
				continue
			}
			if _, err := c.SetMetadata(ctx, cid, p.Title, p.Series, p.Index); err != nil {
				writeErr(w, http.StatusBadGateway, "tidied "+strconv.Itoa(fixed)+" titles, then: "+err.Error())
				return fixed, skipped, false
			}
			saved = true
		}
		if !saved {
			why := "no Calibre entry known for it"
			if len(other) > 0 {
				why = "the Content server already shows the tidy title for " + strings.Join(other, ", ") + ", but the library NovelCheck reads (" + s.readLibrary() + ") still has “" + seen[p.Book] +
					"”. The Content server and NovelCheck are not looking at the same library (the Content server serves “" + s.Store.Setting(store.KeyCalibreSrvLibrary) + "”): point both at the same one under Admin → Calibre Library"
			}
			if len(gone) > 0 {
				why = "the Content server has no book " + strings.Join(gone, ", ") + " (is it serving the same library NovelCheck reads? If the library changed, read it again first)." + serverNote
			}
			skipped = append(skipped, "“"+p.Title+"”: "+why)
			continue
		}
		if err := s.Store.SavedToCalibre(p.Book, p.Title, p.Series, p.Index); err != nil {
			writeStoreErr(w, err)
			return fixed, skipped, false
		}
		fixed++
		if unchanged {
			accept = append(accept, p.Book)
		}
	}
	log.Printf("calibre: tidied %d titles via Content server (requested by admin)", fixed)
	// Calibre moved the renamed books' files: re-read the library (in the background, so the
	// save answers at once) so downloads keep working.
	s.syncSoon(accept)
	return fixed, skipped, true
}
