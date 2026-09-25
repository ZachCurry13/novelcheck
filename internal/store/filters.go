package store

import (
	"strconv"
	"strings"
)

// BookFilter describes a dashboard search across all catalogs.
type BookFilter struct {
	Query          string   // matches title or author
	CatalogID      int64    // only books in this catalog
	OverlapWith    int64    // ...that are also in this catalog
	MultiCatalog   bool     // only books present in 2+ catalogs
	DeepOnly       bool     // only books rated by a Deep Scan (full text)
	Classification string   // "Closed Door" | "Open Door" | "No Spice" | "Pending"
	Flags          []string // books that HAVE all of these flags
	ExcludeFlags   []string // hide books with ANY of these flags (parent-approved books stay)
	AnyFlags       []string // books with ANY of these flags (used for the Calibre removal list)
	Status         string
	Age            string // "" any | "unset" | "1".."5" = suitable up to that age group
	Spice          string // "" any | "0".."5" exact peppers | "old" = rated before the pepper scale
	Format         string // "" any | "epub" etc. | "multi" (2+ formats) | "none" (no file) | "dupes" (2+ Calibre entries)
	Author         string // part of an author's name
	Series         string // part of a series name
	Genre          string // a category key (package genres)
	Kind           string // "fiction", "nonfiction" or "unknown"
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
	"open_door":        "b.classification = 'Open Door'",
}

// fileFormats are the e-book formats the Format filter offers.
var fileFormats = map[string]bool{"epub": true, "azw3": true, "azw": true, "mobi": true, "kfx": true, "pdf": true}

var formatConds = map[string]string{
	"multi": "(SELECT COUNT(DISTINCT LOWER(fm.format)) FROM catalog_books fm WHERE fm.book_id = b.id AND fm.format NOT IN ('', 'list')) > 1",
	"none":  "NOT EXISTS (SELECT 1 FROM catalog_books fn WHERE fn.book_id = b.id AND fn.format NOT IN ('', 'list'))",
	"dupes": `(SELECT COUNT(DISTINCT dd.external_id) FROM catalog_books dd JOIN catalogs dk ON dk.id = dd.catalog_id
		AND dk.source = 'calibre' WHERE dd.book_id = b.id) > 1`,
}

var classifications = map[string]bool{"Closed Door": true, "Open Door": true, "No Spice": true}

// ValidClassification reports whether c is one of the three spice levels.
func ValidClassification(c string) bool { return classifications[c] }

// visibilityClause enforces a user's profile content rules at the SQL level so
// hidden titles can never be listed, searched, fetched or queued. Books a
// parent marked "OK" (approved) are exempt, e.g. Harry Potter's fantasy magic.
func visibilityClause(u *User) (string, []any) {
	if u == nil {
		return "", nil
	}
	var parts []string
	if u.HideUnrated {
		// A parent picking an age group counts as rating the book.
		parts = append(parts, "(b.status = 'analyzed' OR b.age_level > 0)")
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
	if u.MaxSpice >= 0 {
		// Unrated books are governed by HideUnrated, so they pass here.
		parts = append(parts, "COALESCE("+effectiveSpice+", 0) <= "+strconv.Itoa(u.MaxSpice))
	}
	clause := ""
	if len(parts) > 0 {
		clause = " AND (b.approved = 1 OR (" + strings.Join(parts, " AND ") + "))"
	}
	// An age group set on the book always applies to kids of a younger group,
	// even if a parent marked the book OK for its content.
	if u.Role == RoleRestricted && u.AgeLevel > 0 {
		clause += " AND (b.age_level = 0 OR b.age_level <= ?)"
		return clause, []any{u.AgeLevel}
	}
	return clause, nil
}

// filterCond turns f (plus the viewer's content rules) into a WHERE clause.
func filterCond(f BookFilter, viewer *User) (string, []any) {
	where := []string{"1=1"}
	var args []any
	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, "(b.title LIKE ? OR b.author LIKE ? OR b.series LIKE ? OR b.tags LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like, like)
	}
	if f.CatalogID > 0 {
		where = append(where, "EXISTS (SELECT 1 FROM catalog_books x WHERE x.book_id = b.id AND x.catalog_id = ?)")
		args = append(args, f.CatalogID)
	}
	if f.OverlapWith > 0 {
		where = append(where, "EXISTS (SELECT 1 FROM catalog_books y WHERE y.book_id = b.id AND y.catalog_id = ?)")
		args = append(args, f.OverlapWith)
	}
	if f.DeepOnly {
		where = append(where, "b.analysis_model LIKE 'deep:%'")
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
		if p, a, ok := flagPredicate(fl); ok {
			where = append(where, p)
			args = append(args, a...)
		}
	}
	for _, fl := range f.ExcludeFlags {
		if p, a, ok := flagPredicate(fl); ok {
			where = append(where, "(b.approved = 1 OR NOT COALESCE("+p+", 0))")
			args = append(args, a...)
		}
	}
	var anyOf []string
	for _, fl := range f.AnyFlags {
		if p, a, ok := flagPredicate(fl); ok {
			anyOf = append(anyOf, p)
			args = append(args, a...)
		}
	}
	if len(anyOf) > 0 {
		where = append(where, "b.approved = 0 AND ("+strings.Join(anyOf, " OR ")+")")
	}
	switch {
	case f.Age == "unset":
		where = append(where, "b.age_level = 0")
	case len(f.Age) == 1 && f.Age >= "1" && f.Age <= "5":
		where = append(where, "b.age_level BETWEEN 1 AND ?")
		args = append(args, int(f.Age[0]-'0'))
	}
	switch {
	case f.Spice == "old":
		where = append(where, "b.status = 'analyzed' AND b.spice_level IS NULL")
	case len(f.Spice) == 1 && f.Spice >= "0" && f.Spice <= "5":
		where = append(where, "b.spice_level = ?")
		args = append(args, int(f.Spice[0]-'0'))
	}
	if c, ok := formatConds[f.Format]; ok {
		where = append(where, c)
	} else if fileFormats[f.Format] {
		where = append(where, "EXISTS (SELECT 1 FROM catalog_books fx WHERE fx.book_id = b.id AND LOWER(fx.format) = ?)")
		args = append(args, f.Format)
	}
	if f.Status != "" {
		where = append(where, "b.status = ?")
		args = append(args, f.Status)
	}
	where, args = browseConds(f, where, args)
	vis, vargs := visibilityClause(viewer)
	return strings.Join(where, " AND ") + vis, append(args, vargs...)
}

// ListBooks returns books matching f that viewer is allowed to see, plus the
// total match count for pagination.
func (s *Store) ListBooks(f BookFilter, viewer *User) ([]Book, int, error) {
	cond, args := filterCond(f, viewer)
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
	q := `SELECT ` + bookCols + derivedCols + `, COALESCE((SELECT GROUP_CONCAT(name, ', ') FROM (
			SELECT DISTINCT c.name FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
			WHERE cb.book_id = b.id)), '') AS catalogs
		FROM books b WHERE ` + cond + ` ORDER BY ` + order + ` LIMIT ? OFFSET ?`
	var books []Book
	err := s.DB.Select(&books, q, append(args, limit, max(f.Offset, 0))...)
	return books, total, err
}

// CalibreMatch is a Calibre book that matches a removal filter.
type CalibreMatch struct {
	CalibreID string `db:"external_id" json:"calibre_id"`
	Title     string `db:"title" json:"title"`
	Author    string `db:"author" json:"author"`
}

// CalibreMatches lists Calibre books (by Calibre id) matching f, for the
// "remove from Calibre" helper. Parent-approved books are never included.
func (s *Store) CalibreMatches(f BookFilter) ([]CalibreMatch, error) {
	cond, args := filterCond(f, nil)
	var out []CalibreMatch
	err := s.DB.Select(&out, `SELECT cb.external_id, MIN(b.title) AS title, MIN(b.author) AS author
		FROM books b JOIN catalog_books cb ON cb.book_id = b.id
		JOIN catalogs c ON c.id = cb.catalog_id AND c.source = 'calibre'
		WHERE b.approved = 0 AND `+cond+`
		GROUP BY cb.external_id ORDER BY title COLLATE NOCASE`, args...)
	return out, err
}
