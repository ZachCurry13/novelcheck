package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The Kindle app (kindle-app/, for a jailbroken Kindle without KOReader) uses the plugin's endpoints, plus these:
// a book as AZW3 or MOBI, which Amazon's own reader opens (Calibre converts an EPUB first), and covers.

// kindleFormats are what Amazon's reader opens from a file, best first (never KFX: a bad one crashes it).
var kindleFormats = []string{"azw3", "mobi", "azw"}

// kindleFile is a copy of the book Amazon's reader opens, on this server's disk: a Calibre copy in one of
// kindleFormats, or the file Calibre wrote beside the EPUB when it converted it (NovelCheck sees it at the next sync).
func (s *Server) kindleFile(copies []store.BookCopy) (path, format string) {
	for _, f := range kindleFormats {
		for _, c := range copies {
			if c.Source != "calibre" || c.Path == "" || !s.insideCalibre(c.Path) {
				continue
			}
			p := c.Path
			if !strings.EqualFold(c.Format, f) {
				p = strings.TrimSuffix(p, filepath.Ext(p)) + "." + f // Calibre names every format of a book alike
			}
			if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
				return p, f
			}
		}
	}
	return "", ""
}

// kindleJobs are the conversions to AZW3 running or failed, by NovelCheck book id ("" while running).
var (
	kindleJobsMu sync.Mutex
	kindleJobs   = map[int64]string{}
)

// handlePluginKindleBook: GET /kosync/plugin/kindle/{id}. The book as AZW3/MOBI, or 202 while Calibre makes one.
func (s *Server) handlePluginKindleBook(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	id, ok := pathID(r, "id")
	b, err := s.Store.BookByID(id, u) // kids' rules apply
	if !ok || err != nil {
		kosyncJSON(w, http.StatusNotFound, map[string]string{"message": "No such book."})
		return
	}
	copies, _ := s.Store.BookCopies(id)
	if p, f := s.kindleFile(copies); p != "" {
		if r.URL.Query().Get("check") != "" { // the app asks first, then has the Kindle download it
			kosyncJSON(w, http.StatusOK, map[string]string{"file": fileName(b.Title, f)})
			return
		}
		file, err := os.Open(p)
		if err != nil {
			kosyncJSON(w, http.StatusNotFound, map[string]string{"message": "The file can't be read."})
			return
		}
		defer file.Close()
		st, _ := file.Stat()
		w.Header().Set("Content-Type", bookTypes[f])
		w.Header().Set("Content-Disposition", `attachment; filename="`+fileName(b.Title, f)+`"`)
		http.ServeContent(w, r, filepath.Base(p), st.ModTime(), file)
		return
	}
	kindleJobsMu.Lock()
	why, known := kindleJobs[id]
	kindleJobsMu.Unlock()
	switch {
	case known && why == "":
		kosyncJSON(w, http.StatusAccepted, map[string]string{"message": "Getting it ready for Kindle…"})
	case known && r.URL.Query().Get("again") == "":
		kosyncJSON(w, http.StatusConflict, map[string]string{"message": "Calibre couldn't convert it: " + why})
	default:
		src, calibreID := s.convertSource(copies)
		if src == "" || s.Store.Setting(store.KeyCalibreSrvURL) == "" {
			kosyncJSON(w, http.StatusConflict, map[string]string{"message": "There's no Kindle copy of this book, and NovelCheck can't make one."})
			return
		}
		kindleJobsMu.Lock()
		kindleJobs[id] = ""
		kindleJobsMu.Unlock()
		safe.Go("convert for the Kindle app", func() { s.convertForKindle(id, calibreID, src) })
		kosyncJSON(w, http.StatusAccepted, map[string]string{"message": "Getting it ready for Kindle…"})
	}
}

// convertSource is the Calibre format to convert from (EPUB first) and its Calibre id.
func (s *Server) convertSource(copies []store.BookCopy) (format string, calibreID int) {
	for _, want := range []string{"epub", ""} {
		for _, c := range copies {
			id, err := strconv.Atoi(c.ExternalID)
			if c.Source != "calibre" || err != nil || c.Format == "" || (want != "" && !strings.EqualFold(c.Format, want)) {
				continue
			}
			if want == "" && !store.Convertible[strings.ToLower(c.Format)] {
				continue
			}
			return strings.ToLower(c.Format), id
		}
	}
	return "", 0
}

// convertForKindle has Calibre add an AZW3 to the book; a failure is kept so the app can say why.
func (s *Server) convertForKindle(id int64, calibreID int, from string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	why := "it took longer than 20 minutes"
	c := s.calibreClient()
	if job, err := c.StartConversionTo(ctx, calibreID, from, "AZW3", nil); err != nil {
		why = err.Error()
	} else {
		for ctx.Err() == nil {
			st, err := c.ConversionStatus(ctx, job)
			if err != nil {
				why = err.Error()
				break
			}
			if !st.Running {
				why = ""
				if !st.OK {
					why = lastLine(st.Trace, st.Log, st.Msg)
				}
				break
			}
			time.Sleep(2 * time.Second)
		}
	}
	kindleJobsMu.Lock()
	defer kindleJobsMu.Unlock()
	if why == "" {
		delete(kindleJobs, id) // the AZW3 is beside the EPUB now
	} else {
		kindleJobs[id] = why
	}
}

// handlePluginCover: GET /kosync/plugin/cover/{id}, the web app's cover for the plugin's sign-in.
func (s *Server) handlePluginCover(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	s.handleCover(w, r.WithContext(auth.WithUser(r.Context(), u)))
}

// kosyncCORS lets the Kindle app (a page opened from the Kindle's own files) call the plugin's endpoints.
// Safe for any origin: these take no cookies, only the sync key in headers, which another page doesn't have.
func kosyncCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", "*")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT")
			h.Set("Access-Control-Allow-Headers", "Content-Type, Accept, x-auth-user, x-auth-key")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
