package store

// The evidence that two entries are one book: same author, same ISBN, same place in a
// series, descriptions that say the same thing, and how the titles differ. A clear
// contradiction (different numbers in the same series, descriptions about different
// things) means the pair is not offered at all.

import (
	"fmt"
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
func judge(a, b *sameRow, kind string) *judged {
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
	seriesSame := false
	if a.SeriesIndex > 0 && b.SeriesIndex > 0 {
		if a.SeriesIndex != b.SeriesIndex && !isbn {
			return nil // book 1 and book 5 of a series are different books
		}
		seriesSame = true
		good(fmt.Sprintf("Same place in the series (#%g)", a.SeriesIndex))
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
	case isbn || kind == "exact":
		conf = "sure"
	case descSame || seriesSame:
		conf = "likely"
	}
	return &judged{a: a, b: b, conf: conf, reasons: rs}
}
