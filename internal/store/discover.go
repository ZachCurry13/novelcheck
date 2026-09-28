package store

// Discover: the outside lists behind the Discover tab (New York Times best
// sellers, Open Library classics) and the family's own new and popular books.

import "strconv"

const (
	KeyNYTAPIKey           = "nyt_api_key"            // New York Times Books API key (optional)
	KeyDiscoverDaily       = "discover_daily_ratings" // Discover books the AI rates per day
	KeyDiscoverLastRefresh = "discover_last_refresh"  // RFC 3339 time of the last list refresh
	KeyDiscoverLastResult  = "discover_last_result"   // what it found, or why it failed
)

func init() {
	Defaults[KeyDiscoverDaily] = "30"
	SecretKeys[KeyNYTAPIKey] = true
}

// DiscoverEntry is one book on an outside list.
type DiscoverEntry struct {
	Rank        int
	Title       string
	Author      string
	ISBN        string
	Description string
	CoverURL    string // the list's cover image
	Link        string // the book's Open Library page, when known
	WeeksOnList int    // NYT: weeks on the best-seller list (1 = new this week)
}

// DiscoverBook is a book on the Discover tab.
type DiscoverBook struct {
	Book
	List   string `db:"list" json:"list"`
	Rank   int    `db:"rank" json:"rank"`
	Weeks  int    `db:"weeks_on_list" json:"weeks_on_list"`
	Link   string `db:"link" json:"link"`
	Owned  bool   `db:"owned" json:"owned"`   // in one of the family's libraries
	Wished bool   `db:"wished" json:"wished"` // on the viewer's wishlist
	Queued bool   `db:"queued" json:"queued"` // in the viewer's Up Next
}

// SaveDiscoverList replaces a list's books. A book the family has nowhere
// yet is filed under the Discover catalog and waits to be rated.
func (s *Store) SaveDiscoverList(list string, entries []DiscoverEntry) (int, error) {
	cat, err := s.EnsureCatalog(DiscoverCatalog, "custom")
	if err != nil {
		return 0, err
	}
	if err := s.ClearDiscoverList(list); err != nil {
		return 0, err
	}
	n := 0
	for i, e := range entries {
		id, err := s.UpsertBook(e.Title, e.Author, e.ISBN, e.Description)
		if err != nil {
			continue // no usable title
		}
		var copies int
		_ = s.DB.Get(&copies, `SELECT COUNT(*) FROM catalog_books WHERE book_id = ?`, id)
		if copies == 0 {
			if err := s.AddCopy(cat, id, "discover:"+strconv.FormatInt(id, 10), "list", ""); err != nil {
				return n, err
			}
		}
		rank := e.Rank
		if rank <= 0 {
			rank = i + 1
		}
		if _, err := s.DB.Exec(`INSERT OR IGNORE INTO discover_items (list, rank, book_id, weeks_on_list, cover_url, link)
			VALUES (?, ?, ?, ?, ?, ?)`, list, rank, id, e.WeeksOnList, e.CoverURL, e.Link); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// ClearDiscoverList empties a list (e.g. a fallback once the NYT key works).
func (s *Store) ClearDiscoverList(list string) error {
	_, err := s.DB.Exec(`DELETE FROM discover_items WHERE list = ?`, list)
	return err
}

// discoverCols selects a book plus the viewer's own marks (two args: the viewer id).
const discoverCols = bookCols + derivedCols + `, '' AS catalogs, ` + ownedCond + ` AS owned,
	EXISTS (SELECT 1 FROM wishlist w WHERE w.book_id = b.id AND w.user_id = ? AND w.status IN ('wanted', 'approved')) AS wished,
	EXISTS (SELECT 1 FROM queue_items q WHERE q.book_id = b.id AND q.user_id = ?) AS queued`

// discoverWhere is the viewer's content rules; kids also never see books the
// AI hasn't rated yet (a parent's age group counts as rated).
func discoverWhere(u *User) (string, []any) {
	where, args := visibilityClause(u)
	if u != nil && u.Role == RoleRestricted {
		where += " AND (b.status = 'analyzed' OR b.age_level > 0)"
	}
	return where, args
}

func viewerID(u *User) int64 {
	if u == nil {
		return 0
	}
	return u.ID
}

// DiscoverItems returns every listed book the viewer may see, by list and rank.
func (s *Store) DiscoverItems(u *User) ([]DiscoverBook, error) {
	where, args := discoverWhere(u)
	out := []DiscoverBook{}
	err := s.DB.Select(&out, `SELECT `+discoverCols+`, d.list, d.rank, d.weeks_on_list, d.link
		FROM discover_items d JOIN books b ON b.id = d.book_id WHERE 1=1`+where+` ORDER BY d.list, d.rank`,
		append([]any{viewerID(u), viewerID(u)}, args...)...)
	return out, err
}

// NewInLibraries returns books the family added in the last 30 days, newest
// first. The first sync's books don't count, or everything would be "new".
func (s *Store) NewInLibraries(u *User, limit int) ([]DiscoverBook, error) {
	where, args := discoverWhere(u)
	out := []DiscoverBook{}
	err := s.DB.Select(&out, `WITH first_added AS (SELECT cb.book_id, MIN(cb.added_at) AS added FROM catalog_books cb
			JOIN catalogs c ON c.id = cb.catalog_id WHERE c.name NOT IN ('`+LookedUpCatalog+`', '`+DiscoverCatalog+`')
			GROUP BY cb.book_id)
		SELECT `+discoverCols+`, 'new_in_library' AS list, 0 AS rank, 0 AS weeks_on_list, '' AS link
		FROM first_added fa JOIN books b ON b.id = fa.book_id
		WHERE fa.added >= datetime('now', '-30 days') AND fa.added > datetime((SELECT MIN(added) FROM first_added), '+1 day')`+
		where+` ORDER BY fa.added DESC, b.id DESC LIMIT ?`,
		append(append([]any{viewerID(u), viewerID(u)}, args...), limit)...)
	return out, err
}

// FamilyFavorites returns the family's books most added to Up Next (or read)
// in the last 90 days, by how many people chose them.
func (s *Store) FamilyFavorites(u *User, limit int) ([]DiscoverBook, error) {
	where, args := discoverWhere(u)
	out := []DiscoverBook{}
	err := s.DB.Select(&out, `SELECT `+discoverCols+`, 'family' AS list, 0 AS rank, 0 AS weeks_on_list, '' AS link
		FROM books b JOIN (SELECT book_id, COUNT(DISTINCT user_id) AS n, MAX(updated_at) AS last FROM queue_items
			WHERE updated_at >= datetime('now', '-90 days') GROUP BY book_id) p ON p.book_id = b.id
		WHERE `+ownedCond+where+` ORDER BY p.n DESC, p.last DESC LIMIT ?`,
		append(append([]any{viewerID(u), viewerID(u)}, args...), limit)...)
	return out, err
}

// QueueDiscoverRatings marks up to n unrated listed books for rating, best
// ranked first, and returns their ids for the worker.
func (s *Store) QueueDiscoverRatings(n int) ([]int64, error) {
	var ids []int64
	if n <= 0 {
		return nil, nil
	}
	if err := s.DB.Select(&ids, `SELECT b.id FROM books b JOIN (SELECT book_id, MIN(rank) AS r FROM discover_items
		GROUP BY book_id) d ON d.book_id = b.id WHERE b.status = 'pending' ORDER BY d.r, b.id LIMIT ?`, n); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if err := s.SetStatus(id, "queued", ""); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

// DiscoverCoverURL is a listed book's cover image from its list, or "".
func (s *Store) DiscoverCoverURL(bookID int64) string {
	var u string
	_ = s.DB.Get(&u, `SELECT cover_url FROM discover_items WHERE book_id = ? AND cover_url != '' LIMIT 1`, bookID)
	return u
}

// DiscoverCounts is how many books the lists hold, and how many are rated.
type DiscoverCounts struct {
	Listed  int `db:"listed" json:"listed"`
	Rated   int `db:"rated" json:"rated"`
	Waiting int `db:"waiting" json:"waiting"`
}

func (s *Store) DiscoverCounts() DiscoverCounts {
	var c DiscoverCounts
	_ = s.DB.Get(&c, `SELECT COUNT(*) AS listed, COALESCE(SUM(b.status = 'analyzed'), 0) AS rated,
		COALESCE(SUM(b.status IN ('pending', 'queued', 'processing')), 0) AS waiting
		FROM books b WHERE b.id IN (SELECT book_id FROM discover_items)`)
	return c
}
