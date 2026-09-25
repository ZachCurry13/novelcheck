// Package suggest picks books from the family's library for one reader:
// the next book in a series they're reading, more by authors they read, and
// books whose descriptions are like theirs, while a 👎 pushes similar books
// down. The matching is free and instant; depending on the admin's setting
// the AI then picks the best of this shortlist (see service.go).
package suggest

import (
	"cmp"
	"slices"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Pick is a suggested library book and why.
type Pick struct {
	Book   store.SuggestBook
	Score  float64
	Reason string // "Next in Discworld after Mort", "More by …", "Like …"
}

const (
	minScore  = 1.0 // weaker matches aren't worth showing
	perSeries = 1   // only the next unread book of a series
	perAuthor = 3
	ruledOut  = 1000 // a 👎 said "not this author / series / this spicy"
)

// Rank scores pool against the reader's signals and returns the best n.
// family counts how many other people read each book; it's what suggestions
// fall back on while the reader has nothing in Up Next yet.
func Rank(pool []store.SuggestBook, signals []store.SuggestSignal, family map[int64]int, n int) []Pick {
	docs := make([]string, 0, len(pool)+len(signals))
	for _, b := range pool {
		docs = append(docs, text(b))
	}
	for _, s := range signals {
		docs = append(docs, text(s.SuggestBook))
	}
	c := newCorpus(docs)
	sigVecs := make([]vector, len(signals))
	liked := false
	for i, s := range signals {
		sigVecs[i] = c.vector(text(s.SuggestBook))
		liked = liked || s.Weight > 0
	}

	var picks []Pick
	for i, b := range pool {
		v := c.vector(docs[i])
		total, best, reason := 0.0, 0.0, ""
		for j, s := range signals {
			if s.Weight < 0 {
				total -= dislike(b, v, s, sigVecs[j])
				continue
			}
			m, why := match(b, v, s.SuggestBook, sigVecs[j])
			if m == 0 {
				continue
			}
			w := m * s.Weight / 3
			total += w
			if w > best {
				best, reason = w, why
			}
		}
		if f := family[b.ID]; f > 0 {
			if !liked {
				total, reason = float64(f)+minScore, "Popular in your family"
			} else if total > 0 {
				total += 0.3 * float64(f)
			}
		}
		if total < minScore {
			continue
		}
		if reason == "" {
			reason = "Matches what you read"
		}
		picks = append(picks, Pick{b, total, reason})
	}
	slices.SortFunc(picks, func(a, b Pick) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(a.Book.SeriesIndex, b.Book.SeriesIndex), strings.Compare(a.Book.Title, b.Book.Title))
	})
	return diverse(picks, n)
}

// match is how strongly book b (words v) resembles signal s (words sv).
func match(b store.SuggestBook, v vector, s store.SuggestBook, sv vector) (float64, string) {
	m, why := 0.0, ""
	if sk := seriesKey(b.Series); sk != "" && sk == seriesKey(s.Series) {
		switch d := b.SeriesIndex - s.SeriesIndex; {
		case d > 0 && d <= 1:
			m, why = 10, "Next in "+b.Series+" after "+s.Title
		case d > 1:
			m, why = 1.5, "Later in "+b.Series
		default:
			m, why = 2, "Also in "+b.Series
		}
	}
	if a := authorKey(b.Author); a != "" && a == authorKey(s.Author) {
		m += 3
		if why == "" {
			why = "More by " + firstAuthor(b.Author)
		}
	}
	if sim := cosine(v, sv); sim > 0.06 {
		m += 8 * sim
		if why == "" {
			why = "Like " + s.Title
		}
	}
	if m > 0 && b.Spice >= 0 && s.Spice >= 0 && b.Spice-s.Spice <= 1 && s.Spice-b.Spice <= 1 {
		m += 0.5 // a similar pepper level
	}
	return m, why
}

// dislike is how much a 👎 on s counts against book b, by the reason given
// in "Why not?": the author, series or pepper level rule those books out;
// "not my kind of story" drops books described alike. Without a reason it's
// a light nudge, so one 👎 doesn't outweigh a series someone is reading.
func dislike(b store.SuggestBook, v vector, s store.SuggestSignal, sv vector) float64 {
	switch s.Reason {
	case "author":
		if a := authorKey(b.Author); a != "" && a == authorKey(s.Author) {
			return ruledOut
		}
	case "series":
		if k := seriesKey(b.Series); k != "" && k == seriesKey(s.Series) {
			return ruledOut
		}
	case "story": // the themes: books described alike
		return 15 * cosine(v, sv)
	case "spicy":
		if b.Spice >= 0 && s.Spice >= 0 && b.Spice >= s.Spice {
			return ruledOut
		}
	case "", "notwant", "disliked": // weights -1 and -2
		m, _ := match(b, v, s.SuggestBook, sv)
		return m * -s.Weight / 3
	}
	return 0
}

// diverse keeps the best n, with at most perSeries books from one series
// and perAuthor from one author.
func diverse(picks []Pick, n int) []Pick {
	series, authors := map[string]int{}, map[string]int{}
	out := make([]Pick, 0, n)
	for _, p := range picks {
		sk, ak := seriesKey(p.Book.Series), authorKey(p.Book.Author)
		if (sk != "" && series[sk] >= perSeries) || (ak != "" && authors[ak] >= perAuthor) {
			continue
		}
		series[sk]++
		authors[ak]++
		if out = append(out, p); len(out) == n {
			break
		}
	}
	return out
}

func text(b store.SuggestBook) string { return b.Title + " " + b.Series + " " + b.Description }

func seriesKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// authorKey is the first author's surname, as NovelCheck matches authors.
func authorKey(a string) string { return strings.TrimPrefix(store.NormKey("", a), "|") }

func firstAuthor(a string) string {
	first, _, _ := strings.Cut(a, " & ")
	return strings.TrimSpace(first)
}
