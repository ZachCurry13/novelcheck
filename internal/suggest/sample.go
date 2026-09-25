package suggest

// Books for the taste profile: a varied handful from the library for
// someone to mark (want to read, read & liked, …). First books of a series,
// one per author, spread over pepper levels, favouring books the family
// has read, shuffled so "20 more" brings new ones.

import (
	"cmp"
	"math/rand/v2"
	"slices"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Sample picks up to n books from pool, leaving out exclude (books already
// on screen).
func Sample(pool []store.SuggestBook, family map[int64]int, exclude map[int64]bool, n int) []store.SuggestBook {
	type scored struct {
		b store.SuggestBook
		s float64
	}
	var cands []scored
	for _, b := range pool {
		if !exclude[b.ID] {
			s := 2*float64(min(family[b.ID], 3)) + 2*rand.Float64()
			if b.Spice >= 0 {
				s++ // rated books say more about someone's taste
			}
			cands = append(cands, scored{b, s})
		}
	}
	slices.SortFunc(cands, func(a, b scored) int { return cmp.Compare(b.s, a.s) })

	out := make([]store.SuggestBook, 0, n)
	taken := map[int64]bool{}
	// First pass keeps it varied; later passes fill up a small library.
	for _, strict := range []bool{true, false} {
		authors, levels := map[string]int{}, map[int]int{}
		for _, b := range out {
			authors[authorKey(b.Author)]++
			levels[b.Spice]++
		}
		for _, c := range cands {
			b := c.b
			if len(out) == n {
				return out
			}
			if taken[b.ID] {
				continue
			}
			ak := authorKey(b.Author)
			if strict && (b.SeriesIndex > 1 || (ak != "" && authors[ak] > 0) || levels[b.Spice] >= (n+3)/4) {
				continue
			}
			taken[b.ID] = true
			authors[ak]++
			levels[b.Spice]++
			out = append(out, b)
		}
	}
	return out
}
