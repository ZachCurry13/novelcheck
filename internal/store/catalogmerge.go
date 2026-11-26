package store

import (
	"errors"
	"strings"
)

// CatalogPair is two libraries of one person that look like the same thing: both hold their Kindle books.
type CatalogPair struct {
	Owner     string `json:"owner"`
	OwnerID   int64  `json:"owner_id"`
	Into      int64  `json:"into"`      // the bigger one, which stays
	IntoName  string `json:"into_name"` // its name now
	IntoBooks int    `json:"into_books"`
	From      int64  `json:"from"` // the smaller one, which joins it
	FromName  string `json:"from_name"`
	FromBooks int    `json:"from_books"`
	NewName   string `json:"new_name"` // what the joined library will be called
}

// KindleLibraryPairs finds people who have two or more private libraries with "Kindle" in the name (an Amazon import, the Kindle
// app's store books) and suggests joining each smaller one into the biggest.
func (s *Store) KindleLibraryPairs() []CatalogPair {
	var rows []struct {
		ID      int64  `db:"id"`
		Name    string `db:"name"`
		OwnerID int64  `db:"owner_id"`
		Owner   string `db:"owner"`
		Books   int    `db:"books"`
	}
	_ = s.DB.Select(&rows, `SELECT c.id, c.name, c.owner_id, u.username AS owner,
		(SELECT COUNT(DISTINCT book_id) FROM catalog_books cb WHERE cb.catalog_id = c.id) AS books
		FROM catalogs c JOIN users u ON u.id = c.owner_id
		WHERE c.private = 1 AND c.physical = 0 AND c.source != 'calibre' AND LOWER(c.name) LIKE '%kindle%' ORDER BY c.owner_id, books DESC, c.id`)
	out := []CatalogPair{}
	var best int // index in rows of the biggest for the current owner
	for i, r := range rows {
		if i == 0 || r.OwnerID != rows[best].OwnerID {
			best = i
			continue
		}
		b := rows[best]
		out = append(out, CatalogPair{Owner: r.Owner, OwnerID: r.OwnerID, Into: b.ID, IntoName: b.Name, IntoBooks: b.Books,
			From: r.ID, FromName: r.Name, FromBooks: r.Books, NewName: r.Owner + "'s Kindle"})
	}
	return out
}

// MergeCatalogs joins library from into library into (same owner, neither Calibre), renaming the result. Every copy moves; a copy
// the other library already has is kept once. The books themselves, their ratings and everyone's queues are untouched.
func (s *Store) MergeCatalogs(from, into int64, newName string) error {
	newName = strings.TrimSpace(newName)
	if from == into || newName == "" || len(newName) > 80 {
		return errors.New("pick two different libraries and a name")
	}
	a, err := s.CatalogByID(from)
	if err != nil {
		return err
	}
	b, err := s.CatalogByID(into)
	if err != nil {
		return err
	}
	if a.Source == "calibre" || b.Source == "calibre" || a.OwnerID == nil || b.OwnerID == nil || *a.OwnerID != *b.OwnerID {
		return errors.New("only two libraries of the same person can be joined")
	}
	if taken, err := s.CatalogByName(newName); err == nil && taken.ID != into && taken.ID != from {
		return errors.New("a library already has that name")
	}
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`UPDATE OR IGNORE catalog_books SET catalog_id = ? WHERE catalog_id = ?`,
		`DELETE FROM catalog_books WHERE catalog_id = ?`,
		`UPDATE OR IGNORE catalog_views SET catalog_id = ? WHERE catalog_id = ?`,
		`UPDATE users SET claim_catalog = ? WHERE claim_catalog = ?`,
	} {
		args := []any{into, from}
		if strings.HasPrefix(q, "DELETE") {
			args = []any{from}
		}
		if _, err := tx.Exec(q, args...); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM catalogs WHERE id = ?`, from); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE catalogs SET name = ? WHERE id = ?`, newName, into); err != nil {
		return err
	}
	return tx.Commit()
}
