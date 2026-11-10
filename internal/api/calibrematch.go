package api

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A Content server can serve several libraries, and its default is not always the one NovelCheck reads (the
// book numbers then don't line up and Tidy finds "no such book"). matchServerLibrary asks each library the
// server offers about a sample of the books NovelCheck knows by number, and picks the one whose titles agree.
// want maps a Calibre book number to the titles NovelCheck knows it by (tidy and as read). It returns the winning library id
// ("" if none matches) and a line per library saying how many of the sample matched.
func (s *Server) matchServerLibrary(ctx context.Context, want map[int][]string) (string, []string) {
	c := s.calibreClient()
	libs, _, err := c.Libraries(ctx)
	if err != nil || len(libs) == 0 || len(want) == 0 {
		return "", nil
	}
	ids := make([]int, 0, len(want))
	for id := range want {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if len(ids) > 30 {
		ids = ids[:30]
	}
	keys := make([]string, 0, len(libs))
	for k := range libs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	best, bestN := "", 0
	var report []string
	for _, k := range keys {
		c.Library = k
		have, err := c.Titles(ctx, ids)
		if err != nil {
			report = append(report, fmt.Sprintf("“%s”: couldn't be asked (%v)", libs[k], err))
			continue
		}
		n := 0
		for _, id := range ids {
			if t, ok := have[id]; ok && agrees(t, want[id]) {
				n++
			}
		}
		report = append(report, fmt.Sprintf("“%s”: %d of %d sample books match (%d numbers exist)", libs[k], n, len(ids), len(have)))
		if n > bestN {
			best, bestN = k, n
		}
	}
	if bestN*2 < len(ids) && bestN < 1 { // nothing agrees
		return "", report
	}
	if len(ids) > 1 && bestN*2 < len(ids) { // only a few agree: not clear enough to switch on
		return "", report
	}
	return best, report
}

// useServerLibrary remembers which library the Content server should use.
func (s *Server) useServerLibrary(id string) {
	_ = s.Store.SetSetting(store.KeyCalibreSrvLibrary, id)
}

func joinLines(l []string) string { return strings.Join(l, "; ") }

func agrees(title string, known []string) bool {
	for _, k := range known {
		if k != "" && store.NormKey(title, "") == store.NormKey(k, "") {
			return true
		}
	}
	return false
}
