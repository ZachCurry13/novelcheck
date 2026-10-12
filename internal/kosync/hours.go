package kosync

import (
	"database/sql"
	"fmt"
	"strings"
)

// StatHour is one book's reading in one hour (hour starts, Unix seconds in
// UTC): the page-turn rows of KOReader's statistics rolled up, so a reader's
// calendar and timeline can be drawn in their own time zone.
type StatHour struct {
	Title, Authors, MD5 string
	Hour                int64
	Seconds             int64
	Pages               int // different pages seen
}

// maxPageSeconds is the longest one page can count (KOReader ignores long
// pauses itself; a forgotten open book should not be a day of reading).
const maxPageSeconds = 600

// ReadHours reads the per-hour reading of a statistics.sqlite3 file.
func ReadHours(path string) ([]StatHour, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT COALESCE(b.title, ''), COALESCE(b.authors, ''), COALESCE(b.md5, ''),
		(p.start_time / 3600) * 3600, SUM(MIN(p.duration, ?)), COUNT(DISTINCT p.page)
		FROM page_stat_data p JOIN book b ON b.id = p.id_book
		WHERE p.start_time > 0 AND p.duration > 0
		GROUP BY b.id, (p.start_time / 3600)`, maxPageSeconds)
	if err != nil {
		return nil, fmt.Errorf("not a KOReader statistics file: %w", err)
	}
	defer rows.Close()
	var out []StatHour
	for rows.Next() {
		var h StatHour
		if err := rows.Scan(&h.Title, &h.Authors, &h.MD5, &h.Hour, &h.Seconds, &h.Pages); err != nil {
			return nil, err
		}
		h.Title = strings.TrimSpace(h.Title)
		if h.Title == "" {
			continue
		}
		h.Authors, _, _ = strings.Cut(strings.TrimSpace(h.Authors), "\n")
		h.MD5 = strings.ToLower(strings.TrimSpace(h.MD5))
		out = append(out, h)
	}
	return out, rows.Err()
}
