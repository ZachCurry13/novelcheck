package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Search for the plugin and the Kindle app, the same as Check a book: a book the family doesn't have can be looked up, rated
// and asked for. Kids need a parent's OK (the same switch as the AI features), and a book outside their rules never shows.

// mayLookUp: a kid needs the parent's "AI features" switch, because a look-up rates a new book with AI.
func mayLookUp(u *store.User) bool { return u.MayUseAI() }

// handlePluginLookup: GET /kosync/plugin/lookup?q=words: the books that name could mean (Open Library), to pick from.
func (s *Server) handlePluginLookup(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	if !mayLookUp(u) {
		kosyncJSON(w, http.StatusForbidden, map[string]string{"message": "Looking up other books is off for your account. Ask a parent to turn it on."})
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) < 2 {
		kosyncJSON(w, http.StatusOK, map[string]any{"choices": []enrich.Choice{}})
		return
	}
	choices := s.Worker.Enricher().Search(r.Context(), q, maxChoices)
	if choices == nil {
		choices = []enrich.Choice{}
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"choices": choices, "sure": enrich.Sure(q, choices)})
}

// handlePluginCheck: POST /kosync/plugin/check {"title","author","isbn"}: save the picked book under Looked up and start rating
// it. The caller then asks /plugin/info/{id} until its "status" is "analyzed".
func (s *Server) handlePluginCheck(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	if !mayLookUp(u) {
		kosyncJSON(w, http.StatusForbidden, map[string]string{"message": "Looking up other books is off for your account. Ask a parent to turn it on."})
		return
	}
	var f enrich.Found
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&f); err != nil || strings.TrimSpace(f.Title) == "" {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "which book?"})
		return
	}
	f.Title, f.Author, f.ISBN = strings.TrimSpace(f.Title), strings.TrimSpace(f.Author), enrich.CleanISBN(f.ISBN)
	start := time.Now()
	id := s.Store.MatchBook(f.Title, f.Author)
	if s.otherAuthor(id, f.Author) {
		id = 0
	}
	if id == 0 {
		var err error
		if id, err = s.saveLookup(f); err != nil {
			kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "could not save it"})
			return
		}
	}
	b, err := s.Store.BookByID(id, nil) // rated first, whoever asks: whether this person may see it is decided on its page (blockedInfo)
	if err != nil {
		kosyncJSON(w, http.StatusNotFound, map[string]string{"message": "No results."})
		return
	}
	s.rateChecked(b, checkRun{At: start, User: u.Username, What: f.Title, Title: b.Title, Lookup: time.Since(start)})
	kosyncJSON(w, http.StatusOK, map[string]any{"id": id})
}

// requestInfo says what the person may ask for on a book's page: "request" lists the formats still missing, "wished" the ones
// already asked for. Nothing is offered for a book the person can already get in every format.
func (s *Server) requestInfo(u *store.User, b *store.Book, out map[string]any) {
	owned := s.Store.Owned(b.ID)
	out["in_library"] = owned
	var ask, wished []string
	for _, f := range []string{"ebook", "audio"} {
		if s.Store.HasFormat(b.ID, f) {
			continue
		}
		if s.Store.MyWishFormat(b.ID, u.ID, f) != nil || (!owned && s.Store.MyWish(b.ID, u.ID) != nil) {
			wished = append(wished, f)
		} else {
			ask = append(ask, f)
		}
	}
	out["request"], out["wished"] = ask, wished
}

// handlePluginWish: POST /kosync/plugin/wish {"book_id", "format": "ebook"|"audio"}: ask for the book in that format.
func (s *Server) handlePluginWish(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		BookID int64  `json:"book_id"`
		Format string `json:"format"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil || body.BookID <= 0 || (body.Format != "ebook" && body.Format != "audio") {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "which book, and ebook or audio?"})
		return
	}
	if _, err := s.Store.BookByID(body.BookID, u); err != nil {
		kosyncJSON(w, http.StatusNotFound, map[string]string{"message": "no such book"})
		return
	}
	if s.Store.HasFormat(body.BookID, body.Format) {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "the family already has it in that format"})
		return
	}
	if err := s.Store.AddWishFormat(body.BookID, u, "", body.Format); err != nil {
		if errors.Is(err, store.ErrAlreadyWished) {
			kosyncJSON(w, http.StatusConflict, map[string]string{"message": "you already asked for it"})
			return
		}
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "could not save the request"})
		return
	}
	s.Store.Notify("info", "wishlist", "New books are on the family wishlist.", "#/wishlist")
	kosyncJSON(w, http.StatusOK, map[string]string{"wish": body.Format})
}

// blockedInfo answers /plugin/info for a book this person's rules hide. While the book is still being rated it says "processing"
// (the rules may need the rating); once rated and still hidden it says so plainly, shows nothing about the book (no rating, so a
// kid is not pointed at it), and tells the parents once, who can allow it by marking the book OK.
func (s *Server) blockedInfo(u *store.User, id int64) (map[string]any, bool) {
	b, err := s.Store.BookByID(id, nil)
	if err != nil {
		return nil, false
	}
	if b.Status != "analyzed" {
		return map[string]any{"id": id, "status": b.Status, "pending": true}, true
	}
	if _, told := blockedTold.LoadOrStore(fmt.Sprintf("%d|%d", u.ID, id), true); !told && u.Role == store.RoleRestricted {
		s.Store.Notify("info", "lookups", fmt.Sprintf("%s looked up “%s”, which is outside their rules. It stays hidden from them unless you mark it OK.", u.Username, b.Title), "#/library")
	}
	return map[string]any{"id": id, "status": "analyzed", "blocked": true, "message": "That book isn't available on your account. A parent has been told."}, true
}

var blockedTold sync.Map // "person|book" already reported to the parents
