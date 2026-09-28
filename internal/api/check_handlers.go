package api

import (
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/auth"
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
	look, ok := s.lookUp(w, r, body.Query, body.Image)
	if !ok {
		return
	}
	id := look.id
	if id == 0 {
		var err error
		if id, err = s.saveLookup(look.best()); err != nil {
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
		s.rateSoon(b)
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
		"in_library": len(catalogs) > 0, "catalogs": catalogs, "found": look.found, "source": look.source,
		"my_wish": s.Store.MyWish(id, auth.UserFrom(r).ID)})
}

// lookup is the book a photo or a typed title, author or ISBN points to.
type lookup struct {
	found     enrich.Found // what was read or typed, completed by Open Library
	canonical enrich.Found // for a photo: Open Library's own title and author
	source    string       // "photo" or "search"
	id        int64        // the book NovelCheck already has, or 0
}

// best is the title and author to save for a book NovelCheck doesn't have yet.
func (l lookup) best() enrich.Found {
	if l.canonical.Title != "" {
		return l.canonical
	}
	return l.found
}

// lookUp finds the book a cover photo or a typed query means (Check a book
// and paper books). On failure it has already written the reply.
func (s *Server) lookUp(w http.ResponseWriter, r *http.Request, query, image string) (lookup, bool) {
	ctx := r.Context()
	ec := s.Worker.Enricher()
	var l lookup
	l.source = "search"
	switch query = strings.TrimSpace(query); {
	case image != "":
		img, mediaType, ok := decodePhoto(image)
		if !ok {
			writeErr(w, http.StatusBadRequest, "that photo couldn't be opened; please take it again")
			return l, false
		}
		f, err := s.Worker.ReadCover(ctx, img, mediaType)
		if err != nil {
			log.Printf("check a book: %v", err)
			msg := "Your AI couldn't read the cover. Type the title instead."
			if !errors.Is(err, analyzer.ErrCantSee) {
				msg = "Reading the photo failed: " + err.Error()
			}
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": msg, "detail": err.Error()})
			return l, false
		}
		l.found, l.source = f, "photo"
		l.canonical, _ = ec.FindTitle(ctx, f.Title, f.Author)
	case query != "":
		f, ok := ec.Find(ctx, query)
		switch {
		case ok:
			l.found = f
		case enrich.CleanISBN(query) != "":
			writeErr(w, http.StatusNotFound, "no book found with that ISBN; try typing the title")
			return l, false
		default:
			l.found = enrich.Found{Title: query} // not in Open Library: rate what was typed
		}
	default:
		writeErr(w, http.StatusBadRequest, "type a title or take a photo of the cover")
		return l, false
	}
	l.id = s.Store.MatchBook(l.found.Title, l.found.Author)
	if l.id == 0 && l.canonical.Title != "" {
		l.id = s.Store.MatchBook(l.canonical.Title, l.canonical.Author)
	}
	return l, true
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
