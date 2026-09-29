package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/kosync"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// KOReader progress sync (KOReader: Tools → Progress sync → Custom sync
// server, then the NovelCheck address followed by /kosync). KOReader signs
// in with the NovelCheck name and the sync code from Profile, sent as
// x-auth-user and x-auth-key (the code's MD5). No session or CSRF header:
// the code is the key, like the KOReader catalog's address.

func kosyncJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// kosyncUser is the reader signing in, or nil (and 401 sent).
func (s *Server) kosyncUser(w http.ResponseWriter, r *http.Request) *store.User {
	if !s.Store.SettingBool(store.KeyModuleKOReader) {
		kosyncJSON(w, http.StatusUnauthorized, map[string]string{"message": "KOReader is turned off in NovelCheck (Admin → System & Toggles → Features)"})
		return nil
	}
	u, ok := s.Store.KosyncUser(r.Header.Get("x-auth-user"), r.Header.Get("x-auth-key"))
	if !ok {
		kosyncJSON(w, http.StatusUnauthorized, map[string]string{"message": "Unauthorized: use your NovelCheck name and the sync code from Profile"})
		return nil
	}
	return u
}

// handleKosyncCreate: KOReader's "Register" signs in, since accounts are
// NovelCheck's own; a wrong name or code is refused.
func (s *Server) handleKosyncCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "bad request"})
		return
	}
	if _, ok := s.Store.KosyncUser(body.Username, body.Password); !ok {
		kosyncJSON(w, http.StatusPaymentRequired, map[string]string{"message": "Use your NovelCheck name and the sync code from Profile → KOReader"})
		return
	}
	kosyncJSON(w, http.StatusCreated, map[string]string{"username": body.Username})
}

func (s *Server) handleKosyncAuth(w http.ResponseWriter, r *http.Request) {
	if s.kosyncUser(w, r) != nil {
		kosyncJSON(w, http.StatusOK, map[string]string{"authorized": "OK"})
	}
}

// handleKosyncPut keeps the reader's place and moves the book along in
// their Up Next when NovelCheck knows which book it is.
func (s *Server) handleKosyncPut(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var p store.KosyncProgress
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&p); err != nil || strings.TrimSpace(p.Document) == "" {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "bad request"})
		return
	}
	p.Document = strings.ToLower(strings.TrimSpace(p.Document))
	p.Timestamp = time.Now().Unix()
	if err := s.Store.SaveKosync(u.ID, p); err != nil {
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "could not save"})
		return
	}
	if book := s.kosyncBook(u.ID, p.Document); book > 0 && p.Percentage > 0 {
		_, _ = s.Store.SyncReading(u.ID, book, p.Percentage >= store.FinishedAt)
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"document": p.Document, "timestamp": p.Timestamp})
}

func (s *Server) handleKosyncGet(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	if p, ok := s.Store.Kosync(u.ID, strings.ToLower(chi.URLParam(r, "document"))); ok {
		kosyncJSON(w, http.StatusOK, p)
		return
	}
	kosyncJSON(w, http.StatusOK, map[string]any{})
}

// kosyncBook finds the book a fingerprint belongs to: known already, or one
// of the files of the reader's Up Next books (worked out now, at most 40).
func (s *Server) kosyncBook(userID int64, document string) int64 {
	if id := s.Store.DocBook(document); id > 0 {
		return id
	}
	for _, f := range s.Store.SyncFiles(userID, 40) {
		if !s.insideCalibre(f.Path) {
			continue
		}
		if sum, err := kosync.PartialMD5(f.Path); err == nil {
			s.Store.SetDocBook(sum, f.BookID)
			if sum == document {
				return f.BookID
			}
		}
	}
	return 0
}

// rememberFingerprint notes the fingerprint of a file NovelCheck sends to
// an e-reader, so KOReader's progress finds its book at once.
func (s *Server) rememberFingerprint(bookID int64, path string) {
	if sum, err := kosync.PartialMD5(path); err == nil {
		s.Store.SetDocBook(sum, bookID)
	}
}

// handleMyKosync returns the reader's sync code ({"renew": true} makes a new one).
func (s *Server) handleMyKosync(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	code, err := s.Store.KosyncCode(u.ID, r.Method == http.MethodPost)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": u.Username, "code": code, "path": "/kosync"})
}

// handleUserKosync is a kid's (or, for admins, anyone's) sync code, for a
// parent setting up their e-reader.
func (s *Server) handleUserKosync(w http.ResponseWriter, r *http.Request) {
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
		writeErr(w, http.StatusForbidden, "editors can only set up kids' devices")
		return
	}
	code, err := s.Store.KosyncCode(target.ID, r.Method == http.MethodPost)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": target.Username, "code": code, "path": "/kosync"})
}
