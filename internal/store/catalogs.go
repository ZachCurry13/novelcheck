package store

import (
	"database/sql"
	"errors"
	"strings"
	"unicode"
)

const CalibreCatalogName = "Calibre Main"

func (s *Store) ListCatalogs() ([]Catalog, error) {
	var cs []Catalog
	err := s.DB.Select(&cs, `SELECT c.id, c.name, c.source, c.created_at,
		(SELECT COUNT(DISTINCT book_id) FROM catalog_books cb WHERE cb.catalog_id = c.id) AS book_count
		FROM catalogs c ORDER BY c.name`)
	return cs, err
}

func (s *Store) CatalogByID(id int64) (*Catalog, error) {
	var c Catalog
	err := s.DB.Get(&c, `SELECT id, name, source, created_at, 0 AS book_count FROM catalogs WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// EnsureCatalog returns the catalog with the given name, creating it if needed.
func (s *Store) EnsureCatalog(name, source string) (int64, error) {
	if _, err := s.DB.Exec(`INSERT INTO catalogs (name, source) VALUES (?, ?)
		ON CONFLICT(name) DO NOTHING`, name, source); err != nil {
		return 0, err
	}
	var id int64
	err := s.DB.Get(&id, `SELECT id FROM catalogs WHERE name = ?`, name)
	return id, err
}

func (s *Store) RenameCatalog(id int64, name string) error {
	_, err := s.DB.Exec(`UPDATE catalogs SET name = ? WHERE id = ? AND source != 'calibre'`, name, id)
	return err
}

// ClearCatalogCopies unlinks every copy in the named catalog but keeps the
// book rows (and their analyses); the next sync re-links what still exists
// and prunes the rest.
func (s *Store) ClearCatalogCopies(name string) error {
	_, err := s.DB.Exec(`DELETE FROM catalog_books WHERE catalog_id =
		(SELECT id FROM catalogs WHERE name = ?)`, name)
	return err
}

// DeleteCatalog removes a catalog and then any books no longer in any catalog.
func (s *Store) DeleteCatalog(id int64) error {
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM catalogs WHERE id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM books WHERE id NOT IN (SELECT book_id FROM catalog_books)`); err != nil {
		return err
	}
	return tx.Commit()
}

// NormKey builds the de-duplication key used to match the same title across
// catalogs (e.g. a Calibre copy and a Kindle copy): lowercase alphanumerics
// of the title plus the author's last name.
func NormKey(title, author string) string {
	return squash(title) + "|" + squash(lastName(author))
}

func squash(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// lastName handles "First Last", "Last, First" and multi-author "A & B" forms.
func lastName(author string) string {
	a := strings.TrimSpace(author)
	for _, sep := range []string{" & ", ";", " and "} {
		if i := strings.Index(a, sep); i > 0 {
			a = a[:i]
		}
	}
	if i := strings.Index(a, ","); i > 0 {
		return strings.TrimSpace(a[:i])
	}
	f := strings.Fields(a)
	if len(f) == 0 {
		return ""
	}
	return f[len(f)-1]
}
