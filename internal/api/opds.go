package api

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/delivery"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// KOReader (and other e-reader apps) read an OPDS 1.2 catalog. Each person's
// private address /opds/<token> lists their Up Next with download links; the
// token stands in for signing in, which KOReader can't do.

type opdsLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr"`
}

type opdsEntry struct {
	Title   string     `xml:"title"`
	ID      string     `xml:"id"`
	Updated string     `xml:"updated"`
	Author  string     `xml:"author>name"`
	Summary string     `xml:"summary"`
	Links   []opdsLink `xml:"link"`
}

type opdsFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	ID      string      `xml:"id"`
	Title   string      `xml:"title"`
	Updated string      `xml:"updated"`
	Author  string      `xml:"author>name"`
	Links   []opdsLink  `xml:"link"`
	Entries []opdsEntry `xml:"entry"`
}

const opdsType = "application/atom+xml;profile=opds-catalog;kind=acquisition"

var bookTypes = map[string]string{"epub": "application/epub+zip", "pdf": "application/pdf",
	"mobi": "application/x-mobipocket-ebook", "azw3": "application/vnd.amazon.ebook", "azw": "application/vnd.amazon.ebook"}

// opdsUser checks the token and that KOReader delivery is switched on.
func (s *Server) opdsUser(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	u, err := s.Store.UserByOPDSToken(chi.URLParam(r, "token"))
	if err != nil || !s.Store.SettingBool(store.KeyModuleKOReader) {
		http.NotFound(w, r)
		return nil, false
	}
	return u, true
}

// baseURL is the address the e-reader used to reach us (http or https).
func (s *Server) baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || (s.Cfg.TrustProxy && r.Header.Get("X-Forwarded-Proto") == "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func (s *Server) handleOPDSFeed(w http.ResponseWriter, r *http.Request) {
	u, ok := s.opdsUser(w, r)
	if !ok {
		return
	}
	self := s.baseURL(r) + "/opds/" + chi.URLParam(r, "token")
	now := time.Now().UTC().Format(time.RFC3339)
	feed := opdsFeed{ID: "urn:novelcheck:up-next:" + u.Username, Title: "NovelCheck: Up Next for " + u.Username,
		Updated: now, Author: "NovelCheck",
		Links: []opdsLink{{"self", self, opdsType}, {"start", self, opdsType}}}
	items, _ := s.Store.ListQueue(u.ID, false)
	for _, it := range items {
		b, err := s.Store.BookByID(it.BookID, u) // kids' rules still apply
		if err != nil {
			continue
		}
		copies, _ := s.Store.BookCopies(b.ID)
		best, ok := delivery.BestFile(copies, false)
		if !ok || !s.insideCalibre(best.Path) {
			continue
		}
		format := strings.ToLower(best.Format)
		mime := bookTypes[format]
		if mime == "" {
			mime = "application/octet-stream"
		}
		summary := b.SummaryVerdict
		if b.SpiceLevel != nil {
			summary = fmt.Sprintf("Level %d. %s", *b.SpiceLevel, summary)
		}
		href := fmt.Sprintf("%s/book/%d/%s", self, b.ID, fileName(b.Title, format))
		feed.Entries = append(feed.Entries, opdsEntry{Title: b.Title, ID: fmt.Sprintf("urn:novelcheck:book:%d", b.ID),
			Updated: now, Author: b.Author, Summary: summary,
			Links: []opdsLink{{"http://opds-spec.org/acquisition", href, mime}}})
	}
	w.Header().Set("Content-Type", opdsType+";charset=utf-8")
	_, _ = w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	_ = enc.Encode(feed)
}

// handleOPDSBook sends the book file to the e-reader.
func (s *Server) handleOPDSBook(w http.ResponseWriter, r *http.Request) {
	u, ok := s.opdsUser(w, r)
	if !ok {
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
	copies, _ := s.Store.BookCopies(id)
	best, ok := delivery.BestFile(copies, false)
	if !ok || !s.insideCalibre(best.Path) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(best.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	format := strings.ToLower(best.Format)
	if mime := bookTypes[format]; mime != "" {
		w.Header().Set("Content-Type", mime)
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+fileName(b.Title, format)+`"`)
	http.ServeContent(w, r, filepath.Base(best.Path), st.ModTime(), f)
}

// fileName makes a safe download name like "Pride-and-Prejudice.epub".
func fileName(title, format string) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == ' ' || r == '-' || r == '_':
			return '-'
		}
		return -1
	}, title)
	if name = strings.Trim(name, "-"); name == "" {
		name = "book"
	}
	return name + "." + format
}
