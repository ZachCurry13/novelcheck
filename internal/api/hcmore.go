package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The book page's "From Hardcover" section (js/hcextras.js): the audiobook (length, narrators) and "More like
// this", the books Hardcover's readers find similar, marked when the family has them. Asked for when a book
// page opens, with a family member's Hardcover connection, and kept for a day.

type similarBook struct {
	Title  string  `json:"title"`
	Author string  `json:"author"`
	Cover  string  `json:"cover"`
	Rating float64 `json:"hc_rating"`
	BookID int64   `json:"book_id,omitempty"` // in a family library: open it
	Year   int     `json:"year,omitempty"`
}

type hcMore struct {
	hardcover.More
	Similar []similarBook `json:"similar"`
	at      time.Time
}

var hcMoreBusy = map[int]bool{} // answers being fetched again in the background (guarded by hcMoreCache)

var hcMoreCache = struct {
	sync.Mutex
	m map[int]hcMore
}{m: map[int]hcMore{}}

// handleHCMore: GET /api/books/{id}/hardcover. Empty when nobody connected Hardcover or it doesn't know the book.
func (s *Server) handleHCMore(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	b, err := s.Store.BookByID(id, u)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	// A person is looking at a page: when Hardcover is resting or busy with the background work (it shares one pace per token), the
	// page opens without this part after a few seconds instead of waiting a minute or more behind it.
	hctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	out, _ := s.hardcoverMore(hctx, b)
	// Kids see only the similar books they could open in the family's libraries.
	list := []similarBook{}
	for _, x := range out.Similar {
		if x.BookID != 0 {
			if _, err := s.Store.BookByID(x.BookID, u); err != nil {
				x.BookID = 0
			}
		}
		if x.BookID == 0 && u.Role == store.RoleRestricted {
			continue
		}
		list = append(list, x)
	}
	out.Similar = list
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) hardcoverMore(ctx context.Context, b *store.Book) (hcMore, error) {
	token := s.Store.RatingsToken()
	if token == "" {
		return hcMore{}, nil
	}
	hc := &hardcover.Client{Token: token, URL: hardcoverURL}
	hcID := b.HCID
	if hcID == 0 {
		info, ok, err := hc.InfoByTitle(ctx, b.Title, b.Author)
		if err != nil || !ok {
			return hcMore{}, err
		}
		hcID = info.ID
		s.Store.SaveHCID(b.ID, hcID)
	}
	hcMoreCache.Lock()
	c, ok := hcMoreCache.m[hcID]
	hcMoreCache.Unlock()
	if ok && time.Since(c.at) < 24*time.Hour {
		return c, nil
	}
	// kept in the database too, so a restart or a week later is not a wait: an answer under a week old is used
	// as it is, one under three months old is shown at once and fetched again behind it
	if data, at, found := s.Store.HCMoreGet(hcID); found {
		var kept hcMore
		if json.Unmarshal([]byte(data), &kept) == nil {
			kept.at = at
			age := time.Since(at)
			if age < 90*24*time.Hour {
				if age >= 7*24*time.Hour {
					s.refreshHCMore(hc, b, hcID)
				}
				hcMoreCache.Lock()
				hcMoreCache.m[hcID] = kept
				hcMoreCache.Unlock()
				return kept, nil
			}
		}
	}
	return s.fetchHCMore(ctx, hc, b, hcID)
}

// refreshHCMore fetches a kept answer again in the background (once at a time per book).
func (s *Server) refreshHCMore(hc *hardcover.Client, b *store.Book, hcID int) {
	hcMoreCache.Lock()
	if hcMoreBusy[hcID] {
		hcMoreCache.Unlock()
		return
	}
	hcMoreBusy[hcID] = true
	hcMoreCache.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		_, _ = s.fetchHCMore(ctx, hc, b, hcID)
		hcMoreCache.Lock()
		delete(hcMoreBusy, hcID)
		hcMoreCache.Unlock()
	}()
}

func (s *Server) fetchHCMore(ctx context.Context, hc *hardcover.Client, b *store.Book, hcID int) (hcMore, error) {
	m, err := hc.MoreAbout(ctx, hcID, 12)
	if err != nil {
		return hcMore{}, err
	}
	out := hcMore{More: m, Similar: []similarBook{}, at: time.Now()}
	for _, x := range m.Similar {
		sb := similarBook{Title: x.Title, Cover: x.Image, Rating: x.Rating, Year: x.Year}
		if len(x.Authors) > 0 {
			sb.Author = x.Authors[0]
			sb.BookID = s.Store.LibraryBookByKey(store.NormKey(x.Title, x.Authors[0]))
		}
		if sb.BookID == b.ID {
			continue
		}
		switch { // covers come through NovelCheck (the page may only load its own images)
		case sb.BookID != 0:
			sb.Cover = fmt.Sprintf("/api/books/%d/cover", sb.BookID)
		case strings.HasPrefix(sb.Cover, "https://"):
			sb.Cover = "/api/libby/cover?u=" + url.QueryEscape(sb.Cover)
		default:
			sb.Cover = ""
		}
		out.Similar = append(out.Similar, sb)
	}
	if raw, err := json.Marshal(out); err == nil {
		s.Store.HCMoreSave(hcID, string(raw))
	}
	hcMoreCache.Lock()
	hcMoreCache.m[hcID] = out
	hcMoreCache.Unlock()
	return out, nil
}
