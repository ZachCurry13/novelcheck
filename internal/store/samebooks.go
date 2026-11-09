package store

// "Same book, listed twice": a book looked up or wished for (no file) and the
// same book arriving later from Calibre or Audiobookshelf can end up as two
// entries when the title or author is written a little differently ("Cinder"
// and "Cinder (The Lunar Chronicles #1)" by "Marissa Meyer" and "Meyer,
// Marissa"). This finds such pairs. A pair where one is in a library and the
// other isn't is merged by itself; the rest are shown for a person to decide.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/titles"
)

// SameBook is one side of a possible pair.
type SameBook struct {
	ID      int64  `db:"id" json:"id"`
	Title   string `db:"title" json:"title"`
	Author  string `db:"author" json:"author"`
	Owned   bool   `db:"owned" json:"owned"`
	Formats string `db:"formats" json:"formats"`
	Wanted  int    `db:"wanted" json:"wanted"` // people with it on a wishlist or Nightstand
}

// SamePair is two entries that look like one book.
type SamePair struct {
	A SameBook `json:"a"`
	B SameBook `json:"b"`
}

type sameRow struct {
	SameBook
	last       string
	haveAuthor bool
	bases      []string
}

func (s *Store) sameRows() ([]*sameRow, error) {
	var rows []SameBook
	if err := s.DB.Select(&rows, `SELECT b.id, b.title, b.author, `+ownedCond+` AS owned,
		COALESCE((SELECT GROUP_CONCAT(f, ',') FROM (SELECT DISTINCT UPPER(fc.format) AS f FROM catalog_books fc
			WHERE fc.book_id = b.id AND fc.format NOT IN ('', 'list') ORDER BY f)), '') AS formats,
		(SELECT COUNT(*) FROM wishlist w WHERE w.book_id = b.id AND w.status IN ('wanted', 'approved'))
			+ (SELECT COUNT(*) FROM queue_items q WHERE q.book_id = b.id) AS wanted
		FROM books b WHERE NOT `+discoverOnlyCond+` ORDER BY b.id`); err != nil {
		return nil, err
	}
	out := make([]*sameRow, 0, len(rows))
	for _, r := range rows {
		sr := &sameRow{SameBook: r, last: squash(lastName(r.Author))}
		sr.haveAuthor = !junkAuthors[strings.ToLower(strings.TrimSpace(r.Author))] && sr.last != ""
		for _, v := range titleVariants(titles.Clean(r.Title), sr.haveAuthor) {
			if base := squash(dropArticle(strings.TrimSpace(v))); len(base) >= 4 {
				sr.bases = append(sr.bases, base)
			}
		}
		out = append(out, sr)
	}
	return out, nil
}

// likelyPairs finds the pairs, not counting ones a person said are different.
func (s *Store) likelyPairs() ([]*[2]*sameRow, error) {
	rows, err := s.sameRows()
	if err != nil {
		return nil, err
	}
	ignored := map[[2]int64]bool{}
	var ig []struct {
		A int64 `db:"a_id"`
		B int64 `db:"b_id"`
	}
	_ = s.DB.Select(&ig, `SELECT a_id, b_id FROM dup_ignores`)
	for _, i := range ig {
		ignored[[2]int64{i.A, i.B}] = true
	}
	byBase := map[string][]*sameRow{}
	for _, r := range rows {
		for _, b := range r.bases {
			byBase[b] = append(byBase[b], r)
		}
	}
	seen := map[[2]int64]bool{}
	var out []*[2]*sameRow
	for _, r := range rows {
		for _, base := range r.bases {
			for _, o := range byBase[base] {
				if o.ID <= r.ID || (r.haveAuthor && o.haveAuthor && r.last != o.last) {
					continue
				}
				k := [2]int64{r.ID, o.ID}
				if seen[k] || ignored[k] {
					continue
				}
				seen[k] = true
				out = append(out, &[2]*sameRow{r, o})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0].ID < out[j][0].ID })
	return out, nil
}

// SamePairs lists the pairs for a person to look at, looking like the same book.
func (s *Store) SamePairs() ([]SamePair, error) {
	ps, err := s.likelyPairs()
	if err != nil {
		return nil, err
	}
	out := []SamePair{}
	for _, p := range ps {
		out = append(out, SamePair{A: p[0].SameBook, B: p[1].SameBook})
	}
	return out, nil
}

// IgnoreSame remembers that two entries are different books.
func (s *Store) IgnoreSame(a, b int64) error {
	if a > b {
		a, b = b, a
	}
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO dup_ignores (a_id, b_id) VALUES (?, ?)`, a, b)
	return err
}

// AutoMergeSame merges the clear cases: an entry that is in no library (only
// looked up, wished for or on a list) with the one library book it matches,
// when that match is the only one. It returns how many were merged.
func (s *Store) AutoMergeSame() int {
	ps, err := s.likelyPairs()
	if err != nil {
		return 0
	}
	owned := map[int64][]int64{} // an unowned entry -> the owned books it matches
	for _, p := range ps {
		a, b := p[0], p[1]
		if a.Owned != b.Owned {
			if a.Owned {
				a, b = b, a
			}
			owned[a.ID] = append(owned[a.ID], b.ID)
		}
	}
	merged := 0
	for loose, matches := range owned {
		if len(matches) != 1 {
			continue
		}
		if _, err := s.MergeBooks(matches[0], loose); err == nil {
			merged++
		}
	}
	return merged
}

// TidySame merges the clear duplicates after a sync and tells the parents.
func (s *Store) TidySame() {
	if n := s.AutoMergeSame(); n > 0 {
		s.NotifyRoutine("same-merged", fmt.Sprintf("NovelCheck joined %d book%s that were listed twice (for example one added to a reading list before it was in the library).",
			n, map[bool]string{true: "", false: "s"}[n == 1]), "#/duplicates")
	}
}
