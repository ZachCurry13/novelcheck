package api

// Doing one thing to many books picked in the Library.

import (
	"errors"
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleBulkBooks adds the picked books to Up Next, Deep Scans them (admins
// start the scans, others request them), or deletes them: books in the
// person's own libraries are taken out straight away, anything else becomes
// a delete request for the admins. It answers with a count per outcome.
func (s *Server) handleBulkBooks(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"` // queue, deep or delete
		Reason string  `json:"reason"`
	}
	if !readJSON(w, r, &body, 64<<10) {
		return
	}
	if len(body.IDs) == 0 || len(body.IDs) > 500 {
		writeErr(w, http.StatusBadRequest, "pick between 1 and 500 books")
		return
	}
	u := auth.UserFrom(r)
	var act func(b *store.Book) string
	switch body.Action {
	case "queue":
		if !s.Store.SettingBool(store.KeyModuleQueue) {
			writeErr(w, http.StatusForbidden, "The reading queue is turned off (Admin → System & Toggles → Features)")
			return
		}
		act = func(b *store.Book) string { return outcome(s.Store.Enqueue(u.ID, b.ID), "queued") }
	case "deep":
		act = func(b *store.Book) string { return s.bulkDeep(b, u, body.Reason) }
	case "delete":
		owners := map[int64]*int64{} // library id -> owner, looked up once
		act = func(b *store.Book) string { return s.bulkDelete(b, u, body.Reason, owners) }
	default:
		writeErr(w, http.StatusBadRequest, "action must be queue, deep or delete")
		return
	}
	counts := map[string]int{}
	for _, id := range body.IDs {
		b, err := s.Store.BookByID(id, u)
		if err != nil {
			counts["skipped"]++
			continue
		}
		counts[act(b)]++
	}
	switch {
	case counts["requested"] > 0 && body.Action == "delete":
		s.Store.Notify("info", "delete-requests", deleteReviewMsg, "#/deletions")
	case counts["requested"] > 0:
		s.Store.Notify("info", "deep-scan-requests", "Books are waiting for your Deep Scan approval.", "#/deepscan")
	case counts["started"] > 0:
		s.Deep.Wake()
	}
	writeJSON(w, http.StatusOK, counts)
}

func outcome(err error, ok string) string {
	if err != nil {
		return "skipped"
	}
	return ok
}

func (s *Server) bulkDeep(b *store.Book, u *store.User, reason string) string {
	_, e, err := s.Deep.Prepare(b.ID)
	if err != nil {
		return "no_epub"
	}
	admin := u.Role == store.RoleAdmin
	source := map[bool]string{true: "admin", false: "request"}[admin]
	if _, err := s.Store.RequestDeepRead(b.ID, source, u.Username, reason, admin, e.Words, e.Parts, e.Tokens); err != nil {
		if errors.Is(err, store.ErrDeepReadOpen) {
			return "already"
		}
		return "skipped"
	}
	return map[bool]string{true: "started", false: "requested"}[admin]
}

// bulkDelete takes a book out of the libraries u owns; a book that isn't in
// any of them gets a delete request instead.
func (s *Server) bulkDelete(b *store.Book, u *store.User, reason string, owners map[int64]*int64) string {
	copies, err := s.Store.BookCopies(b.ID)
	if err != nil {
		return "skipped"
	}
	removed := false
	for _, c := range copies {
		owner, known := owners[c.CatalogID]
		if !known {
			if cat, err := s.Store.CatalogByID(c.CatalogID); err == nil {
				owner = cat.OwnerID
			}
			owners[c.CatalogID] = owner
		}
		if owner != nil && *owner == u.ID {
			if ok, err := s.Store.RemoveFromCatalog(c.CatalogID, b.ID); err == nil && ok {
				removed = true
			}
		}
	}
	switch {
	case removed:
		return "removed"
	case !s.Store.Owned(b.ID):
		return "skipped" // only looked up: nothing to delete
	}
	return outcome(s.Store.RequestDelete(b, u, reason), "requested")
}
