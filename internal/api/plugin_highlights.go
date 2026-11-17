package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// What the KOReader plugin sends from a book: its highlights and notes (kept, and shown on the book's page), and
// the end of the book (stars and the finished mark, with the next book of the series to download).

type pluginHighlight struct {
	Text     string `json:"text"`
	Note     string `json:"note"`
	Chapter  string `json:"chapter"`
	Page     string `json:"page"`
	Datetime string `json:"datetime"`
}

type pluginFile struct {
	MD5      string `json:"md5"`
	Filename string `json:"filename"`
	Title    string `json:"title"`
	Author   string `json:"author"`
}

func (f pluginFile) device() store.DeviceBook {
	return store.DeviceBook{MD5: strings.ToLower(strings.TrimSpace(f.MD5)), Filename: filepath.Base(strings.ReplaceAll(f.Filename, "\\", "/")),
		Title: f.Title, Author: f.Author}
}

// handlePluginHighlights: POST /kosync/plugin/highlights {"device", "books": [{md5, filename, title, author,
// highlights: [{text, note, chapter, page, datetime}]}]}. Each book is matched the way the device report matches
// it; highlights sent before are not added twice.
func (s *Server) handlePluginHighlights(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Device string `json:"device"`
		Books  []struct {
			pluginFile
			Highlights []pluginHighlight `json:"highlights"`
		} `json:"books"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&body); err != nil || len(body.Books) > 500 {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "bad request"})
		return
	}
	device := strings.TrimSpace(body.Device)
	matched, added, unmatched := 0, 0, 0
	ok := make([]bool, len(body.Books)) // which books were in the library, in the order sent
	for i, b := range body.Books {
		id, ignored := s.Store.LibraryBookForDevice(u.ID, b.device())
		if id == 0 || ignored {
			unmatched++
			continue
		}
		list := make([]store.Highlight, 0, len(b.Highlights))
		for _, h := range b.Highlights {
			list = append(list, store.Highlight{Text: h.Text, Note: h.Note, Chapter: h.Chapter, Page: h.Page, TakenAt: h.Datetime, Device: device})
		}
		n, err := s.Store.SaveHighlights(u.ID, id, list)
		if err != nil {
			setDetail(w, err)
			kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "could not save"})
			return
		}
		ok[i] = true
		matched++
		added += n
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"books": matched, "added": added, "unmatched": unmatched, "ok": ok})
}

// handleBookHighlights: GET /api/books/{id}/highlights, the person's own highlights of a book.
func (s *Server) handleBookHighlights(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.Store.BookByID(id, u); err != nil { // kids' rules apply
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"highlights": s.Store.Highlights(u.ID, id)})
}

// handlePluginFinish: POST /kosync/plugin/finish {md5, filename, title, author, stars, day}. The end of a book
// on the e-reader: the stars (0 = none given), the finished mark (day "2026-11-12", else today), and the next
// book of its series with a file the plugin can download ("next": null when none).
func (s *Server) handlePluginFinish(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		pluginFile
		Device string  `json:"device"`
		Stars  float64 `json:"stars"`
		Day    string  `json:"day"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "bad request"})
		return
	}
	id, ignored := s.Store.LibraryBookForDevice(u.ID, body.device())
	if id == 0 || ignored {
		kosyncJSON(w, http.StatusNotFound, map[string]string{"message": "that book isn't in your NovelCheck library"})
		return
	}
	book, err := s.Store.BookByID(id, u)
	if err != nil {
		kosyncJSON(w, http.StatusNotFound, map[string]string{"message": "that book isn't in your NovelCheck library"})
		return
	}
	if body.Stars > 0 {
		if err := s.Store.SetRating(u.ID, id, body.Stars); err != nil {
			kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
			return
		}
	}
	day := strings.TrimSpace(body.Day)
	if t, err := time.Parse("2006-01-02", day); err != nil || t.After(time.Now().AddDate(0, 0, 1)) || t.Year() < 1900 {
		day = time.Now().Format("2006-01-02")
	}
	_ = s.Store.SetFinishedOn(u.ID, id, day)
	_, _ = s.Store.SyncReading(u.ID, id, true)
	out := map[string]any{"title": book.Title, "stars": s.Store.RatingOf(u.ID, id), "next": nil}
	if next := s.nextInSeries(u, book, body.Device); next != nil {
		out["next"] = next
	}
	kosyncJSON(w, http.StatusOK, out)
}

type nextBook struct {
	pluginBook
	Have bool `json:"have"` // false: the family has no file of it to download
}

// nextInSeries is the book after this one in its series, when it is in the family's library (and not already on
// this device); with a file to download when there is one.
func (s *Server) nextInSeries(u *store.User, book *store.Book, device string) *nextBook {
	if strings.TrimSpace(book.Series) == "" || book.SeriesIndex <= 0 {
		return nil
	}
	list, err := s.Store.SeriesBooks(book.Series, u)
	if err != nil {
		return nil
	}
	onDevice := map[int64]bool{}
	for _, id := range s.Store.DeviceBookIDs(u.ID, strings.TrimSpace(device)) {
		onDevice[id] = true
	}
	for _, b := range list {
		if b.SeriesIndex <= book.SeriesIndex || b.ID == book.ID {
			continue
		}
		if onDevice[b.ID] {
			return nil // the next one is already on the device
		}
		full, err := s.Store.BookByID(b.ID, u)
		if err != nil {
			continue
		}
		if pb, ok := s.pluginBookFor(full); ok {
			return &nextBook{pluginBook: pb, Have: true}
		}
		return &nextBook{pluginBook: pluginBook{ID: full.ID, Title: full.Title, Author: full.Author, Level: full.SpiceLevel}}
	}
	return nil
}
