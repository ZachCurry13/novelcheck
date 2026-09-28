package store

// Collections: shelves across libraries, made by a parent or filled by the
// AI from a theme ("idea" = an AI proposal a parent hasn't kept yet). A kid
// with only_collections sees only books in the collections picked for them.

import (
	"database/sql"
	"errors"
	"strings"
)

const (
	KeyCollectionIdeas     = "collection_ideas"      // weekly AI ideas: "" = on with a local AI; "on"; "off"
	KeyCollectionIdeasLast = "collection_ideas_last" // RFC 3339 time of the last ideas
)

func init() { Defaults[KeyCollectionIdeas] = "" }

// Collection is a collection as someone sees it.
type Collection struct {
	ID          int64   `db:"id" json:"id"`
	Name        string  `db:"name" json:"name"`
	Icon        string  `db:"icon" json:"icon"`
	Description string  `db:"description" json:"description"`
	Kind        string  `db:"kind" json:"kind"` // manual | ai | idea
	Theme       string  `db:"theme" json:"theme"`
	Season      string  `db:"season" json:"season"`
	CreatedBy   string  `db:"created_by" json:"created_by"`
	CreatedAt   string  `db:"created_at" json:"created_at"`
	Books       int     `db:"books" json:"books"` // books the viewer may see
	Covers      []int64 `db:"-" json:"covers"`    // up to 3 of them, for the card
}

const collectionCols = `c.id, c.name, c.icon, c.description, c.kind, c.theme, c.season, c.created_by, c.created_at`

// collectionScope limits the collections a viewer may see: kids limited to
// collections see only theirs, and ideas are for parents.
func collectionScope(u *User) (string, []any) {
	if u == nil || u.CanManage() {
		return "1=1", nil
	}
	if u.OnlyCollections {
		return "c.kind != 'idea' AND c.id IN (SELECT collection_id FROM user_collections WHERE user_id = ?)", []any{u.ID}
	}
	return "c.kind != 'idea'", nil
}

// ListCollections returns the collections viewer may see, each with how many
// of its books they may see (collections with none are left out for kids).
func (s *Store) ListCollections(viewer *User) ([]Collection, error) {
	vis, vargs := visibilityClause(viewer)
	scope, sargs := collectionScope(viewer)
	out := []Collection{}
	err := s.DB.Select(&out, `SELECT `+collectionCols+`, (SELECT COUNT(*) FROM collection_books cb JOIN books b ON b.id = cb.book_id
		WHERE cb.collection_id = c.id`+vis+`) AS books FROM collections c WHERE `+scope+`
		ORDER BY c.kind = 'idea', c.name COLLATE NOCASE`, append(vargs, sargs...)...)
	if err != nil {
		return nil, err
	}
	kept := out[:0]
	for _, c := range out {
		if c.Books == 0 && viewer != nil && !viewer.CanManage() {
			continue
		}
		c.Covers = s.collectionCovers(c.ID, viewer)
		kept = append(kept, c)
	}
	return kept, nil
}

func (s *Store) collectionCovers(id int64, viewer *User) []int64 {
	vis, args := visibilityClause(viewer)
	ids := []int64{}
	_ = s.DB.Select(&ids, `SELECT b.id FROM collection_books cb JOIN books b ON b.id = cb.book_id
		WHERE cb.collection_id = ?`+vis+` ORDER BY cb.added_at, b.id LIMIT 3`, append([]any{id}, args...)...)
	return ids
}

// CollectionByID returns a collection if viewer may see it.
func (s *Store) CollectionByID(id int64, viewer *User) (*Collection, error) {
	vis, vargs := visibilityClause(viewer)
	scope, sargs := collectionScope(viewer)
	var c Collection
	err := s.DB.Get(&c, `SELECT `+collectionCols+`, (SELECT COUNT(*) FROM collection_books cb JOIN books b ON b.id = cb.book_id
		WHERE cb.collection_id = c.id`+vis+`) AS books FROM collections c WHERE c.id = ? AND `+scope,
		append(append(vargs, id), sargs...)...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// NewCollection is what's needed to make one.
type NewCollection struct {
	Name, Icon, Description, Kind, Theme, Season, By string
}

// CreateCollection saves a new collection and returns its id.
func (s *Store) CreateCollection(n NewCollection) (int64, error) {
	if n.Icon == "" {
		n.Icon = "📚"
	}
	if n.Kind == "" {
		n.Kind = "manual"
	}
	res, err := s.DB.Exec(`INSERT INTO collections (name, icon, description, kind, theme, season, created_by) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(n.Name), n.Icon, strings.TrimSpace(n.Description), n.Kind, n.Theme, n.Season, n.By)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateCollection renames a collection or changes its icon or description.
func (s *Store) UpdateCollection(id int64, name, icon, description string) error {
	_, err := s.DB.Exec(`UPDATE collections SET name = ?, icon = ?, description = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		strings.TrimSpace(name), icon, strings.TrimSpace(description), id)
	return err
}

// KeepIdea turns an AI proposal into an ordinary AI collection.
func (s *Store) KeepIdea(id int64) error {
	_, err := s.DB.Exec(`UPDATE collections SET kind = 'ai', updated_at = CURRENT_TIMESTAMP WHERE id = ? AND kind = 'idea'`, id)
	return err
}

// DeleteCollection removes a collection (its books stay).
func (s *Store) DeleteCollection(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM collections WHERE id = ?`, id)
	return err
}

// AddToCollection adds books with the AI's reasons (reasons may be nil) and
// says how many were new.
func (s *Store) AddToCollection(id int64, bookIDs []int64, reasons map[int64]string) (int, error) {
	n := 0
	for _, b := range bookIDs {
		res, err := s.DB.Exec(`INSERT OR IGNORE INTO collection_books (collection_id, book_id, reason) VALUES (?, ?, ?)`, id, b, reasons[b])
		if err != nil {
			return n, err
		}
		if k, _ := res.RowsAffected(); k > 0 {
			n++
		}
	}
	if n > 0 {
		_, _ = s.DB.Exec(`UPDATE collections SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	}
	return n, nil
}

// RemoveFromCollection takes a book out of a collection.
func (s *Store) RemoveFromCollection(id, bookID int64) error {
	_, err := s.DB.Exec(`DELETE FROM collection_books WHERE collection_id = ? AND book_id = ?`, id, bookID)
	return err
}

// CollectionBookIDs lists every book in a collection (for "find more" to skip).
func (s *Store) CollectionBookIDs(id int64) []int64 {
	ids := []int64{}
	_ = s.DB.Select(&ids, `SELECT book_id FROM collection_books WHERE collection_id = ?`, id)
	return ids
}

// BookCollections lists the collections a book is in that viewer may see.
func (s *Store) BookCollections(bookID int64, viewer *User) []Collection {
	scope, args := collectionScope(viewer)
	out := []Collection{}
	_ = s.DB.Select(&out, `SELECT `+collectionCols+`, 0 AS books FROM collections c
		JOIN collection_books cb ON cb.collection_id = c.id WHERE cb.book_id = ? AND `+scope+` ORDER BY c.name COLLATE NOCASE`,
		append([]any{bookID}, args...)...)
	return out
}

// SeasonCollection is the collection a parent built for a seasonal shelf
// (0 if none has books): the shelf then shows it instead of keyword matches.
func (s *Store) SeasonCollection(key string) int64 {
	var id int64
	_ = s.DB.Get(&id, `SELECT c.id FROM collections c WHERE c.season = ? AND c.kind != 'idea'
		AND EXISTS (SELECT 1 FROM collection_books cb WHERE cb.collection_id = c.id) ORDER BY c.updated_at DESC LIMIT 1`, key)
	return id
}

// CollectionNames lists every collection's name, for the AI not to repeat one.
func (s *Store) CollectionNames() []string {
	names := []string{}
	_ = s.DB.Select(&names, `SELECT name FROM collections`)
	return names
}

// PendingIdeas counts AI proposals waiting for a parent.
func (s *Store) PendingIdeas() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM collections WHERE kind = 'idea'`)
	return n
}

func (s *Store) userCollections(userID int64) ([]int64, error) {
	ids := []int64{}
	err := s.DB.Select(&ids, `SELECT collection_id FROM user_collections WHERE user_id = ? ORDER BY collection_id`, userID)
	return ids, err
}

// setUserCollections replaces the collections a kid may be limited to.
func (s *Store) setUserCollections(userID int64, ids []int64) error {
	if _, err := s.DB.Exec(`DELETE FROM user_collections WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := s.DB.Exec(`INSERT OR IGNORE INTO user_collections (user_id, collection_id)
			SELECT ?, id FROM collections WHERE id = ? AND kind != 'idea'`, userID, id); err != nil {
			return err
		}
	}
	return nil
}
