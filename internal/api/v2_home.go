package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Home (the start screen): for everyone, what is next for them (their Nightstand, new books, their asks, their e-reader's
// books NovelCheck couldn't match); for a parent, the asks waiting for an answer; for an admin, the inbox of things that
// need a decision (each linked to the tool that decides it, and set aside with "Not now") and how healthy NovelCheck is.

type homeCard struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Count  int    `json:"count"`
	Link   string `json:"link,omitempty"`
}

type homeBook struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Level  *int   `json:"level"`
	Status string `json:"status,omitempty"`
}

// one says the singular for 1 and the plural otherwise ("1 book has no cover", "3 books have no cover").
func one(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	out := map[string]any{"name": u.Username, "role": u.Role, "may_upload": u.MayUpload()}
	next := []homeBook{}
	if items, err := s.Store.ListQueue(u.ID, false); err == nil {
		for _, it := range items {
			if len(next) < 4 && it.Status != "finished" {
				next = append(next, homeBook{ID: it.BookID, Title: it.Title, Author: it.Author, Level: it.SpiceLevel, Status: it.Status})
			}
		}
	}
	out["next_up"] = next
	fresh := []homeBook{}
	if books, _, err := s.Store.ListBooks(store.BookFilter{Sort: "recent", Limit: 8}, u); err == nil {
		for _, b := range books {
			fresh = append(fresh, homeBook{ID: b.ID, Title: b.Title, Author: b.Author, Level: b.SpiceLevel})
		}
	}
	out["new_books"] = fresh
	mine := s.Store.RequestsFor(u.ID)
	out["my_requests"] = mine[:min(len(mine), 5)]
	out["unmatched"] = len(s.Store.KindleAttention(u.ID))
	if !u.CanManage() {
		writeJSON(w, http.StatusOK, out)
		return
	}
	out["requests"] = s.openRequestsFor(u)
	snoozed := s.Store.Snoozed(u.ID)
	if u.IsAdmin() && !snoozed["setup"] { // a new NovelCheck: what to do first, until it is all done
		if steps := s.setupSteps(); !setupDone(steps) {
			out["setup"] = steps
		}
	}
	inbox := []homeCard{}
	add := func(id, title, detail string, n int, link string) {
		if n > 0 && !snoozed[id] {
			inbox = append(inbox, homeCard{id, title, detail, n, link})
		}
	}
	add("review", one(s.Store.CountReviewRatings(), "rating to look at", "ratings to look at"), "The AI wasn't sure about these. A person decides the level.", s.Store.CountReviewRatings(), "#/library?spice=review")
	add("disagree", one(s.Store.UnresolvedFeedback(), "rating a person disagrees with", "ratings people disagree with"), "Someone gave a different level than the AI. Use theirs, or leave it.", s.Store.UnresolvedFeedback(), "#/accuracy")
	if u.IsAdmin() {
		add("covers", one(len(s.missingForPage()), "book has no cover", "books have no cover"), "The cover finder keeps trying; you can pick one by hand.", len(s.missingForPage()), "#/admin?tab=cleanup")
		if u.Can(store.AreaCalibre) {
			if c, err := s.cleanupSummary(); err == nil {
				num := func(k string) int { n, _ := c[k].(int); return n }
				add("pairs", one(num("pairs"), "book listed twice in NovelCheck", "books listed twice in NovelCheck"), "The same book under two names. Join them or say they differ.", num("pairs"), "#/duplicates")
				add("dups", one(num("calibre_dups"), "book is in Calibre more than once", "books are in Calibre more than once"), "Pick the copy to keep; the others go to Calibre's recycle bin.", num("calibre_dups"), "#/duplicates")
				add("titles", one(num("titles"), "title can be tidied in Calibre", "titles can be tidied in Calibre"), "NovelCheck found cleaner titles.", num("titles"), "#/admin?tab=cleanup")
				add("convert", one(num("convert"), "book can be converted to EPUB", "books can be converted to EPUB"), "They have no EPUB but a file Calibre can convert.", num("convert"), "#/admin?tab=cleanup")
			}
		}
		items := s.healthItems()
		var worst []healthItem
		for _, it := range items {
			if it.Level != "ok" && len(worst) < 4 {
				worst = append(worst, it)
			}
		}
		out["health"] = map[string]any{"level": healthLevel(items), "items": worst}
	}
	out["inbox"] = inbox
	writeJSON(w, http.StatusOK, out)
}

// handleSnooze: POST /api/home/snooze {"id", "days"}: "Not now" for an inbox item (a week by default).
func (s *Server) handleSnooze(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID   string `json:"id"`
		Days int    `json:"days"`
	}
	if !readJSON(w, r, &body, 1<<10) || body.ID == "" || len(body.ID) > 60 {
		return
	}
	if body.Days <= 0 || body.Days > 60 {
		body.Days = 7
	}
	if err := s.Store.Snooze(auth.UserFrom(r).ID, body.ID, body.Days); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
