package api

import (
	"fmt"
	"regexp"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The same failure often repeats dozens of times (a crashing scanner, a request that fails every minute). The bug
// link shows each different one once, with how many times it happened and the first and last time.

type errGroup struct {
	store.ServerError
	Count int
	First string // oldest time of the repeats
}

var errNumbers = regexp.MustCompile(`\d+`)

// groupErrors folds errors (newest first) that say the same thing; digits are ignored when comparing.
func groupErrors(errs []store.ServerError, max int) []errGroup {
	var out []errGroup
	at := map[string]int{}
	for _, e := range errs {
		key := fmt.Sprint(e.Method, e.Path, e.Status, errNumbers.ReplaceAllString(e.Detail, "#"))
		if i, ok := at[key]; ok {
			out[i].Count++
			out[i].First = e.At
			continue
		}
		if len(out) == max {
			continue
		}
		at[key] = len(out)
		out = append(out, errGroup{ServerError: e, Count: 1, First: e.At})
	}
	return out
}

func (g errGroup) times() string {
	if g.Count == 1 {
		return ""
	}
	return fmt.Sprintf(" [x%d, first %s UTC]", g.Count, g.First)
}
