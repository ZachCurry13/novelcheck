package store

import (
	"database/sql"
	"errors"
	"strings"
)

const bookCols = `b.id, b.norm_key, b.title, b.author, b.isbn, b.description, b.blurb, b.status,
	b.classification, b.nudity, b.solo_acts, b.heavy_innuendo, b.playful_fantasy, b.dark_occult,
	b.demonic_presence, b.lgbtq_content, b.summary_verdict, b.approved, b.approved_by, b.age_level, b.age_set_by, b.analysis_model, b.analysis_error,
	b.analyzed_at, b.created_at, b.updated_at`

// derivedCols adds each book's file formats and number of Calibre entries.
const derivedCols = `, COALESCE((SELECT GROUP_CONCAT(f, ',') FROM (SELECT DISTINCT UPPER(fc.format) AS f
		FROM catalog_books fc WHERE fc.book_id = b.id AND fc.format NOT IN ('', 'list') ORDER BY f)), '') AS formats,
	(SELECT COUNT(DISTINCT dc.external_id) FROM catalog_books dc JOIN catalogs dcat ON dcat.id = dc.catalog_id
		AND dcat.source = 'calibre' WHERE dc.book_id = b.id) AS calibre_copies`

// UpsertBook inserts a book or returns the existing one with the same NormKey,
// filling in any metadata the stored row is missing. Returns the book id.
func (s *Store) UpsertBook(title, author, isbn, description string) (int64, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return 0, errors.New("title required")
	}
	key := NormKey(title, author)
	_, err := s.DB.Exec(`INSERT INTO books (norm_key, title, author, isbn, description)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(norm_key) DO UPDATE SET
			author = CASE WHEN books.author = '' THEN excluded.author ELSE books.author END,
			isbn = CASE WHEN books.isbn = '' THEN excluded.isbn ELSE books.isbn END,
			description = CASE WHEN books.description = '' THEN excluded.description ELSE books.description END,
			updated_at = CURRENT_TIMESTAMP`,
		key, title, strings.TrimSpace(author), strings.TrimSpace(isbn), strings.TrimSpace(description))
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.DB.Get(&id, `SELECT id FROM books WHERE norm_key = ?`, key)
	return id, err
}

// AddCopy links a book to a catalog location (idempotent).
func (s *Store) AddCopy(catalogID, bookID int64, path, format, externalID string) error {
	_, err := s.DB.Exec(`INSERT INTO catalog_books (catalog_id, book_id, path, format, external_id)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
		catalogID, bookID, path, strings.ToLower(format), externalID)
	return err
}

// PruneCatalog drops catalog links whose external ids are no longer present
// (e.g. books deleted from Calibre), then deletes orphaned books.
func (s *Store) PruneCatalog(catalogID int64, keepExternalIDs map[string]bool) (int, error) {
	var ids []string
	if err := s.DB.Select(&ids, `SELECT DISTINCT external_id FROM catalog_books WHERE catalog_id = ?`, catalogID); err != nil {
		return 0, err
	}
	removed := 0
	for _, id := range ids {
		if keepExternalIDs[id] {
			continue
		}
		if _, err := s.DB.Exec(`DELETE FROM catalog_books WHERE catalog_id = ? AND external_id = ?`, catalogID, id); err != nil {
			return removed, err
		}
		removed++
	}
	_, err := s.DB.Exec(`DELETE FROM books WHERE id NOT IN (SELECT book_id FROM catalog_books)`)
	return removed, err
}

// BookByID returns a book, or ErrNotFound if it doesn't exist or is hidden by
// the viewer's content rules (viewer may be nil for internal callers).
func (s *Store) BookByID(id int64, viewer *User) (*Book, error) {
	where, args := visibilityClause(viewer)
	q := `SELECT ` + bookCols + derivedCols + `, '' AS catalogs FROM books b WHERE b.id = ?` + where
	var b Book
	err := s.DB.Get(&b, q, append([]any{id}, args...)...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &b, err
}

func (s *Store) BookCopies(bookID int64) ([]BookCopy, error) {
	var cs []BookCopy
	err := s.DB.Select(&cs, `SELECT cb.catalog_id, c.name AS catalog_name, c.source, cb.path,
		cb.format, cb.external_id FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
		WHERE cb.book_id = ? ORDER BY c.name`, bookID)
	return cs, err
}

// SetApproved marks a book "OK" (shown despite filters and content rules) or
// clears the mark. by records which parent did it.
func (s *Store) SetApproved(id int64, approved bool, by string) error {
	if !approved {
		by = ""
	}
	_, err := s.DB.Exec(`UPDATE books SET approved = ?, approved_by = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, approved, by, id)
	return err
}

func (s *Store) SetBlurb(id int64, blurb string) error {
	_, err := s.DB.Exec(`UPDATE books SET blurb = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, blurb, id)
	return err
}

func (s *Store) SetStatus(id int64, status, errMsg string) error {
	_, err := s.DB.Exec(`UPDATE books SET status = ?, analysis_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, status, errMsg, id)
	return err
}

// SaveAnalysis persists an LLM verdict and marks the book analyzed.
func (s *Store) SaveAnalysis(id int64, a Analysis) error {
	_, err := s.DB.Exec(`UPDATE books SET status = 'analyzed', classification = ?, nudity = ?,
		solo_acts = ?, heavy_innuendo = ?, playful_fantasy = ?, dark_occult = ?, demonic_presence = ?,
		lgbtq_content = ?, summary_verdict = ?, analysis_model = ?, analysis_error = '',
		analyzed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		a.Classification, a.Nudity, a.SoloActs, a.HeavyInnuendo, a.PlayfulFantasy, a.DarkOccult,
		a.DemonicPresence, a.LGBTQContent, a.SummaryVerdict, a.Model, id)
	return err
}

// QueueForAnalysis flips up to limit pending books to 'queued' and returns their ids.
func (s *Store) QueueForAnalysis(limit int) ([]int64, error) {
	var ids []int64
	if err := s.DB.Select(&ids, `SELECT id FROM books WHERE status = 'pending'
		ORDER BY id LIMIT ?`, limit); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if err := s.SetStatus(id, "queued", ""); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

// ResetQueued returns queued/processing books to pending (wipe analysis queue).
func (s *Store) ResetQueued() (int64, error) {
	res, err := s.DB.Exec(`UPDATE books SET status = 'pending' WHERE status IN ('queued', 'processing')`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// StatusCounts returns the number of books per analysis status.
func (s *Store) StatusCounts() (map[string]int, error) {
	var rows []struct {
		Status string `db:"status"`
		N      int    `db:"n"`
	}
	if err := s.DB.Select(&rows, `SELECT status, COUNT(*) AS n FROM books GROUP BY status`); err != nil {
		return nil, err
	}
	out := map[string]int{"pending": 0, "queued": 0, "processing": 0, "analyzed": 0, "error": 0}
	for _, r := range rows {
		out[r.Status] = r.N
	}
	return out, nil
}
