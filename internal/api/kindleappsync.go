package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/content"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// What the Kindle app (kindle-app/) sends beyond the plugin's reports: Amazon store books become a private library
// so they are rated, a "My Kindle" list with their ratings, and the words looked up in Amazon's reader.

// maxNewStoreBooks caps the store books one report adds (each is rated by the AI).
const maxNewStoreBooks = 500

// addStoreBooks puts the report's Amazon store books (md5 "asin:<id>") that aren't in the library yet into the
// person's store library and queues them for rating, so the report then matches them.
func (s *Server) addStoreBooks(u *store.User, books []store.DeviceBook) {
	if fixed := s.Store.RepairJunkAuthors(); len(fixed) > 0 { // books made earlier with Amazon's identifier in the author
		s.Worker.Enqueue(false, fixed...)
	}
	var cat int64
	var queue []int64
	for _, b := range books {
		b.Author = store.StripJunkToken(b.Author)
		asin, ok := strings.CutPrefix(b.MD5, "asin:")
		if !ok || asin == "" || strings.TrimSpace(b.Title) == "" || strings.TrimSpace(b.Author) == "" {
			continue
		}
		if id, _ := s.Store.LibraryBookForDevice(u.ID, b); id != 0 {
			continue
		}
		if cat == 0 {
			var err error
			if cat, err = s.Store.StoreCatalog(u); err != nil {
				log.Printf("kindle store library for %s: %v", u.Username, err)
				return
			}
		}
		id, created, err := s.Store.AddStoreBook(cat, strings.TrimSpace(b.Title), strings.TrimSpace(b.Author), asin)
		if err != nil {
			log.Printf("kindle store book %q: %v", b.Title, err)
			continue
		}
		if created {
			if queue = append(queue, id); len(queue) >= maxNewStoreBooks {
				break
			}
		}
	}
	if len(queue) > 0 {
		s.Worker.Enqueue(false, queue...)
	}
}

// handleKindleMine: GET /kosync/plugin/kindle-mine?device=, the app's "My Kindle" list with each book's rating.
func (s *Server) handleKindleMine(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"books": s.Store.KindleLibrary(u.ID, strings.TrimSpace(r.URL.Query().Get("device")))})
}

// handleKindleWords: POST /kosync/plugin/kindle-words {"books": [{title, author, words: [{word, usage, at}]}]}, the
// words looked up in Amazon's reader, matched to library books like the device report.
func (s *Server) handleKindleWords(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Books []struct {
			Title  string             `json:"title"`
			Author string             `json:"author"`
			Words  []store.KindleWord `json:"words"`
		} `json:"books"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&body); err != nil || len(body.Books) > 2000 {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "bad request"})
		return
	}
	matched, added := 0, 0
	for _, b := range body.Books {
		id, ignored := s.Store.LibraryBookForDevice(u.ID, store.DeviceBook{Title: b.Title, Author: b.Author, Filename: b.Title})
		if id == 0 || ignored {
			continue
		}
		n, err := s.Store.SaveKindleWords(u.ID, id, b.Words)
		if err != nil {
			setDetail(w, err)
			kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "could not save"})
			return
		}
		matched++
		added += n
	}
	kosyncJSON(w, http.StatusOK, map[string]int{"books": matched, "added": added})
}

// moreInfo adds what a book page on an e-reader shows beyond the plugin's basics (the Kindle app's book page): the
// door, the content found grouped as on the web page, this person's heads-up, the rating's yes/no answers, genres,
// Hardcover readers' stars and the person's own.
func (s *Server) moreInfo(u *store.User, b *store.Book, out map[string]any) {
	if b.SpiceLevel != nil {
		out["door"] = "No Spice"
		if *b.SpiceLevel >= 4 {
			out["door"] = "Open Door"
		} else if *b.SpiceLevel >= 2 {
			out["door"] = "Closed Door"
		}
	}
	groups := map[string][]string{}
	var order []string
	for _, kv := range strings.Split(b.Content, ",") {
		k, _, _ := strings.Cut(kv, ":")
		g := content.GroupOf(k)
		if g == "" {
			continue
		}
		if _, ok := groups[g]; !ok {
			order = append(order, g)
		}
		groups[g] = append(groups[g], content.Label(k))
	}
	var found []map[string]any
	for _, g := range order {
		found = append(found, map[string]any{"group": content.Label(g), "items": groups[g]})
	}
	out["content"] = found
	var flags []string
	for _, f := range []struct {
		on    bool
		label string
	}{{b.Nudity, "Nudity"}, {b.SoloActs, "Solo acts"}, {b.HeavyInnuendo, "Heavy innuendo"}, {b.DarkOccult, "Dark occult"}, {b.DemonicPresence, "Demonic presence"}} {
		if f.on {
			flags = append(flags, f.label)
		}
	}
	out["flags"] = flags
	out["heads_up"] = s.Store.HeadsUp(u, b)
	var genres []string
	for _, g := range strings.Split(strings.Trim(b.Genres, ","), ",") {
		if g != "" {
			genres = append(genres, g)
		}
	}
	out["genres"] = genres
	out["hc_rating"], out["hc_ratings"] = b.HCRating, b.HCRatings
	out["my_stars"] = s.Store.RatingOf(u.ID, b.ID)
	out["old_rules"] = store.OldRating(b)
}
