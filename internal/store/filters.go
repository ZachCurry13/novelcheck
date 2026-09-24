package store

import "strings"

// BookFilter describes a dashboard search across all catalogs.
type BookFilter struct {
	Query          string   // matches title or author
	CatalogID      int64    // only books in this catalog
	OverlapWith    int64    // ...that are also in this catalog
	MultiCatalog   bool     // only books present in 2+ catalogs
	Classification string   // "Closed Door" | "Open Door" | "No Spice" | "Pending"
	Flags          []string // books that HAVE all of these flags
	ExcludeFlags   []string // books that have NONE of these flags
	Status         string
	Sort           string // "title" (default) | "author" | "recent"
	Limit, Offset  int
}

// flagColumns whitelists filterable flag names -> SQL predicates.
var flagColumns = map[string]string{
	"nudity":           "b.nudity = 1",
	"solo_acts":        "b.solo_acts = 1",
	"heavy_innuendo":   "b.heavy_innuendo = 1",
	"lgbtq":            "b.lgbtq_content = 1",
	"dark_occult":      "(b.dark_occult = 1 OR b.demonic_presence = 1)",
	"playful_fantasy":  "b.playful_fantasy = 1",
	"demonic_presence": "b.demonic_presence = 1",
}

var classifications = map[string]bool{"Closed Door": true, "Open Door": true, "No Spice": true}

// ValidClassification reports whether c is one of the three spice levels.
func ValidClassification(c string) bool { return classifications[c] }

// visibilityClause enforces a user's profile content rules at the SQL level so
// hidden titles can never be listed, searched, fetched or queued.
func visibilityClause(u *User) (string, []any) {
	if u == nil {
		return "", nil
	}
	var parts []string
	if u.HideUnrated {
		parts = append(parts, "b.status = 'analyzed'")
	}
	if u.HideOpenDoor {
		parts = append(parts, "COALESCE(b.classification, '') != 'Open Door'")
	}
	if u.HideNudity {
		parts = append(parts, "b.nudity = 0")
	}
	if u.HideSoloActs {
		parts = append(parts, "b.solo_acts = 0")
	}
	if u.HideInnuendo {
		parts = append(parts, "b.heavy_innuendo = 0")
	}
	if u.HideDarkOccult {
		parts = append(parts, "b.dark_occult = 0 AND b.demonic_presence = 0")
	}
	if u.HideLGBTQ {
		parts = append(parts, "b.lgbtq_content = 0")
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " AND (" + strings.Join(parts, " AND ") + ")", nil
}

// ListBooks returns books matching f that viewer is allowed to see, plus the
// total match count for pagination.
func (s *Store) ListBooks(f BookFilter, viewer *User) ([]Book, int, error) {
	where := []string{"1=1"}
	var args []any
	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, "(b.title LIKE ? OR b.author LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like)
	}
	if f.CatalogID > 0 {
		where = append(where, "EXISTS (SELECT 1 FROM catalog_books x WHERE x.book_id = b.id AND x.catalog_id = ?)")
		args = append(args, f.CatalogID)
	}
	if f.OverlapWith > 0 {
		where = append(where, "EXISTS (SELECT 1 FROM catalog_books y WHERE y.book_id = b.id AND y.catalog_id = ?)")
		args = append(args, f.OverlapWith)
	}
	if f.MultiCatalog {
		where = append(where, "(SELECT COUNT(DISTINCT z.catalog_id) FROM catalog_books z WHERE z.book_id = b.id) > 1")
	}
	switch {
	case f.Classification == "Pending":
		where = append(where, "b.classification IS NULL")
	case classifications[f.Classification]:
		where = append(where, "b.classification = ?")
		args = append(args, f.Classification)
	}
	for _, fl := range f.Flags {
		if p, ok := flagColumns[fl]; ok {
			where = append(where, p)
		}
	}
	for _, fl := range f.ExcludeFlags {
		if p, ok := flagColumns[fl]; ok {
			where = append(where, "NOT "+p)
		}
	}
	if f.Status != "" {
		where = append(where, "b.status = ?")
		args = append(args, f.Status)
	}
	vis, vargs := visibilityClause(viewer)
	cond := strings.Join(where, " AND ") + vis
	args = append(args, vargs...)

	var total int
	if err := s.DB.Get(&total, `SELECT COUNT(*) FROM books b WHERE `+cond, args...); err != nil {
		return nil, 0, err
	}
	order := "b.title COLLATE NOCASE"
	switch f.Sort {
	case "author":
		order = "b.author COLLATE NOCASE, b.title COLLATE NOCASE"
	case "recent":
		order = "b.created_at DESC, b.id DESC"
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	q := `SELECT ` + bookCols + `, COALESCE((SELECT GROUP_CONCAT(name, ', ') FROM (
			SELECT DISTINCT c.name FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
			WHERE cb.book_id = b.id)), '') AS catalogs
		FROM books b WHERE ` + cond + ` ORDER BY ` + order + ` LIMIT ? OFFSET ?`
	var books []Book
	err := s.DB.Select(&books, q, append(args, limit, max(f.Offset, 0))...)
	return books, total, err
}
