package store

import (
	"errors"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/seasons"
)

// "Not for this shelf": books a parent takes off a seasonal shelf or a
// collection leave it at once and are remembered, so the shelf's words and
// the AI (Build again, Find more) never bring them back.

// SeasonShelf and CollectionShelf name a shelf in shelf_rejects.
func SeasonShelf(key string) string   { return "season:" + key }
func CollectionShelf(id int64) string { return "collection:" + strconv.FormatInt(id, 10) }
func (s *Store) collectionSeason(id int64) string {
	var key string
	_ = s.DB.Get(&key, `SELECT season FROM collections WHERE id = ?`, id)
	return key
}

// shelvesOf is the collection a shelf shows (a seasonal shelf shows the
// collection built for it, if any) and the names its rejections go under.
func (s *Store) shelvesOf(collectionID int64, season string) (int64, []string) {
	var names []string
	if collectionID == 0 && season != "" {
		if _, ok := seasons.Find(season); ok {
			names = append(names, SeasonShelf(season))
			collectionID = s.SeasonCollection(season)
		}
	}
	if collectionID > 0 {
		names = append(names, CollectionShelf(collectionID))
		if key := s.collectionSeason(collectionID); key != "" && key != season {
			names = append(names, SeasonShelf(key))
		}
	}
	return collectionID, names
}

// RejectFromShelf takes books off a collection (collectionID) or a seasonal
// shelf (season) for good, and says how many there were.
func (s *Store) RejectFromShelf(collectionID int64, season string, ids []int64, by string) (int, error) {
	collectionID, names := s.shelvesOf(collectionID, season)
	if len(names) == 0 {
		return 0, errors.New("no such shelf")
	}
	tx, err := s.DB.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	n := 0
	for _, id := range ids {
		for _, name := range names {
			res, err := tx.Exec(`INSERT OR IGNORE INTO shelf_rejects (shelf, book_id, rejected_by) SELECT ?, id, ? FROM books WHERE id = ?`, name, by, id)
			if err != nil {
				return 0, err
			}
			if key, ok := strings.CutPrefix(name, "season:"); ok {
				if _, err := tx.Exec(`DELETE FROM season_books WHERE season = ? AND book_id = ?`, key, id); err != nil {
					return 0, err
				}
			}
			if k, _ := res.RowsAffected(); k > 0 && name == names[0] {
				n++
			}
		}
		if collectionID > 0 {
			if _, err := tx.Exec(`DELETE FROM collection_books WHERE collection_id = ? AND book_id = ?`, collectionID, id); err != nil {
				return 0, err
			}
		}
	}
	return n, tx.Commit()
}

// ShelfRejects lists the books taken off a collection or seasonal shelf, for
// the AI to skip.
func (s *Store) ShelfRejects(collectionID int64, season string) []int64 {
	_, names := s.shelvesOf(collectionID, season)
	ids := []int64{}
	if len(names) == 0 {
		return ids
	}
	q := `SELECT DISTINCT book_id FROM shelf_rejects WHERE shelf IN (?` + strings.Repeat(", ?", len(names)-1) + `)`
	args := make([]any, len(names))
	for i, n := range names {
		args[i] = n
	}
	_ = s.DB.Select(&ids, q, args...)
	return ids
}

// OccultIDs lists the books flagged Dark Occult / Demonic that no parent
// marked OK: faith shelves leave them out, so the AI doesn't pick them either.
func (s *Store) OccultIDs() []int64 {
	ids := []int64{}
	_ = s.DB.Select(&ids, `SELECT id FROM books WHERE (dark_occult = 1 OR demonic_presence = 1) AND approved = 0`)
	return ids
}

// ShelfToCheck is a shelf's theme and books (up to limit) for the AI to
// check: a collection's, or a seasonal shelf's (its collection's when a
// parent built one).
func (s *Store) ShelfToCheck(collectionID int64, season string, limit int) (theme string, books []Book, err error) {
	books = []Book{}
	collectionID, names := s.shelvesOf(collectionID, season)
	if len(names) == 0 {
		return "", nil, ErrNotFound
	}
	if collectionID > 0 {
		var c struct {
			Name        string `db:"name"`
			Description string `db:"description"`
			Theme       string `db:"theme"`
		}
		if err := s.DB.Get(&c, `SELECT name, description, theme FROM collections WHERE id = ?`, collectionID); err != nil {
			return "", nil, ErrNotFound
		}
		theme = c.Theme
		if theme == "" {
			theme = strings.TrimSpace(c.Name + ". " + c.Description)
		}
		err = s.DB.Select(&books, `SELECT `+bookCols+derivedCols+`, '' AS catalogs FROM books b
			JOIN collection_books cb ON cb.book_id = b.id AND cb.collection_id = ? ORDER BY b.title COLLATE NOCASE LIMIT ?`, collectionID, limit)
		return theme, books, err
	}
	se, _ := seasons.Find(season)
	if err := s.seasonReady(se); err != nil {
		return "", nil, err
	}
	cond := ownedCond
	if se.Faith { // the shelf never shows these, so don't check them
		cond += " AND (b.approved = 1 OR NOT (b.dark_occult = 1 OR b.demonic_presence = 1))"
	}
	err = s.DB.Select(&books, `SELECT `+bookCols+derivedCols+`, '' AS catalogs FROM books b
		JOIN season_books sn ON sn.book_id = b.id AND sn.season = ? WHERE `+cond+` ORDER BY b.title COLLATE NOCASE LIMIT ?`, se.Key, limit)
	return se.Theme, books, err
}
