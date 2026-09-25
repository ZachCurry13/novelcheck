package api_test

import (
	"encoding/xml"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/deepread"
	"github.com/zachcurry13/novelcheck/internal/store"
)

type feed struct {
	Entries []struct {
		Title string `xml:"title"`
		Links []struct {
			Rel  string `xml:"rel,attr"`
			Href string `xml:"href,attr"`
			Type string `xml:"type,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

func getFeed(t *testing.T, url string) (int, feed) {
	t.Helper()
	res, err := http.Get(url) // no cookies: KOReader can't sign in
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var f feed
	if res.StatusCode == 200 {
		raw, _ := io.ReadAll(res.Body)
		if err := xml.Unmarshal(raw, &f); err != nil {
			t.Fatalf("feed XML: %v\n%s", err, raw)
		}
	}
	return res.StatusCode, f
}

func TestKOReaderFeed(t *testing.T) {
	calibreDir := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) {
		s.Cfg.CalibreDir = calibreDir
		s.Deep = deepread.New(s.Store, calibreDir)
	})
	admin := login(t, srv, "admin", "adminpass1")
	_ = st.SetSetting(store.KeySMTPFrom, "books@example.com")
	if _, me := admin.do("GET", "/api/me", nil, false); me["delivery_from"] != "books@example.com" {
		t.Fatalf("delivery_from: %v", me["delivery_from"])
	}
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	file := filepath.Join(calibreDir, "A", "Spicy Read.epub")
	_ = os.MkdirAll(filepath.Dir(file), 0o755)
	_ = os.WriteFile(file, []byte("epub bytes"), 0o644)
	four := 4
	id, _ := st.UpsertBook("Spicy Read", "A", "", "")
	_ = st.AddCopy(cat, id, file, "epub", "1")
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &four, Model: "gpt", SummaryVerdict: "Explicit."})
	_ = st.Enqueue(1, id)

	_, my := admin.do("GET", "/api/me/opds", nil, false)
	path := my["path"].(string)
	code, f := getFeed(t, srv.URL+path)
	if code != 200 || len(f.Entries) != 1 || f.Entries[0].Title != "Spicy Read" {
		t.Fatalf("feed: %d %+v", code, f)
	}
	link := f.Entries[0].Links[0]
	if link.Rel != "http://opds-spec.org/acquisition" || link.Type != "application/epub+zip" || !strings.HasSuffix(link.Href, "/Spicy-Read.epub") {
		t.Fatalf("acquisition link: %+v", link)
	}
	res, err := http.Get(link.Href)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(body) != "epub bytes" || res.Header.Get("Content-Type") != "application/epub+zip" {
		t.Fatalf("download: %d %q %s", res.StatusCode, body, res.Header.Get("Content-Type"))
	}
	if code, _ := getFeed(t, srv.URL+"/opds/"+strings.Repeat("0", 40)); code != 404 {
		t.Fatalf("unknown token: %d", code)
	}

	// A kid's feed follows the kid's rules, even for direct links.
	_, kid := admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kidID := int64(kid["id"].(float64))
	_ = st.Enqueue(kidID, id)
	_, kf := admin.do("GET", "/api/admin/users/"+itoa(kidID)+"/opds", nil, false)
	if code, f := getFeed(t, srv.URL+kf["path"].(string)); code != 200 || len(f.Entries) != 0 {
		t.Fatalf("a 4-pepper book must not reach a kid's reader: %d %+v", code, f)
	}
	if res, _ := http.Get(srv.URL + kf["path"].(string) + "/book/" + itoa(id) + "/x.epub"); res.StatusCode != 404 {
		t.Fatalf("kid download: %d", res.StatusCode)
	}

	// Editors set up kids' devices, not the admin's.
	admin.do("POST", "/api/admin/users", map[string]string{"username": "mom", "password": "editorpass1", "role": "editor"}, true)
	mom := login(t, srv, "mom", "editorpass1")
	if res, _ := mom.do("GET", "/api/admin/users/1/opds", nil, false); res.StatusCode != 403 {
		t.Fatalf("editor → admin's feed: %d", res.StatusCode)
	}

	// A new address retires the old one; switching KOReader off closes feeds.
	admin.do("POST", "/api/me/opds/reset", nil, true)
	if code, _ := getFeed(t, srv.URL+path); code != 404 {
		t.Fatalf("old address after reset: %d", code)
	}
	_, my = admin.do("GET", "/api/me/opds", nil, false)
	admin.do("PUT", "/api/admin/settings", map[string]string{"module_koreader": "false"}, true)
	if code, _ := getFeed(t, srv.URL+my["path"].(string)); code != 404 {
		t.Fatalf("KOReader switched off: %d", code)
	}
}
