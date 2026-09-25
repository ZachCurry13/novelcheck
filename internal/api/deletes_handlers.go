package api

import (
	"net/http"
	"strconv"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

const deleteReviewMsg = "Books are waiting for your delete review."

// handleRequestDelete lets anyone ask an admin to delete a book they can see.
func (s *Server) handleRequestDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	u := auth.UserFrom(r)
	b, err := s.Store.BookByID(id, u)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if !s.Store.Owned(id) {
		writeErr(w, http.StatusBadRequest, "this book isn't in your library, so there's nothing to delete")
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if r.ContentLength > 0 && !readJSON(w, r, &body, 4<<10) {
		return
	}
	if err := s.Store.RequestDelete(b, u, body.Reason); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.Store.Notify("info", "delete-requests", deleteReviewMsg, "#/deletions")
	writeJSON(w, http.StatusOK, map[string]any{"request": s.Store.MyDeleteRequest(id, u.ID)})
}

func (s *Server) handleCancelDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.CancelDelete(id, auth.UserFrom(r).ID); err != nil {
		writeStoreErr(w, err)
		return
	}
	s.resolveDeleteNotice()
	writeJSON(w, http.StatusOK, map[string]any{"request": nil})
}

func (s *Server) resolveDeleteNotice() {
	if s.Store.PendingDeleteCount() == 0 {
		s.Store.Resolve("delete-requests")
	}
}

// handleDeleteRequests is the admin's review list plus recent decisions.
func (s *Server) handleDeleteRequests(w http.ResponseWriter, r *http.Request) {
	pending, err := s.Store.PendingDeletes()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	recent, err := s.Store.RecentDeleteDecisions(30)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if pending == nil {
		pending = []store.DeleteGroup{}
	}
	if recent == nil {
		recent = []store.DeleteRequest{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pending": pending, "recent": recent,
		"can_remove": s.Store.Setting(store.KeyCalibreSrvURL) != "",
	})
}

// handleDecideDeletes applies the admin's decision to the chosen books:
// "delete" removes their Calibre entries (to calibre's recycle bin, title
// checked) then closes the requests; "dismiss" keeps the books; "done"
// closes requests the admin handled elsewhere (e.g. deleted on the Kindle).
func (s *Server) handleDecideDeletes(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BookIDs []int64 `json:"book_ids"`
		Action  string  `json:"action"`
	}
	if !readJSON(w, r, &body, 64<<10) {
		return
	}
	if len(body.BookIDs) == 0 {
		writeErr(w, http.StatusBadRequest, "pick at least one book")
		return
	}
	by := auth.UserFrom(r).Username
	out := map[string]int{"removed": 0, "skipped": 0, "not_in_calibre": 0}
	switch body.Action {
	case "dismiss", "done":
		status := map[string]string{"dismiss": "dismissed", "done": "done"}[body.Action]
		if err := s.Store.DecideDeletes(body.BookIDs, status, by); err != nil {
			writeStoreErr(w, err)
			return
		}
	case "delete":
		if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
			writeErr(w, http.StatusBadRequest, "set up one-click removal (the calibre Content server) first, or use Copy Calibre search and Mark done")
			return
		}
		var ids []int
		want := map[int]string{}
		var inCalibre, notInCalibre []int64
		for _, bid := range body.BookIDs {
			b, err := s.Store.BookByID(bid, nil)
			if err != nil {
				continue
			}
			copies, _ := s.Store.BookCopies(bid)
			found := false
			for _, c := range copies {
				if c.Source != "calibre" {
					continue
				}
				if n, err := strconv.Atoi(c.ExternalID); err == nil {
					if _, dup := want[n]; !dup {
						ids = append(ids, n)
						want[n] = b.Title
					}
					found = true
				}
			}
			if found {
				inCalibre = append(inCalibre, bid)
			} else {
				notInCalibre = append(notInCalibre, bid)
			}
		}
		if len(ids) > 0 {
			reqIDs, err := s.Store.PendingRequestIDs(inCalibre) // before the books disappear
			if err != nil {
				writeStoreErr(w, err)
				return
			}
			removed, skipped, ok := s.removeFromCalibre(w, r, ids, want)
			if !ok {
				return // nothing is marked; the error explains why
			}
			out["removed"], out["skipped"] = removed, skipped
			_ = s.Store.DecideRequests(reqIDs, "deleted", by)
		}
		out["not_in_calibre"] = len(notInCalibre) // left pending: delete them on the device, then Mark done
	default:
		writeErr(w, http.StatusBadRequest, "action must be delete, dismiss or done")
		return
	}
	s.resolveDeleteNotice()
	writeJSON(w, http.StatusOK, out)
}
