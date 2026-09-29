package kosync

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // KOReader's statistics file is a SQLite database
)

// KOReader's "Reading statistics" plugin keeps statistics.sqlite3: a row per
// book opened on the device (title, authors, the same partial MD5 as
// progress sync, reading time, last opened) and a row per page turn. Its
// Cloud sync merges the file across devices through a WebDAV folder, which
// NovelCheck can be.

// StatBook is one book from KOReader's statistics.
type StatBook struct {
	Title       string
	Authors     string // the first author
	MD5         string
	Pages       int
	LastPage    int     // the page of the newest page turn
	Percent     float64 // 0..1, from the newest page turn
	ReadSeconds int64
	LastOpen    time.Time
}

// ReadStats reads a statistics.sqlite3 file.
func ReadStats(path string) ([]StatBook, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT COALESCE(b.title, ''), COALESCE(b.authors, ''), COALESCE(b.md5, ''), COALESCE(b.pages, 0),
		COALESCE(b.total_read_time, 0), COALESCE(b.last_open, 0),
		COALESCE((SELECT p.page FROM page_stat_data p WHERE p.id_book = b.id ORDER BY p.start_time DESC LIMIT 1), 0),
		COALESCE((SELECT p.total_pages FROM page_stat_data p WHERE p.id_book = b.id ORDER BY p.start_time DESC LIMIT 1), 0)
		FROM book b`)
	if err != nil {
		return nil, fmt.Errorf("not a KOReader statistics file: %w", err)
	}
	defer rows.Close()
	var out []StatBook
	for rows.Next() {
		var b StatBook
		var lastOpen int64
		var total int
		if err := rows.Scan(&b.Title, &b.Authors, &b.MD5, &b.Pages, &b.ReadSeconds, &lastOpen, &b.LastPage, &total); err != nil {
			return nil, err
		}
		b.Title = strings.TrimSpace(b.Title)
		if b.Title == "" {
			continue
		}
		b.Authors, _, _ = strings.Cut(strings.TrimSpace(b.Authors), "\n") // KOReader puts one author a line
		b.MD5 = strings.ToLower(strings.TrimSpace(b.MD5))
		if total <= 0 {
			total = b.Pages
		}
		if total > 0 {
			b.Percent = min(1, float64(b.LastPage)/float64(total))
		}
		if lastOpen > 0 {
			b.LastOpen = time.Unix(lastOpen, 0).UTC()
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
