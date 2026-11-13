package store

import "database/sql"

// Switches for finding free books by themselves (both on by default).
const (
	KeyEventsAutoSYK     = "events_auto_syk"     // make each Stuff Your Kindle day an event on its day
	KeyEventsAutoReddit  = "events_auto_reddit"  // keep "Free right now" from r/FreeEBOOKS
	KeyEventsAutoBookBub = "events_auto_bookbub" // keep "BookBub free today" from the newest BookBub email
	// The mailbox BookBub's emails arrive in, read-only over IMAP.
	KeyEventsMailHost     = "events_mail_host"
	KeyEventsMailUser     = "events_mail_user"
	KeyEventsMailPassword = "events_mail_password" // an app password, never shown again
	KeyEventsBookBubSeen  = "events_bookbub_seen"  // the last BookBub email read (its Message-ID)
)

func init() {
	Defaults[KeyEventsAutoSYK] = "true"
	Defaults[KeyEventsAutoReddit] = "true"
	Defaults[KeyEventsAutoBookBub] = "true"
	Defaults[KeyEventsMailHost] = "imap.gmail.com:993"
	Defaults[KeyEventsMailUser] = ""
	SecretKeys[KeyEventsMailPassword] = true
}

// Events NovelCheck makes by itself (Stuff Your Kindle days, the standing
// list of books free right now) are found again by the page they came from.

// EventBySource is the event made from a page, and whether it is archived.
func (s *Store) EventBySource(sourceURL string) (id int64, archived, found bool) {
	var row struct {
		ID         int64  `db:"id"`
		ArchivedAt string `db:"archived_at"`
	}
	err := s.DB.Get(&row, `SELECT id, archived_at FROM events WHERE source_url = ? ORDER BY id DESC LIMIT 1`, sourceURL)
	if err == sql.ErrNoRows || err != nil {
		return 0, false, false
	}
	return row.ID, row.ArchivedAt != "", true
}

// PinEventOn keeps an event on the Events page (a standing list never ends).
func (s *Store) PinEventOn(id int64) error {
	_, err := s.DB.Exec(`UPDATE events SET pinned = 1 WHERE id = ?`, id)
	return err
}

// ReplaceEventBooks makes entries an event's whole list (books that left
// it leave the Events catalog unless another event lists them), and
// returns the newly listed books that still need rating.
func (s *Store) ReplaceEventBooks(id int64, entries []EventEntry) ([]int64, error) {
	cat, err := s.EnsureCatalog(EventsCatalog, "custom")
	if err != nil {
		return nil, err
	}
	var before []int64
	_ = s.DB.Select(&before, `SELECT book_id FROM event_books WHERE event_id = ?`, id)
	if _, err := s.DB.Exec(`DELETE FROM event_books WHERE event_id = ?`, id); err != nil {
		return nil, err
	}
	unrated, err := s.addEventBooks(id, cat, entries)
	if err != nil {
		return unrated, err
	}
	known := map[int64]bool{}
	for _, b := range before {
		known[b] = true
	}
	fresh := unrated[:0]
	for _, b := range unrated {
		if !known[b] {
			fresh = append(fresh, b) // already waiting from an earlier round
		}
	}
	return fresh, s.dropUnlistedEventCopies()
}
