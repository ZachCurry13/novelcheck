package store

// The evidence that two entries are one book: same author, same ISBN, same place in a
// series, descriptions that say the same thing, and how the titles differ. A clear
// contradiction (different numbers in the same series, descriptions about different
// things) means the pair is not offered at all.

import (
	"fmt"
	"os"
	"strings"
	"unicode"
)

type judged struct {
	a, b    *sameRow
	conf    string // sure | likely | check
	reasons []Reason
}

var descStop = map[string]bool{"that": true, "with": true, "this": true, "from": true, "their": true, "have": true, "been": true,
	"they": true, "will": true, "into": true, "when": true, "which": true, "about": true, "what": true, "after": true, "there": true,
	"would": true, "book": true, "novel": true, "story": true, "series": true, "more": true, "them": true, "than": true, "only": true}

func descWords(d string) map[string]bool {
	if len(d) > 800 {
		d = d[:800]
	}
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(d), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if len(w) >= 4 && !descStop[w] {
			out[w] = true
		}
	}
	return out
}

// descSimilarity is how much of the shorter description's wording the longer one shares (0 to 1).
func descSimilarity(a, b string) float64 {
	x, y := descWords(a), descWords(b)
	if len(x) == 0 || len(y) == 0 {
		return 0
	}
	if len(x) > len(y) {
		x, y = y, x
	}
	n := 0
	for w := range x {
		if y[w] {
			n++
		}
	}
	return float64(n) / float64(len(x))
}

func cleanISBN(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) || r == 'X' || r == 'x' {
			return unicode.ToUpper(r)
		}
		return -1
	}, s)
}

const minDesc = 40 // a description shorter than this says too little to compare

// judge weighs a pair. kind is "exact" (the same title once tidied, alternate or generic
// subtitle ignored) or "subtitle" (one title is the other plus a subtitle). It returns nil
// when the evidence says they are different books.
func (s *Store) judge(a, b *sameRow, kind string) *judged {
	var rs []Reason
	good := func(t string) { rs = append(rs, Reason{t, true}) }
	warn := func(t string) { rs = append(rs, Reason{t, false}) }

	switch {
	case a.haveAuthor && b.haveAuthor:
		good("Same author")
	case a.haveAuthor || b.haveAuthor:
		warn("One of them has no author listed")
	default:
		warn("Neither has an author listed")
	}
	isbn := false
	if x, y := cleanISBN(a.ISBN), cleanISBN(b.ISBN); len(x) >= 10 && x == y {
		isbn = true
		good("Same ISBN (" + x + ")")
	}
	seriesSame, different := false, false // different: something says these are two books
	sa, sb := boil(a.Series), boil(b.Series)
	namesDiffer := sa != "" && sb != "" && sa != sb
	if a.SeriesIndex > 0 && b.SeriesIndex > 0 {
		if a.SeriesIndex != b.SeriesIndex && !isbn {
			return nil // book 1 and book 5 of a series are different books
		}
		if namesDiffer && !isbn { // "Lorien Legacies #1" and "The Lost Files #1" are the first of two series
			different = true
			warn(fmt.Sprintf("Both are #%g, but of different series: “%s” and “%s”", a.SeriesIndex, a.Series, b.Series))
		} else {
			seriesSame = true
			good(fmt.Sprintf("Same place in the series (#%g)", a.SeriesIndex))
		}
	} else if namesDiffer && !isbn {
		different = true
		warn(fmt.Sprintf("Different series: “%s” and “%s”", a.Series, b.Series))
	}
	// A novel and its novella, or a book and a box set, are not one book.
	if ka, kb := shortForm(a), shortForm(b); ka != kb && !isbn {
		different = true
		if ka == "" {
			ka, kb = kb, ka
		}
		warn(fmt.Sprintf("One of them calls itself a %s and the other doesn't", ka))
	}
	// A much shorter file is a story, not the novel.
	if a.Size == 0 {
		a.Size = s.fileSize(a.ID)
	}
	if b.Size == 0 {
		b.Size = s.fileSize(b.ID)
	}
	if a.Size > 0 && b.Size > 0 {
		hi, lo := max(a.Size, b.Size), min(a.Size, b.Size)
		switch {
		case float64(hi) >= 2.5*float64(lo) && !isbn:
			different = true
			warn(fmt.Sprintf("Very different file sizes (%s and %s): one may be a novella or short story", humanSize(hi), humanSize(lo)))
		case float64(hi) <= 1.5*float64(lo):
			good(fmt.Sprintf("Similar file sizes (%s and %s)", humanSize(a.Size), humanSize(b.Size)))
		}
	}
	descSame := false
	da, db := len(strings.TrimSpace(a.Description)) >= minDesc, len(strings.TrimSpace(b.Description)) >= minDesc
	switch {
	case da && db:
		sim := descSimilarity(a.Description, b.Description)
		switch {
		case sim >= 0.5:
			descSame = true
			good(fmt.Sprintf("The descriptions say the same thing (%d%% shared wording)", int(sim*100)))
		case sim < 0.1 && !isbn:
			return nil // two blurbs about different things
		default:
			warn(fmt.Sprintf("The descriptions are only partly alike (%d%% shared wording)", int(sim*100)))
		}
	case da || db:
		warn("Only one of them has a description to compare")
	default:
		warn("Neither has a description to compare")
	}
	longer := a
	if len(b.Title) > len(a.Title) {
		longer = b
	}
	if kind == "exact" {
		good("Same title once tidied")
	} else {
		warn("One title has an extra part: “" + longer.Title + "”")
	}
	conf := "check"
	switch {
	case isbn:
		conf = "sure"
	case different:
		conf = "check" // something says they are two books: a person looks
	case kind == "exact":
		conf = "sure"
	case descSame || seriesSame:
		conf = "likely"
	}
	return &judged{a: a, b: b, conf: conf, reasons: rs}
}

// day is the date part of a Calibre or SQLite time ("2026-11-03T14:22:11+00:00" → "2026-11-03").
func day(t string) string {
	if len(t) >= 10 {
		return t[:10]
	}
	return t
}

// suggestKeep says which of two entries is best to keep as the book's face, and why: the one in a
// library, then the one whose file changed more recently in Calibre (a newer edition or re-issue),
// then the one more people use. It says nothing when none stands out.
func suggestKeep(a, b *sameRow) (string, string) {
	pick := func(aWins bool, why string) (string, string) {
		if aWins {
			return "a", why
		}
		return "b", why
	}
	if a.Owned != b.Owned {
		return pick(a.Owned, "it is in a library and the other isn't")
	}
	if am, bm := day(a.Modified), day(b.Modified); am != "" && bm != "" && am != bm {
		return pick(am > bm, fmt.Sprintf("its file was changed more recently in Calibre (%s, the other %s)", max(am, bm), min(am, bm)))
	}
	if au, bu := a.Readers+a.Wanted, b.Readers+b.Wanted; au != bu {
		return pick(au > bu, fmt.Sprintf("more people use it (%d, the other %d)", max(au, bu), min(au, bu)))
	}
	return "", ""
}

// shortForm is the word a book uses for itself when it is not a full novel (title or blurb).
func shortForm(r *sameRow) string {
	text := strings.ToLower(r.Title + " " + r.Description)
	for _, w := range []string{"novella", "novelette", "short story", "short stories", "omnibus", "box set", "boxed set", "collection", "bundle", "sampler"} {
		if strings.Contains(text, w) {
			return w
		}
	}
	return ""
}

func humanSize(b int64) string {
	if b >= 1<<20 {
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	}
	return fmt.Sprintf("%d KB", max(1, b>>10))
}

// fileSize is the size of a book's biggest ebook file (its EPUB when it has one), 0 if unknown.
func (s *Store) fileSize(id int64) int64 {
	var rows []struct {
		Path   string `db:"path"`
		Format string `db:"format"`
	}
	_ = s.DB.Select(&rows, `SELECT cb.path, LOWER(cb.format) AS format FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id AND c.source = 'calibre'
		WHERE cb.book_id = ? AND cb.format NOT IN ('', 'list', 'paper')`, id)
	var epub, any int64
	for _, r := range rows {
		if st, err := os.Stat(r.Path); err == nil {
			any = max(any, st.Size())
			if r.Format == "epub" {
				epub = max(epub, st.Size())
			}
		}
	}
	if epub > 0 {
		return epub
	}
	return any
}
