package api

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// maxPhoto caps an uploaded cover photo (the app shrinks it to ~1024 px first).
const maxPhoto = 5 << 20

// handleCheck is Check a book: find the book from a photo or what was typed,
// show its rating if NovelCheck knows it, or start rating it right away. The
// app then polls GET /api/books/{id} until the rating is ready, which keeps
// each request short (Cloudflare Tunnel gives up after 100 seconds).
func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query string `json:"query"` // title, author or ISBN as typed
		Image string `json:"image"` // data:image/jpeg;base64,… of the cover
	}
	if !readJSON(w, r, &body, maxPhoto*4/3+4096) {
		return
	}
	ctx := r.Context()
	ec := s.Worker.Enricher()
	var found, canonical enrich.Found
	source := "search"
	switch query := strings.TrimSpace(body.Query); {
	case body.Image != "":
		img, mediaType, ok := decodePhoto(body.Image)
		if !ok {
			writeErr(w, http.StatusBadRequest, "that photo couldn't be opened; please take it again")
			return
		}
		f, err := s.Worker.ReadCover(ctx, img, mediaType)
		if err != nil {
			log.Printf("check a book: %v", err)
			msg := "Your AI couldn't read the cover. Type the title instead."
			if !errors.Is(err, analyzer.ErrCantSee) {
				msg = "Reading the photo failed: " + err.Error()
			}
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": msg, "detail": err.Error()})
			return
		}
		found, source = f, "photo"
		canonical, _ = ec.FindTitle(ctx, f.Title, f.Author)
	case query != "":
		f, ok := ec.Find(ctx, query)
		switch {
		case ok:
			found = f
		case enrich.CleanISBN(query) != "":
			writeErr(w, http.StatusNotFound, "no book found with that ISBN; try typing the title")
			return
		default:
			found = enrich.Found{Title: query} // not in Open Library: rate what was typed
		}
	default:
		writeErr(w, http.StatusBadRequest, "type a title or take a photo of the cover")
		return
	}

	id := s.Store.MatchBook(found.Title, found.Author)
	if id == 0 && canonical.Title != "" {
		id = s.Store.MatchBook(canonical.Title, canonical.Author)
	}
	if id == 0 {
		t := found
		if canonical.Title != "" {
			t = canonical
		}
		var err error
		if id, err = s.saveLookup(t); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	b, err := s.Store.BookByID(id, nil)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if b.Status != "analyzed" && b.Status != "processing" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()
			if err := s.Worker.RateNow(ctx, id); err != nil {
				log.Printf("check a book: rating %q failed: %v", b.Title, err)
			}
		}()
		b.Status = "processing"
	}
	var catalogs []string
	copies, _ := s.Store.BookCopies(id)
	for _, c := range copies {
		if c.CatalogName != store.LookedUpCatalog && !contains(catalogs, c.CatalogName) {
			catalogs = append(catalogs, c.CatalogName)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"book": b, "rating": b.Status == "processing",
		"in_library": len(catalogs) > 0, "catalogs": catalogs, "found": found, "source": source})
}

// saveLookup adds a checked book to the "Looked up" list.
func (s *Server) saveLookup(f enrich.Found) (int64, error) {
	id, err := s.Store.UpsertBook(f.Title, f.Author, f.ISBN, "")
	if err != nil {
		return 0, err
	}
	cat, err := s.Store.EnsureCatalog(store.LookedUpCatalog, "custom")
	if err != nil {
		return 0, err
	}
	return id, s.Store.AddCopy(cat, id, "lookup:"+f.Title, "list", "")
}

// decodePhoto reads a data: URL of a JPEG, PNG or WebP photo.
func decodePhoto(dataURL string) ([]byte, string, bool) {
	head, data, ok := strings.Cut(dataURL, ",")
	mediaType := strings.TrimSuffix(strings.TrimPrefix(head, "data:"), ";base64")
	if !ok || !strings.HasSuffix(head, ";base64") ||
		(mediaType != "image/jpeg" && mediaType != "image/png" && mediaType != "image/webp") {
		return nil, "", false
	}
	img, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(img) == 0 || len(img) > maxPhoto {
		return nil, "", false
	}
	return img, mediaType, true
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
