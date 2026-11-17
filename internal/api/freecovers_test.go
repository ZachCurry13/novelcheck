package api_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The cover finder also looks for the covers of free books (on a running event, not in a library): a book listed
// with its Amazon ASIN is found on Hardcover by that exact number; a book on an archived event is left alone.
func TestCoverFinderFindsFreeBookCovers(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 60, 90))
	for x := 0; x < 60; x++ {
		for y := 0; y < 90; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 2), 90, 255})
		}
	}
	var jpg bytes.Buffer
	_ = jpeg.Encode(&jpg, img, nil)
	asked := map[string]int{}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/free.jpg" {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(jpg.Bytes())
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var q struct{ Query string }
		_ = json.Unmarshal(raw, &q)
		switch {
		case strings.Contains(q.Query, "asin: {_in"):
			asked["asin"]++
			io.WriteString(w, `{"data": {"editions": [{"asin": "B0FREEBOOK", "book": {"id": 77, "title": "Quick Free Read", "cached_image": {"url": "https://assets.hardcover.app/free.jpg"},
				"contributions": [{"author": {"name": "Pat Free"}}]}}]}}`)
		case q.Query != "":
			io.WriteString(w, `{"data": {"me": [{"id": 7, "username": "reader7"}], "search": {"ids": []}}}`)
		default:
			io.WriteString(w, `{"docs": [], "items": []}`)
		}
	}))
	defer fake.Close()
	to, _ := url.Parse(fake.URL)
	old := http.DefaultTransport
	http.DefaultTransport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "assets.hardcover.app" {
			r = r.Clone(r.Context())
			r.URL.Scheme, r.URL.Host = to.Scheme, to.Host
		}
		return old.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	api.SetHardcoverURL(fake.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	srv, st := setupWith(t, func(s *api.Server) {
		s.Covers.OpenLibraryURL, s.Covers.SearchURL, s.Covers.GoogleURL = fake.URL, fake.URL, fake.URL
		s.Worker.NewEnricher = func() *enrich.Client {
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = fake.URL, fake.URL
			return c
		}
	})
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)
	_, ids, err := st.CreateEvent("Stuff Your Kindle: Mystery", "https://example.org/day", "admin", "", []store.EventEntry{
		{Title: "Quick Free Read", Author: "Pat Free", ASIN: "B0FREEBOOK", Link: "https://example.org/a"}})
	if err != nil || len(ids) == 0 {
		t.Fatalf("event: %v %v", ids, err)
	}
	old2, _, err := st.CreateEvent("Last Week", "https://example.org/old", "admin", "", []store.EventEntry{
		{Title: "Old Free Read", Author: "Pat Old", ASIN: "B0OLDBOOK", Link: "https://example.org/b"}})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = st.DB.Exec(`UPDATE events SET archived_at = '2026-01-01' WHERE id = ?`, old2)

	free, _ := st.FreeBooks()
	if len(free) != 1 || free[0].Title != "Quick Free Read" || free[0].ASIN != "B0FREEBOOK" {
		t.Fatalf("only the running event's book, with its ASIN: %+v", free)
	}
	if res, _ := admin.do("POST", "/api/admin/covers/find", map[string]string{}, true); res.StatusCode != 202 {
		t.Fatalf("start: %d", res.StatusCode)
	}
	contentType := func() string {
		res, err := admin.http.Get(srv.URL + "/api/books/" + itoa(ids[0]) + "/cover")
		if err != nil {
			return ""
		}
		defer res.Body.Close()
		return res.Header.Get("Content-Type")
	}
	deadline := time.Now().Add(20 * time.Second)
	for !strings.HasPrefix(contentType(), "image/jpeg") {
		if time.Now().After(deadline) {
			t.Fatalf("no cover for the free book (asked Hardcover by ASIN %d times, served %q)", asked["asin"], contentType())
		}
		time.Sleep(250 * time.Millisecond)
	}
	if asked["asin"] == 0 {
		t.Fatal("the book's ASIN should have been looked up on Hardcover")
	}
}

// A free book that comes back on a later event (its row was deleted and made again, so it has a new id) takes the
// cover that was found for it before, without a search on the web, and is already rated.
func TestFreeBookKeepsItsCoverAndRatingWhenItComesBack(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 60, 90))
	for x := 0; x < 60; x++ {
		for y := 0; y < 90; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), 60, uint8(y * 2), 255})
		}
	}
	var jpg bytes.Buffer
	_ = jpeg.Encode(&jpg, img, nil)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"docs": [], "items": []}`) // nothing on the web: a cover can only come from the memory
	}))
	defer fake.Close()
	var server *api.Server
	srv, st := setupWith(t, func(s *api.Server) {
		server = s
		s.Covers.OpenLibraryURL, s.Covers.SearchURL, s.Covers.GoogleURL = fake.URL, fake.URL, fake.URL
		s.Worker.NewEnricher = func() *enrich.Client {
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = fake.URL, fake.URL
			return c
		}
	})
	admin := login(t, srv, "admin", "adminpass1")
	entry := []store.EventEntry{{Title: "Comes Back Free", Author: "Pat Again", ASIN: "B0AGAIN001", Link: "https://example.org/x"}}
	evt, ids, err := st.CreateEvent("Free Day One", "https://example.org/one", "admin", "", entry)
	if err != nil || len(ids) != 1 {
		t.Fatalf("event: %v %v", ids, err)
	}
	first := ids[0]
	two := 2
	_ = st.SaveAnalysis(first, store.Analysis{SpiceLevel: &two, Classification: "Closed Door", Confidence: "high", Model: "llama3.2"})
	if _, _, ok := server.Covers.Keep("book-"+itoa(first), jpg.Bytes()); !ok {
		t.Fatal("setup: the cover could not be stored")
	}
	// The event ends and the book leaves everything.
	if err := st.DeleteEvent(evt); err != nil {
		t.Fatal(err)
	}
	_, _ = st.DB.Exec(`DELETE FROM books WHERE id NOT IN (SELECT book_id FROM catalog_books)`)
	_, ids, err = st.CreateEvent("Free Day Two", "https://example.org/two", "admin", "", entry)
	if err != nil || len(ids) != 0 {
		t.Fatalf("it comes back already rated, so nothing is waiting to be rated: %v %v", ids, err)
	}
	free, _ := st.FreeBooks()
	if len(free) != 1 || free[0].ID == first {
		t.Fatalf("a new row for the same book: %+v (old id %d)", free, first)
	}
	if b, _ := st.BookByID(free[0].ID, nil); b == nil || b.Status != "analyzed" || b.SpiceLevel == nil || *b.SpiceLevel != 2 {
		t.Fatalf("its rating came back: %+v", b)
	}
	if res, _ := admin.do("POST", "/api/admin/covers/find", map[string]string{}, true); res.StatusCode != 202 {
		t.Fatalf("start: %d", res.StatusCode)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		res, err := admin.http.Get(srv.URL + "/api/books/" + itoa(free[0].ID) + "/cover")
		if err == nil {
			ct := res.Header.Get("Content-Type")
			res.Body.Close()
			if strings.HasPrefix(ct, "image/jpeg") {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("the cover found before was not given back")
		}
		time.Sleep(250 * time.Millisecond)
	}
}
