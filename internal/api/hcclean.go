package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/calibresrv"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Hardcover cleanup of the Calibre library: a background scan compares every book with Hardcover and keeps the
// differences as suggestions; the admin ticks the ones to apply; a second background job writes them to Calibre
// and reads each book back to check Calibre took them.

type hcJobState struct {
	Kind    string   `json:"kind"` // "scan" or "apply"
	Running bool     `json:"running"`
	Done    int      `json:"done"`
	Total   int      `json:"total"`
	Found   int      `json:"found"` // scan: suggestions kept; apply: books fixed
	Note    string   `json:"note"`
	Errors  []string `json:"errors"`
}

type hcJob struct {
	mu   sync.Mutex
	st   hcJobState
	stop bool
}

func (j *hcJob) state() hcJobState {
	j.mu.Lock()
	defer j.mu.Unlock()
	st := j.st
	st.Errors = append([]string{}, j.st.Errors...)
	return st
}

func (j *hcJob) start(kind string, total int) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.st.Running {
		return false
	}
	j.st, j.stop = hcJobState{Kind: kind, Running: true, Total: total}, false
	return true
}

func (j *hcJob) update(f func(*hcJobState)) { j.mu.Lock(); f(&j.st); j.mu.Unlock() }
func (j *hcJob) fail(msg string) {
	j.update(func(s *hcJobState) {
		if len(s.Errors) < 30 {
			s.Errors = append(s.Errors, msg)
		}
	})
}
func (j *hcJob) end(note string) { j.update(func(s *hcJobState) { s.Running, s.Note = false, note }) }
func (j *hcJob) cancel()         { j.mu.Lock(); j.stop = true; j.mu.Unlock() }
func (j *hcJob) stopped() bool   { j.mu.Lock(); defer j.mu.Unlock(); return j.stop }

func isbnOf(d calibresrv.Details, fallback string) string {
	if v := d.Identifiers["isbn"]; v != "" {
		return v
	}
	return fallback
}

// runHCScan compares the library with Hardcover, 25 books at a time.
func (s *Server) runHCScan(token string, refs []store.CalibreBookRef) {
	defer func() {
		if r := recover(); r != nil {
			s.hcjob.end(fmt.Sprintf("Stopped by a problem: %v", r))
		}
	}()
	cal, hc := s.calibreClient(), &hardcover.Client{Token: token, URL: hardcoverURL}
	_ = s.Store.ClearNewHCSuggestions()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Hour)
	defer cancel()
	for i := 0; i < len(refs); i += 25 {
		if s.hcjob.stopped() {
			s.hcjob.end("Stopped; what was found so far is below.")
			return
		}
		chunk := refs[i:min(i+25, len(refs))]
		ids := make([]int, len(chunk))
		for k, r := range chunk {
			ids[k] = r.CalibreID
		}
		have, err := cal.Details(ctx, ids)
		if err != nil {
			s.hcjob.end("Calibre didn't answer: " + err.Error())
			return
		}
		var isbns []string
		for _, r := range chunk {
			if v := isbnOf(have[r.CalibreID], r.ISBN); v != "" {
				isbns = append(isbns, v)
			}
		}
		byISBN, err := hc.InfoByISBN(ctx, isbns)
		if err != nil {
			s.hcjob.end("Hardcover: " + err.Error() + " (what was found so far is below)")
			return
		}
		for _, r := range chunk {
			d, ok := have[r.CalibreID]
			if !ok {
				continue
			}
			info, how := byISBN[digitsOnly(isbnOf(d, r.ISBN))], "ISBN"
			if info.ID == 0 && len(d.Authors) > 0 {
				found, ok, err := hc.InfoByTitle(ctx, d.Title, d.Authors[0])
				if err != nil {
					s.hcjob.end("Hardcover: " + err.Error() + " (what was found so far is below)")
					return
				}
				if ok {
					info, how = found, "exact title and author"
				}
			}
			if info.ID == 0 {
				continue
			}
			for _, c := range hcChanges(d, info) {
				payload, _ := json.Marshal(c.Payload)
				s.Store.AddHCSuggestion(store.HCSuggestion{BookID: r.BookID, CalibreID: r.CalibreID, Title: d.Title, Field: c.Field,
					Current: c.Current, Shown: c.Shown, Payload: string(payload), Confidence: how})
				s.hcjob.update(func(st *hcJobState) { st.Found++ })
			}
		}
		s.hcjob.update(func(st *hcJobState) { st.Done += len(chunk) })
	}
	s.hcjob.end(fmt.Sprintf("Compared %d books with Hardcover.", len(refs)))
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == 'X' || r == 'x' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// runHCApply writes the chosen suggestions to Calibre, one book at a time (calibre renames files, which is slow),
// and checks by reading each book back.
func (s *Server) runHCApply(list []store.HCSuggestion) {
	groups := map[int][]store.HCSuggestion{}
	var order []int
	for _, x := range list {
		if _, ok := groups[x.CalibreID]; !ok {
			order = append(order, x.CalibreID)
		}
		groups[x.CalibreID] = append(groups[x.CalibreID], x)
	}
	cal := s.calibreClient()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
	defer cancel()
	for _, id := range order {
		if s.hcjob.stopped() {
			s.hcjob.end("Stopped.")
			return
		}
		g := groups[id]
		title := g[0].Title
		before, err := cal.Details(ctx, []int{id})
		if err != nil {
			s.hcjob.end("Calibre didn't answer: " + err.Error())
			return
		}
		if d, ok := before[id]; !ok || store.NormKey(d.Title, "") != store.NormKey(title, "") {
			s.hcjob.fail("“" + title + "”: that Calibre book changed since the scan (or is in another library); scan again")
			s.hcjob.update(func(st *hcJobState) { st.Done++ })
			continue
		}
		fields, err := fieldsOf(g)
		if err == nil && len(fields) > 0 {
			_, err = cal.SetFields(ctx, id, fields)
		}
		for _, x := range g {
			if x.Field != "cover" || err != nil {
				continue
			}
			var src string
			_ = json.Unmarshal([]byte(x.Payload), &src)
			img, gerr := fetchCover(ctx, src)
			if gerr == nil {
				gerr = cal.SetCover(ctx, id, img)
			}
			if gerr != nil {
				s.hcjob.fail("“" + title + "”: cover: " + gerr.Error())
			}
		}
		if err != nil {
			s.hcjob.fail("“" + title + "”: " + err.Error())
			s.hcjob.update(func(st *hcJobState) { st.Done++ })
			continue
		}
		after, err := cal.Details(ctx, []int{id})
		var wrote []int64
		for _, x := range g {
			if err == nil && hcApplied(after[id], x) {
				wrote = append(wrote, x.ID)
			} else {
				s.hcjob.fail("“" + title + "”: Calibre didn't keep the " + x.Field)
			}
		}
		s.Store.SetHCSuggestionStatus(wrote, "applied")
		if len(wrote) > 0 {
			s.hcjob.update(func(st *hcJobState) { st.Found++ })
		}
		s.hcjob.update(func(st *hcJobState) { st.Done++ })
	}
	s.syncSoon(nil) // NovelCheck reads Calibre again so it shows the new details
	log.Printf("hardcover cleanup: applied suggestions to %d Calibre books", len(order))
	s.hcjob.end(fmt.Sprintf("Fixed %d books in Calibre.", s.hcjob.state().Found))
}

// hcApplied checks that Calibre now has the suggested value.
func hcApplied(d calibresrv.Details, x store.HCSuggestion) bool {
	var v any
	_ = json.Unmarshal([]byte(x.Payload), &v)
	switch x.Field {
	case "cover":
		return d.Cover != ""
	case "title":
		return d.Title == v
	case "series":
		return d.Series == v
	case "series_index":
		f, _ := v.(float64)
		return d.SeriesIndex == f
	case "comments":
		return d.Comments != ""
	case "authors":
		l, _ := v.([]any)
		return len(l) > 0 && len(d.Authors) > 0 && person(d.Authors[0]) == person(fmt.Sprint(l[0]))
	case "identifiers":
		m, _ := v.(map[string]any)
		return d.Identifiers["isbn"] == fmt.Sprint(m["isbn"])
	case "tags":
		l, _ := v.([]any)
		return len(d.Tags) >= len(l)
	}
	return false
}

// fetchCover downloads a cover picture: https only, a real image, at most 10 MB, no redirects to elsewhere.
func fetchCover(ctx context.Context, src string) ([]byte, error) {
	u, err := url.Parse(src)
	if err != nil || u.Scheme != "https" {
		return nil, fmt.Errorf("not a secure picture address")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	hc := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if r.URL.Scheme != "https" || len(via) > 3 {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
		return nil, fmt.Errorf("no picture there (%s)", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20+1))
	if err != nil || len(data) > 10<<20 || len(data) < 1000 {
		return nil, fmt.Errorf("the picture is too large or empty")
	}
	return data, nil
}
