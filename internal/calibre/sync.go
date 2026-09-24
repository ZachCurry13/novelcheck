// Package calibre imports a Calibre library by reading metadata.db in strict
// read-only mode. It never writes to the Calibre database.
package calibre

import (
	"fmt"
	"html"
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
	ID          int64  `db:"id"`
	Title       string `db:"title"`
	Path        string `db:"path"`
	Authors     string `db:"authors"`
	ISBN        string `db:"isbn"`
	Description string `db:"description"`
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

	var books []calibreBook
	err = cdb.Select(&books, `SELECT b.id, b.title, b.path,
		COALESCE((SELECT GROUP_CONCAT(name, ' & ') FROM (SELECT a.name FROM books_authors_link l
			JOIN authors a ON a.id = l.author WHERE l.book = b.id ORDER BY l.id)), '') AS authors,
		COALESCE(NULLIF(b.isbn, ''), (SELECT val FROM identifiers i WHERE i.book = b.id
			AND i.type = 'isbn' LIMIT 1), '') AS isbn,
		COALESCE((SELECT text FROM comments c WHERE c.book = b.id), '') AS description
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
	keep := map[string]bool{}
	for _, b := range books {
		id, err := st.UpsertBook(b.Title, b.Authors, b.ISBN, StripHTML(b.Description))
		if err != nil {
			continue // skip malformed rows (e.g. empty title) without aborting the sync
		}
		ext := strconv.FormatInt(b.ID, 10)
		keep[ext] = true
		res.Books++
		fs := byBook[b.ID]
		if len(fs) == 0 {
			if err := st.AddCopy(catID, id, "", "", ext); err != nil {
				return res, err
			}
			continue
		}
		for _, f := range fs {
			if err := st.AddCopy(catID, id, FilePath(dir, b.Path, f.Name, f.Format), f.Format, ext); err != nil {
				return res, err
			}
			res.Copies++
		}
	}
	res.Removed, err = st.PruneCatalog(catID, keep)
	return res, err
}

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
