package api

import (
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/zachcurry13/novelcheck/internal/libby"
)

// A library's search answers with look-alikes and popular books when it has nothing for the words. relevance
// says how much of the query (as words) a result's title, author and series contain; numbers are a bonus.
func relevance(query string, f libby.Found) float64 {
	words := func(t string) []string {
		return strings.FieldsFunc(strings.ToLower(t), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	}
	have := words(f.Title + " " + f.Author + " " + f.Series)
	total, hit := 0.0, 0.0
	for _, w := range words(query) {
		isNum := unicode.IsDigit([]rune(w)[0])
		if !isNum {
			total++
		}
		for _, h := range have {
			if h == w || (len(w) >= 3 && strings.HasPrefix(h, w)) {
				if !isNum {
					hit++
				}
				break
			}
		}
	}
	if total == 0 {
		return 1
	}
	return hit / total
}

// rankFound keeps the results that match at least half the words (the best first); if none do, none are shown.
func rankFound(query string, books []libby.Found) []libby.Found {
	type scored struct {
		f libby.Found
		s float64
	}
	var keep []scored
	for _, b := range books {
		if s := relevance(query, b); s >= 0.5 {
			keep = append(keep, scored{b, s})
		}
	}
	sort.SliceStable(keep, func(i, j int) bool { return keep[i].s > keep[j].s })
	out := make([]libby.Found, 0, len(keep))
	for _, k := range keep {
		out = append(out, k.f)
	}
	return out
}

// handleLibbyCover: GET /api/libby/cover?u=<cover address>: a library's cover picture, passed along because the
// page may only show pictures from NovelCheck itself. Only OverDrive's own picture hosts are fetched.
func (s *Server) handleLibbyCover(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(r.URL.Query().Get("u"))
	host := ""
	if err == nil {
		host = strings.ToLower(u.Hostname())
	}
	if err != nil || u.Scheme != "https" || !(strings.HasSuffix(host, ".od-cdn.com") || strings.HasSuffix(host, ".overdrive.com")) {
		http.NotFound(w, r)
		return
	}
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	hc := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse // a redirect could point anywhere, also inside the home network: not followed
	}}
	resp, err := hc.Do(req)
	if err != nil {
		http.Error(w, "cover unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(ct, "image/") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 2<<20))
}
