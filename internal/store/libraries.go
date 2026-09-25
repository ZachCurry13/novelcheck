package store

// Libraries (catalogs) and who owns them: whoever imports a library owns
// it and can make it private, so only they and the admins see its books.

import (
	"database/sql"
	"errors"
)

const catalogCols = `c.id, c.name, c.source, c.created_at, c.owner_id, COALESCE(u.username, '') AS owner, c.private`

// ListCatalogs returns the libraries viewer may see (nil = all), with how
// many books each holds.
func (s *Store) ListCatalogs(viewer *User) ([]Catalog, error) {
	all := viewer == nil || viewer.Role == RoleAdmin
	var id int64
	if viewer != nil {
		id = viewer.ID
	}
	cs := []Catalog{}
	err := s.DB.Select(&cs, `SELECT `+catalogCols+`,
		(SELECT COUNT(DISTINCT book_id) FROM catalog_books cb WHERE cb.catalog_id = c.id) AS book_count
		FROM catalogs c LEFT JOIN users u ON u.id = c.owner_id
		WHERE ? OR c.private = 0 OR c.owner_id = ? ORDER BY c.name`, all, id)
	return cs, err
}

func (s *Store) CatalogByID(id int64) (*Catalog, error) {
	var c Catalog
	err := s.DB.Get(&c, `SELECT `+catalogCols+`, 0 AS book_count FROM catalogs c LEFT JOIN users u ON u.id = c.owner_id
		WHERE c.id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// CatalogByName finds a library by name (any capitalisation).
func (s *Store) CatalogByName(name string) (*Catalog, error) {
	var id int64
	if err := s.DB.Get(&id, `SELECT id FROM catalogs WHERE name = ?`, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.CatalogByID(id)
}

// CreateOwnedCatalog adds a library that ownerID imported.
func (s *Store) CreateOwnedCatalog(name, source string, ownerID int64, private bool) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO catalogs (name, source, owner_id, private) VALUES (?, ?, ?, ?)`, name, source, ownerID, private)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CanEdit: the owner and admins may change a library; editors may
// change the family's libraries (no owner). Nobody edits the Calibre one here.
func (c *Catalog) CanEdit(u *User) bool {
	if c.Source == "calibre" {
		return false
	}
	return u.Role == RoleAdmin || (c.OwnerID != nil && *c.OwnerID == u.ID) || (c.OwnerID == nil && u.CanManage())
}

// SetCatalogPrivate makes a library private or shared.
func (s *Store) SetCatalogPrivate(id int64, private bool) error {
	_, err := s.DB.Exec(`UPDATE catalogs SET private = ? WHERE id = ? AND source != 'calibre'`, private, id)
	return err
}

// SetCatalogOwner gives a library to someone (0 = the whole family, which
// also makes it shared).
func (s *Store) SetCatalogOwner(id, ownerID int64) error {
	var owner any
	if ownerID > 0 {
		owner = ownerID
	}
	_, err := s.DB.Exec(`UPDATE catalogs SET owner_id = ?, private = CASE WHEN ? IS NULL THEN 0 ELSE private END
		WHERE id = ? AND source != 'calibre'`, owner, owner, id)
	return err
}

// RemoveFromCatalog takes a book out of one library (NovelCheck's list
// only; nothing is deleted from a device or Calibre). A book left in no
// library is removed with it. It reports whether the book was there.
func (s *Store) RemoveFromCatalog(catalogID, bookID int64) (bool, error) {
	res, err := s.DB.Exec(`DELETE FROM catalog_books WHERE catalog_id = ? AND book_id = ?`, catalogID, bookID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if _, err := s.DB.Exec(`DELETE FROM books WHERE id = ? AND id NOT IN (SELECT book_id FROM catalog_books)`, bookID); err != nil {
		return false, err
	}
	return n > 0, nil
}
