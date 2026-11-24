package api_test

import (
	"testing"
)

// The Library's "Books I've read": only the viewer's own finished books (Nightstand, import or Hardcover), or the rest.
func TestBooksIveReadFilter(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "sister", "password": "sisterpass1", "role": "editor"}, true)
	cal, _ := st.EnsureCatalog("Family", "custom")
	add := func(title string) int64 {
		id, _ := st.UpsertBook(title, "Writer", "", "")
		_ = st.AddCopy(cal, id, title, "list", "")
		return id
	}
	done, imported, open := add("Done Book"), add("Imported Book"), add("Open Book")
	_, q := admin.do("POST", "/api/queue", map[string]any{"book_id": done}, true)
	_ = q
	_, list := admin.do("GET", "/api/queue", nil, false)
	_ = list
	items := []map[string]any{}
	if res, l := admin.doList("GET", "/api/queue"); res.StatusCode == 200 {
		items = l
	}
	var qid string
	for _, it := range items {
		if int64(it["book_id"].(float64)) == done {
			qid = itoa(int64(it["id"].(float64)))
		}
	}
	if qid == "" {
		_, m := admin.do("GET", "/api/queue", nil, false)
		for _, x := range m["items"].([]any) {
			it := x.(map[string]any)
			if int64(it["book_id"].(float64)) == done {
				qid = itoa(int64(it["id"].(float64)))
			}
		}
	}
	admin.do("POST", "/api/queue/"+qid+"/finish", map[string]any{}, true)
	u, _ := st.UserByName("admin")
	_ = st.SetFinishedOn(u.ID, imported, "2026-01-02")
	titles := func(c *client, read string) map[string]bool {
		_, out := c.do("GET", "/api/books?read="+read+"&limit=50", nil, false)
		got := map[string]bool{}
		for _, x := range out["books"].([]any) {
			got[x.(map[string]any)["title"].(string)] = true
		}
		return got
	}
	if r := titles(admin, "1"); !r["Done Book"] || !r["Imported Book"] || r["Open Book"] {
		t.Fatalf("read: %v", r)
	}
	if r := titles(admin, "0"); r["Done Book"] || r["Imported Book"] || !r["Open Book"] {
		t.Fatalf("not read: %v", r)
	}
	sister := login(t, srv, "sister", "sisterpass1")
	if r := titles(sister, "1"); len(r) != 0 {
		t.Fatalf("someone else's reading doesn't count: %v", r)
	}
	_ = open
}
