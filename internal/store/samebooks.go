package store

// "Same book, listed twice": a book looked up or wished for (no file) and the
// same book arriving later from Calibre or Audiobookshelf can end up as two
// entries when the title or author is written a little differently ("Cinder"
// and "Cinder (The Lunar Chronicles #1)" by "Marissa Meyer" and "Meyer,
// Marissa"). This finds such pairs and weighs the evidence (samesignals.go):
// only the clearest are merged by themselves, the rest are shown with their
// reasons for a person to decide.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/titles"
)

// SameBook is one side of a possible pair.
type SameBook struct {
	ID          int64   `db:"id" json:"id"`
	Title       string  `db:"title" json:"title"`
	Author      string  `db:"author" json:"author"`
	Owned       bool    `db:"owned" json:"owned"`
	Formats     string  `db:"formats" json:"formats"`
	Wanted      int     `db:"wanted" json:"wanted"` // people with it on a wishlist or Nightstand
	Series      string  `db:"series" json:"series"`
	SeriesIndex float64 `db:"series_index" json:"series_index"`
	ISBN        string  `db:"isbn" json:"isbn"`
	Description string  `db:"description" json:"-"`
	Snippet     string  `db:"-" json:"snippet"`
}

// Reason is one piece of evidence for or against a pair being one book.
type Reason struct {
	Text string `json:"text"`
	Good bool   `json:"good"`
}

// SamePair is two entries that may be one book, with how sure that is
// (sure, likely or check) and why.
type SamePair struct {
	A          SameBook `json:"a"`
	B          SameBook `json:"b"`
	Confidence string   `json:"confidence"`
	Reasons    []Reason `json:"reasons"`
}

type sameRow struct {
	SameBook
	last       string
	haveAuthor bool
	full       string   // the whole title, boiled down
	generic    []string // the title, and without an alternate or generic subtitle ("Or, ..." / "A Novel")
	prefixes   []string // the part before any subtitle
}

var subtitleSeps = []string{": ", " - ", " – ", " — "}

func boil(v string) string { return squash(dropArticle(strings.TrimSpace(v))) }

func (s *Store) sameRows() ([]*sameRow, error) {
	var rows []SameBook
	if err := s.DB.Select(&rows, `SELECT b.id, b.title, b.author, `+ownedCond+` AS owned,
		COALESCE((SELECT GROUP_CONCAT(f, ',') FROM (SELECT DISTINCT UPPER(fc.format) AS f FROM catalog_books fc
			WHERE fc.book_id = b.id AND fc.format NOT IN ('', 'list') ORDER BY f)), '') AS formats,
		(SELECT COUNT(*) FROM wishlist w WHERE w.book_id = b.id AND w.status IN ('wanted', 'approved'))
			+ (SELECT COUNT(*) FROM queue_items q WHERE q.book_id = b.id) AS wanted,
		b.series, b.series_index, b.isbn, b.description
		FROM books b WHERE NOT `+discoverOnlyCond+` ORDER BY b.id`); err != nil {
		return nil, err
	}
	out := make([]*sameRow, 0, len(rows))
	for _, r := range rows {
		r.Snippet = r.Description
		if len(r.Snippet) > 180 {
			r.Snippet = r.Snippet[:180] + "…"
		}
		sr := &sameRow{SameBook: r, last: squash(lastName(r.Author))}
		sr.haveAuthor = !junkAuthors[strings.ToLower(strings.TrimSpace(r.Author))] && sr.last != ""
		clean := titles.Clean(r.Title)
		sr.full = boil(clean)
		for _, v := range sameVariants(clean) {
			if b := boil(v); len(b) >= 4 {
				sr.generic = append(sr.generic, b)
			}
		}
		for _, sep := range subtitleSeps {
			if a, _, ok := strings.Cut(clean, sep); ok {
				if b := boil(a); len(b) >= 4 {
					sr.prefixes = append(sr.prefixes, b)
				}
			}
		}
		out = append(out, sr)
	}
	return out, nil
}

// genericSubtitles are subtitles that only decorate a title ("A Novel"); with an alternate
// title ("Or, Disappearance!") they still name the same book. Any other subtitle may name a
// different book ("I Am Number Four: The Lost Files: Six's Legacy" is not "I Am Number Four"),
// so those pairs are only offered, with evidence, never merged by themselves.
var genericSubtitles = map[string]bool{"novel": true, "a novel": true, "a memoir": true, "a thriller": true, "a mystery": true,
	"a story": true, "stories": true, "a love story": true, "a tale": true, "an autobiography": true}

func genericSubtitle(sub string) bool {
	l := strings.ToLower(strings.TrimSpace(sub))
	return strings.HasPrefix(l, "or,") || strings.HasPrefix(l, "or ") || genericSubtitles[strings.Trim(l, " .!")]
}

// sameVariants are the titles an entry could be called when deciding two entries are one
// book: the whole title, and the part before a generic or alternate subtitle.
func sameVariants(clean string) []string {
	out := []string{clean}
	for _, sep := range subtitleSeps {
		if a, b, ok := strings.Cut(clean, sep); ok && genericSubtitle(b) {
			out = append(out, a)
		}
	}
	return out
}

// likelyPairs finds the pairs worth a look, judged, not counting ones a person said are different.
func (s *Store) likelyPairs() ([]*judged, error) {
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
	byGeneric, byFull := map[string][]*sameRow{}, map[string][]*sameRow{}
	for _, r := range rows {
		for _, g := range r.generic {
			byGeneric[g] = append(byGeneric[g], r)
		}
		byFull[r.full] = append(byFull[r.full], r)
	}
	seen := map[[2]int64]bool{}
	var out []*judged
	add := func(x, y *sameRow, kind string) {
		if x.ID > y.ID {
			x, y = y, x
		}
		k := [2]int64{x.ID, y.ID}
		if x.ID == y.ID || seen[k] || ignored[k] || (x.haveAuthor && y.haveAuthor && x.last != y.last) {
			return
		}
		seen[k] = true
		if j := judge(x, y, kind); j != nil {
			out = append(out, j)
		}
	}
	for _, r := range rows {
		for _, g := range r.generic {
			for _, o := range byGeneric[g] {
				add(r, o, "exact")
			}
		}
	}
	for _, r := range rows { // "Title" next to "Title: Something": maybe one book, maybe a spin-off
		for _, p := range r.prefixes {
			for _, o := range byFull[p] {
				add(r, o, "subtitle")
			}
		}
	}
	rank := map[string]int{"sure": 0, "likely": 1, "check": 2}
	sort.SliceStable(out, func(i, j int) bool {
		if rank[out[i].conf] != rank[out[j].conf] {
			return rank[out[i].conf] < rank[out[j].conf]
		}
		return out[i].a.ID < out[j].a.ID
	})
	return out, nil
}

// SamePairs lists the pairs for a person to look at, the surest first, each with its evidence.
func (s *Store) SamePairs() ([]SamePair, error) {
	ps, err := s.likelyPairs()
	if err != nil {
		return nil, err
	}
	out := []SamePair{}
	for _, p := range ps {
		out = append(out, SamePair{A: p.a.SameBook, B: p.b.SameBook, Confidence: p.conf, Reasons: p.reasons})
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

// AutoMergeSame merges only the pairs judged "sure" (the same ISBN, or the same title once tidied
// by the same or an unknown author): an entry that is in no library (only looked up, wished for
// or on a list) with the one library book it matches, when that match is the only one. It
// returns what it joined, as "title → title".
func (s *Store) AutoMergeSame() []string {
	ps, err := s.likelyPairs()
	if err != nil {
		return nil
	}
	owned := map[int64][]*sameRow{} // an unowned entry -> the owned books it surely matches
	loose := map[int64]*sameRow{}
	for _, p := range ps {
		a, b := p.a, p.b
		if p.conf == "sure" && a.Owned != b.Owned {
			if a.Owned {
				a, b = b, a
			}
			owned[a.ID] = append(owned[a.ID], b)
			loose[a.ID] = a
		}
	}
	var merged []string
	for id, matches := range owned {
		if len(matches) != 1 {
			continue
		}
		if _, err := s.MergeBooks(matches[0].ID, id); err == nil {
			merged = append(merged, "“"+loose[id].Title+"” → “"+matches[0].Title+"”")
		}
	}
	sort.Strings(merged)
	return merged
}

// TidySame merges the clear duplicates after a sync and tells the parents which books.
func (s *Store) TidySame() {
	if m := s.AutoMergeSame(); len(m) > 0 {
		list := strings.Join(m[:min(len(m), 5)], "; ")
		if len(m) > 5 {
			list += fmt.Sprintf("; and %d more", len(m)-5)
		}
		s.NotifyRoutine("same-merged", fmt.Sprintf("NovelCheck joined %d book%s that were listed twice: %s.", len(m), map[bool]string{true: "", false: "s"}[len(m) == 1], list), "#/duplicates")
	}
}
