// Package updates checks GitHub for newer NovelCheck releases so the app can
// tell admins an update is available and show release notes.
package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/version"
)

// DefaultURL lists published releases, newest first.
const DefaultURL = "https://api.github.com/repos/ZachCurry13/novelcheck/releases?per_page=20"

// cacheFor limits GitHub calls (unauthenticated limit is 60/hour per IP).
const cacheFor = 6 * time.Hour

type Release struct {
	Tag         string `json:"tag"`
	Name        string `json:"name"`
	Notes       string `json:"notes"` // Markdown
	URL         string `json:"url"`
	PublishedAt string `json:"published_at"`
	Installed   bool   `json:"installed"`
	Newer       bool   `json:"newer"`
}

type Status struct {
	Current         string    `json:"current"`
	Latest          string    `json:"latest"`
	UpdateAvailable bool      `json:"update_available"`
	CheckedAt       string    `json:"checked_at,omitempty"`
	Error           string    `json:"error,omitempty"`
	Releases        []Release `json:"releases"`
}

type Checker struct {
	URL  string
	HTTP *http.Client

	mu      sync.Mutex
	fetched time.Time
	cached  []Release
	lastErr error
}

func New() *Checker {
	return &Checker{URL: DefaultURL, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Status compares the running version with published releases. With
// enabled=false no network call is made and only the current version is set.
func (c *Checker) Status(ctx context.Context, enabled bool) Status {
	st := Status{Current: version.Version, Releases: []Release{}}
	if !enabled {
		return st
	}
	releases, fetched, err := c.releases(ctx)
	if err != nil {
		st.Error = "could not reach GitHub to check for updates"
	}
	if !fetched.IsZero() {
		st.CheckedAt = fetched.UTC().Format(time.RFC3339)
	}
	for _, r := range releases {
		r.Newer = version.Newer(r.Tag, st.Current)
		r.Installed = !r.Newer && sameCore(r.Tag, st.Current)
		if st.Latest == "" && version.Valid(r.Tag) {
			st.Latest = r.Tag
			st.UpdateAvailable = r.Newer
		}
		st.Releases = append(st.Releases, r)
	}
	return st
}

func sameCore(a, b string) bool {
	return version.Valid(a) && version.Valid(b) && !version.Newer(a, b) && !version.Newer(b, a)
}

// releases returns cached releases, refreshing them when stale.
func (c *Checker) releases(ctx context.Context) ([]Release, time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.fetched) < cacheFor && (c.cached != nil || c.lastErr != nil) {
		return c.cached, c.fetched, c.lastErr
	}
	rs, err := c.fetch(ctx)
	c.fetched, c.lastErr = time.Now(), err
	if err == nil {
		c.cached = rs
	}
	return c.cached, c.fetched, err
}

func (c *Checker) fetch(ctx context.Context) ([]Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "NovelCheck/"+version.Version)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub releases: %s", resp.Status)
	}
	var raw []struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Body        string `json:"body"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
		Draft       bool   `json:"draft"`
		Prerelease  bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 2<<20)).Decode(&raw); err != nil {
		return nil, err
	}
	var out []Release
	for _, r := range raw {
		if r.Draft || r.Prerelease {
			continue
		}
		out = append(out, Release{Tag: r.TagName, Name: r.Name, Notes: r.Body, URL: r.HTMLURL, PublishedAt: r.PublishedAt})
	}
	return out, nil
}
