package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/delivery"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The KOReader plugin's library search: look a book up by any words, see what
// it is (rating, summary), download it to the e-reader and/or put it on the
// Nightstand. Kids' rules apply exactly as everywhere else.

const pluginHitLimit = 30

type pluginHit struct {
	pluginBook
	Series   string `json:"series,omitempty"`
	OnDevice bool   `json:"on_device"`
	Queue    string `json:"queue,omitempty"` // the status on the Nightstand, when it is there
}

// pluginBookFor describes a library book for the plugin; false when there is
// nothing to download (no file NovelCheck can send).
func (s *Server) pluginBookFor(b *store.Book) (pluginBook, bool) {
	copies, _ := s.Store.BookCopies(b.ID)
	best, ok := delivery.BestFile(copies, false)
	if !ok || !s.insideCalibre(best.Path) {
		return pluginBook{}, false
	}
	format := strings.ToLower(best.Format)
	var size int64
	if st, err := os.Stat(best.Path); err == nil {
		size = st.Size()
	}
	return pluginBook{ID: b.ID, Title: b.Title, Author: b.Author, Format: format, Size: size, Level: b.SpiceLevel,
		URL: "/kosync/plugin/book/" + strconv.FormatInt(b.ID, 10) + "/" + fileName(b.Title, format)}, true
}

// hits turns books into search results marked with what the person has already.
func (s *Server) hits(u *store.User, device string, books []store.Book) []pluginHit {
	onDevice := map[int64]bool{}
	for _, id := range s.Store.DeviceBookIDs(u.ID, device) {
		onDevice[id] = true
	}
	queued := map[int64]string{}
	if items, err := s.Store.ListQueue(u.ID, false); err == nil {
		for _, it := range items {
			queued[it.BookID] = it.Status
		}
	}
	out := []pluginHit{}
	for i := range books {
		pb, ok := s.pluginBookFor(&books[i])
		if !ok {
			continue
		}
		pb.Status = queued[books[i].ID]
		out = append(out, pluginHit{pluginBook: pb, Series: books[i].Series, OnDevice: onDevice[books[i].ID], Queue: queued[books[i].ID]})
	}
	return out
}

// handlePluginSearch: GET /kosync/plugin/search?q=words&device=name.
func (s *Server) handlePluginSearch(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) < 2 {
		kosyncJSON(w, http.StatusOK, map[string]any{"books": []pluginHit{}})
		return
	}
	books, _, err := s.Store.ListBooks(store.BookFilter{Query: q, Limit: pluginHitLimit * 2, Sort: "title"}, u)
	if err != nil {
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "search failed"})
		return
	}
	res := s.hits(u, r.URL.Query().Get("device"), books)
	if len(res) > pluginHitLimit {
		res = res[:pluginHitLimit]
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"books": res})
}

// handlePluginNew: GET /kosync/plugin/new: the newest books in the library.
func (s *Server) handlePluginNew(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	books, _, err := s.Store.ListBooks(store.BookFilter{Limit: pluginHitLimit * 2, Sort: "recent"}, u)
	if err != nil {
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "could not list books"})
		return
	}
	res := s.hits(u, r.URL.Query().Get("device"), books)
	if len(res) > pluginHitLimit {
		res = res[:pluginHitLimit]
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"books": res})
}

// handlePluginInfo: GET /kosync/plugin/info/{id}: what a book is about, for the details screen.
func (s *Server) handlePluginInfo(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	b, err := s.Store.BookByID(id, u)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	summary := strings.TrimSpace(b.Blurb)
	if r := []rune(summary); len(r) > 600 {
		summary = string(r[:600]) + "…"
	}
	out := map[string]any{"id": b.ID, "title": b.Title, "author": b.Author, "series": b.Series, "series_index": b.SeriesIndex,
		"level": b.SpiceLevel, "reason": b.SpiceReason, "verdict": b.SummaryVerdict, "summary": summary}
	if pb, ok := s.pluginBookFor(b); ok {
		out["format"], out["size"], out["url"] = pb.Format, pb.Size, pb.URL
	}
	if items, err := s.Store.ListQueue(u.ID, false); err == nil {
		for _, it := range items {
			if it.BookID == b.ID {
				out["queue"] = it.Status
			}
		}
	}
	kosyncJSON(w, http.StatusOK, out)
}

// handlePluginNightstand: POST /kosync/plugin/nightstand {"book_id": 12}.
func (s *Server) handlePluginNightstand(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		BookID int64 `json:"book_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil || body.BookID <= 0 {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "which book?"})
		return
	}
	if _, err := s.Store.BookByID(body.BookID, u); err != nil { // kids' rules still apply
		kosyncJSON(w, http.StatusNotFound, map[string]string{"message": "no such book"})
		return
	}
	if !s.Store.SettingBool(store.KeyModuleQueue) {
		kosyncJSON(w, http.StatusForbidden, map[string]string{"message": "the Nightstand is turned off"})
		return
	}
	if err := s.Store.Enqueue(u.ID, body.BookID); err != nil {
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "could not add it"})
		return
	}
	kosyncJSON(w, http.StatusOK, map[string]string{"queue": "queued"})
}
