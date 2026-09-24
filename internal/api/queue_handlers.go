package api

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/delivery"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func (s *Server) handleListQueue(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListQueue(auth.UserFrom(r).ID, r.URL.Query().Get("all") == "1")
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if items == nil {
		items = []store.QueueItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleEnqueue(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	var body struct {
		BookID int64 `json:"book_id"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	// Visibility check: restricted users can't queue titles they can't see.
	if _, err := s.Store.BookByID(body.BookID, u); err != nil {
		writeStoreErr(w, err)
		return
	}
	if err := s.Store.Enqueue(u.ID, body.BookID); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

// handleReorderQueue persists a SortableJS drag-and-drop ordering.
func (s *Server) handleReorderQueue(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if !readJSON(w, r, &body, 256<<10) {
		return
	}
	if err := s.Store.ReorderQueue(auth.UserFrom(r).ID, body.IDs); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDequeue(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.RemoveFromQueue(auth.UserFrom(r).ID, id); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleStartReading delivers the book per the user's preference and moves
// it from Queued to Currently Reading.
func (s *Server) handleStartReading(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := s.Store.QueueItem(u.ID, id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if _, err := s.Store.BookByID(item.BookID, u); err != nil {
		writeStoreErr(w, err) // profile rules changed since it was queued
		return
	}
	note, err := s.deliver(u, item.BookID)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "delivery failed: "+err.Error())
		return
	}
	if err := s.Store.SetQueueStatus(u.ID, id, "reading", note); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reading", "delivery_note": note})
}

func (s *Server) deliver(u *store.User, bookID int64) (string, error) {
	switch u.DeliveryMethod {
	case "koreader":
		return "Flagged for KOReader wireless sync", nil
	case "email":
		copies, err := s.Store.BookCopies(bookID)
		if err != nil {
			return "", err
		}
		best, ok := delivery.BestFile(copies, true)
		if !ok || !s.insideCalibre(best.Path) {
			return "", fmt.Errorf("no EPUB or PDF copy of this book exists in the Calibre library")
		}
		cfg := delivery.SMTPConfig{
			Host:     s.Store.Setting(store.KeySMTPHost),
			Port:     s.Store.Setting(store.KeySMTPPort),
			Username: s.Store.Setting(store.KeySMTPUser),
			Password: s.Store.Setting(store.KeySMTPPassword),
			From:     s.Store.Setting(store.KeySMTPFrom),
		}
		if err := delivery.SendFile(cfg, u.KindleEmail, best.Path); err != nil {
			log.Printf("send-to-kindle for user %d failed: %v", u.ID, err)
			s.Store.Notify("warning", "delivery", "Send-to-Kindle failed: "+err.Error(), "#/admin")
			return "", err
		}
		return "Emailed " + filepath.Base(best.Path) + " to " + u.KindleEmail, nil
	default:
		return "Marked as reading (no delivery method set)", nil
	}
}

func (s *Server) handleFinishReading(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.Store.QueueItem(u.ID, id); err != nil {
		writeStoreErr(w, err)
		return
	}
	if err := s.Store.SetQueueStatus(u.ID, id, "finished", ""); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "finished"})
}
