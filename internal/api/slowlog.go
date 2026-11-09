package api

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	numSeg = regexp.MustCompile(`/\d+`)
	hexSeg = regexp.MustCompile(`/[0-9a-f]{20,}`)
)

// shape turns /api/books/123/cover into /api/books/{id}/cover, so requests to the same page group.
func shape(path string) string {
	return hexSeg.ReplaceAllString(numSeg.ReplaceAllString(path, "/{id}"), "/{token}")
}

// slowSection writes the report's slow-request part: the pages that were slow most
// (how often, the average and the worst time) and the newest few.
func (s *Server) slowSection(line func(string, ...any)) {
	slow := s.Store.RecentSlow(300)
	line("\n== Slow requests (over 0.8 s; downloads and AI calls left out) ==")
	if len(slow) == 0 {
		line("(none)")
		return
	}
	type agg struct {
		key        string
		n, sum, mx int
	}
	by := map[string]*agg{}
	for _, e := range slow {
		ms, _ := strconv.Atoi(strings.TrimSuffix(e.Detail, " ms"))
		k := e.Method + " " + shape(e.Path)
		a := by[k]
		if a == nil {
			a = &agg{key: k}
			by[k] = a
		}
		a.n++
		a.sum += ms
		a.mx = max(a.mx, ms)
	}
	list := make([]*agg, 0, len(by))
	for _, a := range by {
		list = append(list, a)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].sum > list[j].sum })
	line("By page (total time first):")
	for _, a := range list[:min(len(list), 12)] {
		line("  %s: %d times, average %d ms, worst %d ms", a.key, a.n, a.sum/a.n, a.mx)
	}
	line("Newest:")
	for _, e := range slow[:min(len(slow), 12)] {
		line("  %s UTC %s %s -> %d, %s", e.At, e.Method, e.Path, e.Status, e.Detail)
	}
}
