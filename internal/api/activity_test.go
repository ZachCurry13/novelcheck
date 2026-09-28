package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestActivityAndBookStates(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	a, _ := st.UpsertBook("Waiting One", "Author", "", "")
	b, _ := st.UpsertBook("Waiting Two", "Author", "", "")
	_ = st.AddCopy(cat, a, "/c/a.epub", "epub", "1")
	_ = st.AddCopy(cat, b, "/c/b.epub", "epub", "2")

	res, out := admin.do("GET", "/api/activity", nil, false)
	if res.StatusCode != 200 || out["waiting"].(float64) != 2 || out["auto"] != false || out["hours_open"] != true {
		t.Fatalf("activity: %d %v", res.StatusCode, out)
	}
	res, out = admin.do("GET", "/api/books/states?ids="+itoa(a)+",x,"+itoa(b), nil, false)
	if books, _ := out["books"].([]any); res.StatusCode != 200 || len(books) != 2 {
		t.Fatalf("states: %d %v", res.StatusCode, out)
	}

	for _, c := range []struct {
		key, value string
		want       int
	}{
		{"auto_rate", "maybe", 400}, {"auto_rate", "on", 200},
		{"auto_rate_hours", "25-1", 400}, {"auto_rate_hours", "23-7", 200},
		{"auto_rate_tz", "Nowhere/Particular", 400}, {"auto_rate_tz", "Europe/London", 200},
	} {
		if res, _ := admin.do("PUT", "/api/admin/settings", map[string]string{c.key: c.value}, true); res.StatusCode != c.want {
			t.Fatalf("%s=%s: %d, want %d", c.key, c.value, res.StatusCode, c.want)
		}
	}
	if _, out = admin.do("GET", "/api/activity", nil, false); out["auto"] != true {
		t.Fatalf("auto after turning it on: %v", out)
	}
}
