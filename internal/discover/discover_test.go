package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestTitleCase(t *testing.T) {
	for in, want := range map[string]string{
		"THE WOMAN IN ME":                 "The Woman in Me",
		"A COURT OF THORNS AND ROSES":     "A Court of Thorns and Roses",
		"HARRY POTTER: THE BOY WHO LIVED": "Harry Potter: The Boy Who Lived",
		"TWENTY-ONE DAYS OF THE WAR":      "Twenty-One Days of the War",
		"ROCKY II":                        "Rocky II",
		"DON'T LET GO":                    "Don't Let Go",
		"Already Fine":                    "Already Fine",
		"iPhone for Dummies":              "iPhone for Dummies",
	} {
		if got := TitleCase(in); got != want {
			t.Errorf("TitleCase(%q) = %q, want %q", in, got, want)
		}
	}
}

// fakeSources serves NYT lists (a key of "good" works) and Open Library.
func fakeSources(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/nyt/lists/current/"):
			if r.URL.Query().Get("api-key") != "good" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			list := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/nyt/lists/current/"), ".json")
			books := []map[string]any{}
			for i := 1; i <= 3; i++ {
				books = append(books, map[string]any{"rank": i, "title": strings.ToUpper(fmt.Sprintf("the %s book %d", list, i)),
					"author": "Author " + list, "primary_isbn13": fmt.Sprintf("97800000%05d", i), "description": "A story.",
					"book_image": "https://storage.googleapis.com/x.jpg", "weeks_on_list": i})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"results": map[string]any{"books": books}})
		case strings.HasPrefix(r.URL.Path, "/ol/subjects/"):
			subject := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/ol/subjects/"), ".json")
			_ = json.NewEncoder(w).Encode(map[string]any{"works": []map[string]any{
				{"key": "/works/OL1W", "title": "Old " + subject, "authors": []map[string]string{{"name": "Ann Author"}}, "cover_id": 42}}})
		case r.URL.Path == "/ol/trending/weekly.json":
			_ = json.NewEncoder(w).Encode(map[string]any{"works": []map[string]any{
				{"key": "/works/OL2W", "title": "Trendy", "author_name": []string{"Tia Trend"}, "language": []string{"eng"}},
				{"key": "/works/OL3W", "title": "O Livro", "author_name": []string{"Rui"}, "language": []string{"por"}}}})
		default:
			http.NotFound(w, r)
		}
	}))
}

func newService(t *testing.T, srv *httptest.Server) (*Service, *[]int64) {
	t.Helper()
	d, err := db.OpenDSN("file:" + t.Name() + "?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	var queued []int64
	s := New(store.New(d), func(ids ...int64) { queued = append(queued, ids...) })
	s.Client.NYTURL, s.Client.OpenLibraryURL, s.Pause = srv.URL+"/nyt", srv.URL+"/ol", 0
	return s, &queued
}

func TestRefreshWithKey(t *testing.T) {
	srv := fakeSources(t)
	defer srv.Close()
	s, queued := newService(t, srv)
	_ = s.Store.SetSetting(store.KeyNYTAPIKey, "good")
	_ = s.Store.SetSetting(store.KeyDiscoverDaily, "4")
	summary, err := s.Refresh(context.Background())
	if err != nil || !strings.HasPrefix(summary, "16 books on 6 lists; 4 sent to be rated") {
		t.Fatalf("%q %v", summary, err)
	}
	if len(*queued) != 4 || s.Due(timeNow()) {
		t.Fatalf("queued %v, due %v", *queued, s.Due(timeNow()))
	}
	// Today's allowance is used up: refreshing again rates nothing more.
	if summary, _ = s.Refresh(context.Background()); !strings.Contains(summary, "0 sent to be rated") {
		t.Fatalf("second refresh: %q", summary)
	}
	items, _ := s.Store.DiscoverItems(nil)
	rows, _ := Compose(items, nil, nil, nil, false)
	if len(rows) != 6 || rows[0].Key != "popular" || rows[0].Books[0].Title != "The Combined-Print-And-E-Book-Fiction Book 1" ||
		rows[1].Key != "new" || len(rows[1].Books) != 5 || !FromNYT(rows) {
		t.Fatalf("rows: %d %+v", len(rows), rows[0].Books[0].Title)
	}
	if got := s.Store.DiscoverCoverURL(rows[0].Books[0].ID); got != "https://storage.googleapis.com/x.jpg" {
		t.Fatalf("cover %q", got)
	}
}

func TestRefreshWithoutAWorkingKey(t *testing.T) {
	srv := fakeSources(t)
	defer srv.Close()
	s, _ := newService(t, srv)
	_ = s.Store.SetSetting(store.KeyNYTAPIKey, "bad")
	summary, err := s.Refresh(context.Background())
	if err != nil || !strings.Contains(summary, "refused the API key") {
		t.Fatalf("%q %v", summary, err)
	}
	items, _ := s.Store.DiscoverItems(nil)
	rows, _ := Compose(items, nil, nil, nil, false)
	keys := []string{}
	for _, r := range rows {
		keys = append(keys, r.Key+":"+r.Books[0].Title)
	}
	if strings.Join(keys, ",") != "popular:Trendy,teen:Old young_adult_fiction,kids:Old juvenile_fiction,classics:Old classics" || FromNYT(rows) {
		t.Fatalf("fallback rows: %v", keys)
	}
	if n, _ := s.Test(context.Background(), "good"); n != 3 {
		t.Fatalf("test key: %d", n)
	}
}

func TestKidsSeeTheirRowsFirst(t *testing.T) {
	book := func(list string, id int64) store.DiscoverBook {
		return store.DiscoverBook{Book: store.Book{ID: id}, List: list}
	}
	items := []store.DiscoverBook{book(listFiction, 1), book(listTeen, 2), book(listMiddle, 3), book(listClassics, 4)}
	order := func(u *store.User) string {
		var keys []string
		rows, _ := Compose(items, nil, nil, u, false)
		for _, r := range rows {
			keys = append(keys, r.Key)
		}
		return strings.Join(keys, ",")
	}
	if got := order(nil); got != "popular,teen,kids,classics" {
		t.Fatalf("adults: %s", got)
	}
	if got := order(&store.User{Role: store.RoleRestricted, AgeLevel: 2}); got != "kids,teen,classics,popular" {
		t.Fatalf("middle grade: %s", got)
	}
}

func timeNow() time.Time { return time.Now() }

func TestOwnedBooksLeftOutOfTheLists(t *testing.T) {
	book := func(list string, id int64, owned bool) store.DiscoverBook {
		return store.DiscoverBook{Book: store.Book{ID: id}, List: list, Owned: owned}
	}
	items := []store.DiscoverBook{book(listFiction, 1, true), book(listFiction, 2, false), book(listClassics, 3, true), book(listClassics, 1, true)}
	family := []store.DiscoverBook{book("family", 1, true)}
	rows, owned := Compose(items, nil, family, nil, false)
	keys := []string{}
	for _, r := range rows {
		keys = append(keys, fmt.Sprintf("%s:%d", r.Key, len(r.Books)))
	}
	// Book 1 is on two lists but counts once; the family's own row keeps it.
	if strings.Join(keys, ",") != "popular:1,family:1" || owned != 2 {
		t.Fatalf("rows %v, owned %d", keys, owned)
	}
	if rows, owned = Compose(items, nil, family, nil, true); len(rows) != 3 || owned != 0 {
		t.Fatalf("with owned: %d rows, %d", len(rows), owned)
	}
}
