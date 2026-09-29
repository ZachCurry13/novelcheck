package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Reading progress pages: the books opened on your KOReader devices, and
// for parents what a kid is reading and how far along they are.

// koreaderList is someone's KOReader books and when their statistics last
// synced (0 if never).
func (s *Server) koreaderList(userID int64, limit int) (map[string]any, error) {
	books, err := s.Store.KOReaderBooks(userID, limit)
	if err != nil {
		return nil, err
	}
	var synced int64
	if _, at := s.statsPath(userID); !at.IsZero() {
		synced = at.Unix()
	}
	return map[string]any{"books": books, "synced_at": synced}, nil
}

// handleMyKOReaderBooks: GET /api/me/koreader-books.
func (s *Server) handleMyKOReaderBooks(w http.ResponseWriter, r *http.Request) {
	out, err := s.koreaderList(auth.UserFrom(r).ID, 200)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleUserReading: GET /api/admin/users/{id}/reading (parents; editors
// only for kids): what they're reading, how far, and their KOReader books.
func (s *Server) handleUserReading(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	target, err := s.Store.UserByID(id)
	if err != nil || target == nil {
		writeErr(w, http.StatusNotFound, "no such user")
		return
	}
	if auth.UserFrom(r).Role != store.RoleAdmin && target.Role != store.RoleRestricted {
		writeErr(w, http.StatusForbidden, "editors can only see kids' reading")
		return
	}
	items, err := s.Store.ListQueue(target.ID, false)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	progress := s.Store.ReadingProgress(target.ID)
	reading := []store.QueueItem{}
	for _, it := range items {
		if it.Status != "reading" {
			continue
		}
		if p, ok := progress[it.BookID]; ok {
			it.Progress = &p
		}
		reading = append(reading, it)
	}
	out, err := s.koreaderList(target.ID, 10)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	out["reading"] = reading
	writeJSON(w, http.StatusOK, out)
}
