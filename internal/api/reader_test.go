package api_test

import (
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The reader saves your place in a book and gets it back, with where KOReader's progress sync last put you;
// a bad place is refused, and the reader page gets its own security rules (book styles yes, scripts no).
func TestReaderPosition(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	id, err := st.UpsertBook("Reader Book", "A. Writer", "", "")
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/books/" + itoa(id) + "/position"
	if res, _ := admin.do("PUT", path, map[string]any{"cfi": "not a cfi", "fraction": 0.2}, true); res.StatusCode != 400 {
		t.Fatalf("a bad place is refused: %d", res.StatusCode)
	}
	if res, _ := admin.do("PUT", path, map[string]any{"cfi": "epubcfi(/6/4!/4/2/1:0)", "fraction": 0.25}, true); res.StatusCode != 200 {
		t.Fatalf("save: %d", res.StatusCode)
	}
	st.SetDocBook("doc-1", id)
	if err := st.SaveKosync(1, store.KosyncProgress{Document: "doc-1", Progress: "/body", Percentage: 0.6, Device: "Kindle", Timestamp: time.Now().Unix()}); err != nil {
		t.Fatal(err)
	}
	_, out := admin.do("GET", path, nil, false)
	here, _ := out["here"].(map[string]any)
	ko, _ := out["koreader"].(map[string]any)
	if here == nil || here["cfi"] != "epubcfi(/6/4!/4/2/1:0)" || here["fraction"] != 0.25 || ko == nil || ko["fraction"] != 0.6 || ko["device"] != "Kindle" {
		t.Fatalf("your place and KOReader's: %v", out)
	}
	res, _ := admin.do("GET", "/reader.html", nil, false)
	if csp := res.Header.Get("Content-Security-Policy"); res.StatusCode != 200 || !contains(csp, "frame-src 'self' blob:") || !contains(csp, "script-src 'self';") {
		t.Fatalf("reader page rules: %d %q", res.StatusCode, csp)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
