package api_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// statsFile writes a small KOReader statistics.sqlite3: Dune opened just
// now at page 172 of 400, and Mort read to the end long ago.
func statsFile(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "statistics.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE book (id integer PRIMARY KEY autoincrement, title text, authors text, notes integer, last_open integer,
			highlights integer, pages integer, series text, language text, md5 text, total_read_time integer, total_read_pages integer)`,
		`CREATE TABLE page_stat_data (id_book integer, page integer NOT NULL DEFAULT 0, start_time integer NOT NULL DEFAULT 0,
			duration integer NOT NULL DEFAULT 0, total_pages integer NOT NULL DEFAULT 0, UNIQUE (id_book, page, start_time))`,
		fmt.Sprintf(`INSERT INTO book (title, authors, last_open, pages, md5, total_read_time) VALUES
			('Dune', 'Frank Herbert', %d, 400, 'aaa', 7200), ('Mort', 'Terry Pratchett', 1500000000, 300, 'bbb', 600),
			('Somebody Else''s Book', 'Nobody', 1500000000, 100, 'ccc', 60)`, time.Now().Unix()+60),
		`INSERT INTO page_stat_data VALUES (1, 172, 300, 30, 400), (2, 300, 50, 20, 300)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestKOReaderStatisticsFolder(t *testing.T) {
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.DataDir = t.TempDir() })
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, code := admin.do("GET", "/api/me/kosync", nil, false)
	if code["dav"] != "/dav/" {
		t.Fatalf("setup reply: %v", code)
	}
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	for i, b := range [][2]string{{"Dune", "Frank Herbert"}, {"Mort", "Terry Pratchett"}} {
		id, _ := st.UpsertBook(b[0], b[1], "", "")
		_ = st.AddCopy(cal, id, fmt.Sprintf("/c/%d.epub", i), "epub", fmt.Sprint(i+1))
		if b[0] == "Dune" {
			admin.do("POST", "/api/queue", map[string]int64{"book_id": id}, true)
		}
	}

	dav := func(method, path, pass string, body []byte) *http.Response {
		req, _ := http.NewRequest(method, srv.URL+path, bytes.NewReader(body))
		req.SetBasicAuth("admin", pass)
		if method == "PROPFIND" {
			req.Header.Set("Depth", "1")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}
	if res := dav("PROPFIND", "/dav/", "wrong", nil); res.StatusCode != 401 || res.Header.Get("WWW-Authenticate") == "" {
		t.Fatalf("wrong code: %d", res.StatusCode)
	}
	if res := dav("PROPFIND", "/dav/", code["code"].(string), nil); res.StatusCode != 207 {
		t.Fatalf("browse the folder: %d", res.StatusCode)
	}
	if res := dav("GET", "/dav/statistics.sqlite3", code["code"].(string), nil); res.StatusCode != 404 {
		t.Fatalf("first sync finds nothing: %d", res.StatusCode)
	}
	if res := dav("PUT", "/dav/statistics.sqlite3", code["code"].(string), statsFile(t)); res.StatusCode != 201 {
		t.Fatalf("upload: %d", res.StatusCode)
	}

	var books []any
	for i := 0; i < 100 && len(books) == 0; i++ {
		time.Sleep(20 * time.Millisecond)
		_, out := admin.do("GET", "/api/me/koreader-books", nil, false)
		books, _ = out["books"].([]any)
	}
	if len(books) != 3 || books[0].(map[string]any)["title"] != "Dune" || books[0].(map[string]any)["book_id"] == nil ||
		books[2].(map[string]any)["book_id"] != nil {
		t.Fatalf("koreader books: %v", books)
	}

	// Dune was queued and opened since: Reading, 43%. Mort (finished long
	// ago, not queued) isn't added to Up Next.
	_, items := admin.doList("GET", "/api/queue")
	if len(items) != 1 || items[0]["status"] != "reading" {
		t.Fatalf("up next: %v", items)
	}
	if p, _ := items[0]["progress"].(map[string]any); p == nil || p["percent"].(float64) != 0.43 || p["device"] != "KOReader" {
		t.Fatalf("progress: %v", items[0])
	}
	_, reading := admin.do("GET", "/api/admin/users/1/reading", nil, false)
	if r, _ := reading["reading"].([]any); len(r) != 1 || reading["synced_at"].(float64) == 0 {
		t.Fatalf("a reader's reading, for parents: %v", reading)
	}
}
