package api

import (
	"net/http"
	"strconv"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/push"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Book requests. Anyone can ask for a book (the kids' way to ask a parent). A parent who can manage that person answers:
// Yes puts the book on their Nightstand and marks it OK for kids' accounts (the same "OK" a parent gives on a book's page);
// a book that isn't in the library yet is approved, and stays in the admins' inbox until someone gets it ("Got it").

// handleRequests: GET /api/requests: your own asks, and (for a parent) the ones waiting for an answer.
func (s *Server) handleRequests(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	out := map[string]any{"mine": s.Store.RequestsFor(u.ID)}
	if u.CanManage() {
		out["open"] = s.openRequestsFor(u)
	}
	writeJSON(w, http.StatusOK, out)
}

// openRequestsFor are the waiting asks of the people this parent manages.
func (s *Server) openRequestsFor(u *store.User) []store.Request {
	out := []store.Request{}
	for _, q := range s.Store.OpenRequests() {
		if target, err := s.Store.UserByID(q.UserID); err == nil && target != nil && (target.ID == u.ID || canManageUser(u, target)) {
			out = append(out, q)
		}
	}
	return out
}

// handleAddRequest: POST /api/requests {"title", "author", "note"}.
func (s *Server) handleAddRequest(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	var body struct {
		Title  string `json:"title"`
		Author string `json:"author"`
		Note   string `json:"note"`
	}
	if !readJSON(w, r, &body, 2<<10) {
		return
	}
	id, err := s.Store.AddRequest(u.ID, body.Title, body.Author, body.Note)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.Store.Notify("info", "requests", u.Username+" asked for “"+body.Title+"”.", "#/home")
	writeJSON(w, http.StatusOK, map[string]int64{"id": id})
}

// handleDecideRequest: POST /api/requests/{id}/decide {"action": "approve" | "decline" | "done", "answer", "book_id"}.
func (s *Server) handleDecideRequest(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	q, err := s.Store.RequestByID(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	target, err := s.Store.UserByID(q.UserID)
	if err != nil || target == nil || !u.CanManage() || (target.ID != u.ID && !canManageUser(u, target)) {
		writeErr(w, http.StatusForbidden, "you can answer the asks of the people you manage")
		return
	}
	var body struct {
		Action string `json:"action"`
		Answer string `json:"answer"`
		BookID int64  `json:"book_id"`
	}
	if !readJSON(w, r, &body, 2<<10) {
		return
	}
	bookID := body.BookID
	if bookID == 0 && q.BookID != nil {
		bookID = *q.BookID
	}
	if bookID == 0 && body.Action != "decline" { // it may be in the library by now
		bookID = s.Store.MatchBook(q.Title, q.Author)
	}
	switch body.Action {
	case "decline":
		if q.Status != "pending" {
			writeErr(w, http.StatusConflict, "that one is already answered")
			return
		}
		_ = s.Store.DecideRequest(id, "declined", body.Answer, u.Username)
	case "approve", "done":
		if body.Action == "approve" && q.Status != "pending" || body.Action == "done" && q.Status != "approved" {
			writeErr(w, http.StatusConflict, "that one isn't waiting for this")
			return
		}
		answer, status := body.Answer, "approved"
		if bookID > 0 { // it is a library book: OK for kids, and on their Nightstand
			if _, err := s.Store.BookByID(bookID, nil); err != nil {
				writeStoreErr(w, err)
				return
			}
			_ = s.Store.SetApproved(bookID, true, u.Username)
			_ = s.Store.Enqueue(q.UserID, bookID)
			s.Store.LinkRequest(id, bookID)
			status = "done"
			if answer == "" {
				answer = "It's on your Nightstand."
			}
		} else if body.Action == "done" {
			writeErr(w, http.StatusBadRequest, "pick the library book it became")
			return
		} else if answer == "" {
			answer = "Yes. It isn't in the library yet; a parent will get it."
		}
		_ = s.Store.DecideRequest(id, status, answer, u.Username)
	default:
		writeErr(w, http.StatusBadRequest, "approve, decline or done")
		return
	}
	s.tellRequester(id)
	writeJSON(w, http.StatusOK, map[string]string{"ok": strconv.FormatInt(id, 10)})
}

// tellRequester sends the person who asked a phone notice with the parent's answer (Yes, or not this time).
func (s *Server) tellRequester(id int64) {
	q, err := s.Store.RequestByID(id)
	if err != nil || s.Push == nil || (q.Status != "approved" && q.Status != "done" && q.Status != "declined") {
		return
	}
	title, body := "✅ Yes: "+q.Title, q.Answer
	if q.Status == "declined" {
		title = "Not this time: " + q.Title
	}
	if body == "" {
		body = "A parent answered your ask."
	}
	s.Push.ToUser(q.UserID, push.Message{Title: title, Body: body, URL: "/#/home", Tag: "request-" + strconv.FormatInt(id, 10)})
}
