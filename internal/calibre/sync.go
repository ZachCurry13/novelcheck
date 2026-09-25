// Package calibre imports a Calibre library by reading metadata.db in strict
// read-only mode. It never writes to the Calibre database.
package calibre

import (
	"fmt"
	"html"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Result summarises one sync run.
type Result struct {
	Books   int `json:"books"`
	Copies  int `json:"copies"`
	Removed int `json:"removed"`
}

type calibreBook struct {
	ID           int64   `db:"id"`
	Title        string  `db:"title"`
	Path         string  `db:"path"`
	Authors      string  `db:"authors"`
	ISBN         string  `db:"isbn"`
	Description  string  `db:"description"`
	LastModified string  `db:"last_modified"` // e.g. "2024-03-01 18:23:45.123456+00:00"
	Series       string  `db:"series"`
	SeriesIndex  float64 `db:"series_index"`
	Tags         string  `db:"tags"` // joined with the unit separator, char(31)
}

type calibreFile struct {
	BookID int64  `db:"book"`
	Format string `db:"format"`
	Name   string `db:"name"`
}

// OpenReadOnly opens <dir>/metadata.db with mode=ro so Calibre keeps its lock.
func OpenReadOnly(dir string) (*sqlx.DB, error) {
	dsn := "file:" + path.Join(dir, "metadata.db") + "?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(5000)"
	return sqlx.Open("sqlite", dsn)
}

// Sync reads every book from the Calibre library at dir and upserts it into
// the "Calibre Main" catalog. Books are left in Pending Analysis; no LLM
// calls are made here.
func Sync(st *store.Store, dir string) (Result, error) {
	var res Result
	cdb, err := OpenReadOnly(dir)
	if err != nil {
		return res, err
	}
	defer cdb.Close()

	// Calibre records each entry's last change; very old libraries may lack it.
	lastMod := "''"
	if hasColumn(cdb, "books", "last_modified") {
		lastMod = "COALESCE(b.last_modified, '')"
	}
	series := "'' AS series, 0 AS series_index"
	if hasColumn(cdb, "books", "series_index") && hasColumn(cdb, "books_series_link", "series") {
		series = `COALESCE((SELECT s.name FROM books_series_link sl JOIN series s ON s.id = sl.series
			WHERE sl.book = b.id LIMIT 1), '') AS series, COALESCE(b.series_index, 0) AS series_index`
	}
	tags := "'' AS tags"
	if hasColumn(cdb, "books_tags_link", "tag") {
		tags = `COALESCE((SELECT GROUP_CONCAT(t.name, char(31)) FROM books_tags_link tl JOIN tags t ON t.id = tl.tag
			WHERE tl.book = b.id), '') AS tags`
	}
	var books []calibreBook
	err = cdb.Select(&books, `SELECT b.id, b.title, b.path,
		COALESCE((SELECT GROUP_CONCAT(name, ' & ') FROM (SELECT a.name FROM books_authors_link l
			JOIN authors a ON a.id = l.author WHERE l.book = b.id ORDER BY l.id)), '') AS authors,
		COALESCE((SELECT val FROM identifiers i WHERE i.book = b.id
			AND i.type = 'isbn' LIMIT 1), '') AS isbn,
		COALESCE((SELECT text FROM comments c WHERE c.book = b.id), '') AS description,
		`+lastMod+` AS last_modified, `+series+`, `+tags+`
		FROM books b ORDER BY b.id`)
	if err != nil {
		return res, fmt.Errorf("read calibre books: %w", err)
	}
	var files []calibreFile
	if err := cdb.Select(&files, `SELECT book, format, name FROM data`); err != nil {
		return res, fmt.Errorf("read calibre formats: %w", err)
	}
	byBook := map[int64][]calibreFile{}
	for _, f := range files {
		byBook[f.BookID] = append(byBook[f.BookID], f)
	}

	catID, err := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	if err != nil {
		return res, err
	}
	// Older versions stored file-less entries with an empty path; drop those
	// so they're re-added with a placeholder below.
	if err := st.DropEmptyPaths(catID); err != nil {
		return res, err
	}
	keep := map[string]bool{}
	seen := map[store.CopyKey]bool{}
	for _, b := range books {
		ext := strconv.FormatInt(b.ID, 10)
		id, err := st.UpsertCalibreBook(catID, store.CalibreEntry{ExtID: ext, Title: b.Title, Authors: b.Authors,
			ISBN: b.ISBN, Description: StripHTML(b.Description), Series: b.Series, SeriesIndex: b.SeriesIndex, Tags: splitTags(b.Tags)})
		if err != nil {
			continue // skip malformed rows (e.g. empty title) without aborting the sync
		}
		keep[ext] = true
		res.Books++
		fs := byBook[b.ID]
		entryMod := changeTime(b.LastModified)
		if len(fs) == 0 {
			// No files: a placeholder path keeps each Calibre entry distinct, so
			// two empty entries of the same book still show as duplicates.
			seen[store.CopyKey{Book: id, Path: EntryPath(ext)}] = true
			if err := st.AddCalibreCopy(catID, id, EntryPath(ext), "", ext, entryMod); err != nil {
				return res, err
			}
			continue
		}
		for _, f := range fs {
			p := FilePath(dir, b.Path, f.Name, f.Format)
			seen[store.CopyKey{Book: id, Path: p}] = true
			if err := st.AddCalibreCopy(catID, id, p, f.Format, ext, latest(entryMod, fileTime(p))); err != nil {
				return res, err
			}
			res.Copies++
		}
	}
	if res.Removed, err = st.PruneCatalog(catID, keep); err != nil {
		return res, err
	}
	// Files Calibre moved (it renames folders when a title changes) and entries
	// now filed under another book.
	if err := st.PruneStaleCopies(catID, seen); err != nil {
		return res, err
	}
	return res, st.BaselineModified()
}

// hasColumn reports whether Calibre's database has table.column (older
// libraries lack some).
func hasColumn(cdb *sqlx.DB, table, column string) bool {
	var n int
	return cdb.Get(&n, `SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column) == nil && n > 0
}

// changeTime turns Calibre's "2024-03-01 18:23:45.123456+00:00" (UTC) into
// "2024-03-01 18:23:45", which sorts as text.
func changeTime(calibre string) string {
	t := strings.Replace(strings.TrimSpace(calibre), "T", " ", 1)
	if len(t) < 19 {
		return ""
	}
	return t[:19]
}

// fileTime is the file's modification time in the same form ("" if unknown).
func fileTime(p string) string {
	st, err := os.Stat(p)
	if err != nil {
		return ""
	}
	return st.ModTime().UTC().Format("2006-01-02 15:04:05")
}

func latest(a, b string) string {
	if b > a {
		return b
	}
	return a
}

// EntryPath stands in for the file path of a Calibre entry with no files.
func EntryPath(calibreID string) string { return "calibre-entry:" + calibreID }

// FilePath resolves a Calibre relative path to the container mount, e.g.
// ("/calibre", "Author/Title (1)", "Title - Author", "EPUB") ->
// "/calibre/Author/Title (1)/Title - Author.epub".
func FilePath(mount, rel, name, format string) string {
	return path.Join(mount, rel, name+"."+strings.ToLower(format))
}

var tagRE = regexp.MustCompile(`(?s)<[^>]*>`)
var spaceRE = regexp.MustCompile(`\s+`)

// StripHTML turns Calibre's HTML comments field into plain text.
func StripHTML(s string) string {
	s = strings.NewReplacer("</p>", "\n", "<br>", "\n", "<br/>", "\n", "<br />", "\n").Replace(s)
	s = html.UnescapeString(tagRE.ReplaceAllString(s, " "))
	return strings.TrimSpace(spaceRE.ReplaceAllString(s, " "))
}

func splitTags(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\x1f")
}
