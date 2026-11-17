package api

import (
	"context"
	"time"

	"github.com/zachcurry13/novelcheck/internal/push"
)

// New books in the family libraries are announced on everyone's phone (with notifications on): one notice for
// e-books and one for audiobooks, at most every half hour, and never for the first look after start-up or for a big
// first sync. Switch off with the setting new_book_notices = false.

const keyNewBooksSeen = "new_books_seen"

// AnnounceNewBooks sends the notices for what came in since the last look. send is called once per person.
func (s *Server) AnnounceNewBooks(now time.Time, send func(userID int64, m push.Message)) {
	stamp := now.UTC().Format("2006-01-02 15:04:05")
	since := s.Store.Setting(keyNewBooksSeen)
	_ = s.Store.SetSetting(keyNewBooksSeen, stamp)
	if since == "" || s.Store.Setting("new_book_notices") == "false" {
		return
	}
	ebooks, audio := s.Store.NewBookCounts(since)
	var msgs []push.Message
	if ebooks > 0 && ebooks <= 200 {
		msgs = append(msgs, push.Message{Title: "📚 New in the library", Body: plural(ebooks, "new book"), URL: "/#/library", Tag: "new-books"})
	}
	if audio > 0 && audio <= 200 {
		msgs = append(msgs, push.Message{Title: "🎧 New audiobooks", Body: plural(audio, "new audiobook"), URL: "/#/library", Tag: "new-audio"})
	}
	if len(msgs) == 0 {
		return
	}
	users, _ := s.Store.ListUsers()
	for _, u := range users {
		for _, m := range msgs {
			send(u.ID, m)
		}
	}
}

// NewBooksLoop looks every half hour.
func (s *Server) NewBooksLoop(ctx context.Context) {
	t := time.NewTicker(30 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s.Push != nil {
				s.AnnounceNewBooks(time.Now(), s.Push.ToUser)
			}
		}
	}
}
