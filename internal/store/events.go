package store

// Stuff Your Kindle events: a parent pastes an event's list of (often free)
// e-books; NovelCheck lists them with their ratings, what the family already
// has, and a Claim on Amazon link. Books only on an event's list live in the
// hidden "Events" catalog, like Discover's, until someone claims one ("✓ I
// claimed it" adds it to a library). An event is archived when it ends, or
// after 30 days unless pinned; archived events keep their books and can be
// restored or deleted.

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
)

// EventsCatalog holds the books only listed on an event.
const EventsCatalog = "Events"

// eventDaysSQL is how long an unpinned event without an end stays out of
// the archive.
const eventDaysSQL = "30"

// Event is one event as someone sees it.
type Event struct {
	ID         int64  `db:"id" json:"id"`
	Name       string `db:"name" json:"name"`
	SourceURL  string `db:"source_url" json:"source_url"`
	CreatedBy  string `db:"created_by" json:"created_by"`
	CreatedAt  string `db:"created_at" json:"created_at"`
	Pinned     bool   `db:"pinned" json:"pinned"`
	EndsAt     string `db:"ends_at" json:"ends_at"`         // RFC 3339 UTC, or ""
	ArchivedAt string `db:"archived_at" json:"archived_at"` // RFC 3339 UTC, or "" while it's on
	Books      int    `db:"books" json:"books"`             // listed books the viewer may see
	Rated      int    `db:"rated" json:"rated"`             // of those, rated
	DaysLeft   int    `db:"days_left" json:"days_left"`     // before an unpinned event without an end is archived
}

// EventEntry is one book as an event lists it.
type EventEntry struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	ASIN   string `json:"asin"`
	Link   string `json:"link"`
}

// EventBook is a listed book with the viewer's marks.
type EventBook struct {
	Book
	Position int    `db:"position" json:"position"`
	ASIN     string `db:"asin" json:"asin"`
	Link     string `db:"link" json:"link"`
	Owned    bool   `db:"owned" json:"owned"`
	Wished   bool   `db:"wished" json:"wished"`
	Queued   bool   `db:"queued" json:"queued"`
}

const eventCols = `e.id, e.name, e.source_url, e.created_by, e.created_at, e.pinned, e.ends_at, e.archived_at,
	CAST(MAX(0, ` + eventDaysSQL + ` - (julianday('now') - julianday(e.created_at))) AS INTEGER) AS days_left`

// CreateEvent saves an event with its books (matched to books NovelCheck
// has, or new ones filed under Events) and returns its id and the books that
// still need a rating. endsAt is when it's archived (UTC, RFC 3339), or "".
func (s *Store) CreateEvent(name, sourceURL, by, endsAt string, entries []EventEntry) (int64, []int64, error) {
	cat, err := s.EnsureCatalog(EventsCatalog, "custom")
	if err != nil {
		return 0, nil, err
	}
	res, err := s.DB.Exec(`INSERT INTO events (name, source_url, created_by, ends_at) VALUES (?, ?, ?, ?)`, strings.TrimSpace(name), sourceURL, by, endsAt)
	if err != nil {
		return 0, nil, err
	}
	id, _ := res.LastInsertId()
	var unrated []int64
	for i, e := range entries {
		book, err := s.UpsertBook(e.Title, e.Author, "", "")
		if err != nil {
			continue // no usable title
		}
		var copies int
		_ = s.DB.Get(&copies, `SELECT COUNT(*) FROM catalog_books WHERE book_id = ?`, book)
		if copies == 0 {
			if err := s.AddCopy(cat, book, "event:"+e.ASIN+":"+e.Title, "list", e.ASIN); err != nil {
				return id, unrated, err
			}
		}
		res, err := s.DB.Exec(`INSERT OR IGNORE INTO event_books (event_id, book_id, position, asin, link) VALUES (?, ?, ?, ?, ?)`,
			id, book, i+1, e.ASIN, e.Link)
		if err != nil {
			return id, unrated, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			var status string
			_ = s.DB.Get(&status, `SELECT status FROM books WHERE id = ?`, book)
			if status == "pending" || status == "error" {
				unrated = append(unrated, book)
			}
		}
	}
	return id, unrated, nil
}

// eventWhere is the viewer's content rules; kids never see unrated books.
func eventWhere(u *User) (string, []any) { return discoverWhere(u) }

// ListEvents returns the events (pinned, then newest; the archive last,
// latest first) with how many of their books the viewer may see.
func (s *Store) ListEvents(u *User) ([]Event, error) {
	where, args := eventWhere(u)
	out := []Event{}
	err := s.DB.Select(&out, `SELECT `+eventCols+`,
		(SELECT COUNT(*) FROM event_books eb JOIN books b ON b.id = eb.book_id WHERE eb.event_id = e.id`+where+`) AS books,
		(SELECT COUNT(*) FROM event_books eb JOIN books b ON b.id = eb.book_id WHERE eb.event_id = e.id AND b.status = 'analyzed'`+where+`) AS rated
		FROM events e ORDER BY e.archived_at != '', e.archived_at DESC, e.pinned DESC, e.created_at DESC, e.id DESC`, append(args, args...)...)
	return out, err
}

// EventByID returns one event.
func (s *Store) EventByID(id int64, u *User) (*Event, error) {
	all, err := s.ListEvents(u)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].ID == id {
			return &all[i], nil
		}
	}
	return nil, ErrNotFound
}

// EventBooks lists an event's books the viewer may see, in list order.
func (s *Store) EventBooks(id int64, u *User) ([]EventBook, error) {
	where, args := eventWhere(u)
	out := []EventBook{}
	err := s.DB.Select(&out, `SELECT `+discoverCols+`, eb.position, eb.asin, eb.link
		FROM event_books eb JOIN books b ON b.id = eb.book_id WHERE eb.event_id = ?`+where+` ORDER BY eb.position`,
		append([]any{viewerID(u), viewerID(u), id}, args...)...)
	return out, err
}

// ErrNotLibrary is returned when a claimed book can't go into that library.
var ErrNotLibrary = errors.New("pick one of your own e-book libraries")

// ClaimEventBook records that someone got an event book: it goes into their
// chosen library (not Calibre, a paper library or a hidden list), so it
// counts as the family's everywhere.
func (s *Store) ClaimEventBook(eventID, bookID, catalogID int64, u *User) error {
	var asin string
	if err := s.DB.Get(&asin, `SELECT asin FROM event_books WHERE event_id = ? AND book_id = ?`, eventID, bookID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	c, err := s.CatalogByID(catalogID)
	if err != nil {
		return err
	}
	if c.Source == "calibre" || c.Physical || ReservedCatalogName(c.Name) || !c.CanEdit(u) {
		return ErrNotLibrary
	}
	key := asin
	if key == "" {
		key = "book-" + strconv.FormatInt(bookID, 10)
	}
	return s.AddCopy(catalogID, bookID, "claimed:"+key, "list", asin)
}
