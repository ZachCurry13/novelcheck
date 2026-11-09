package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The cleanup summary counts what each step would do, and only offers the safe extras:
// an entry with no EPUB where another entry of the same book has one.
func TestCleanupSummary(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	b, _ := st.UpsertBook("Divergent", "Veronica Roth", "", "")
	_ = st.AddCopy(cal, b, "/c/11/divergent.mobi", "mobi", "11") // an extra entry with no EPUB
	_ = st.AddCopy(cal, b, "/c/12/divergent.epub", "epub", "12") // the entry that stays
	_ = st.AddCopy(cal, b, "/c/12/divergent.pdf", "pdf", "12")   // an extra format on it
	lone, _ := st.UpsertBook("Only Mobi", "Someone Else", "", "")
	_ = st.AddCopy(cal, lone, "/c/20/only.mobi", "mobi", "20") // nothing else: to convert, never removed
	two, _ := st.UpsertBook("Both Epub", "Another Person", "", "")
	_ = st.AddCopy(cal, two, "/c/30/a.epub", "epub", "30")
	_ = st.AddCopy(cal, two, "/c/31/a.epub", "epub", "31") // two EPUB entries are left for a person

	res, c := admin.do("GET", "/api/admin/cleanup", nil, false)
	if res.StatusCode != 200 {
		t.Fatalf("summary: %d", res.StatusCode)
	}
	if c["dup_count"] != float64(1) || len(c["dup_ids"].([]any)) != 1 || c["dup_ids"].([]any)[0] != "11" {
		t.Fatalf("only the MOBI-only extra entry is offered: %v", c)
	}
	if c["convert"] != float64(2) { // entry 11 (mobi, no epub) and entry 20
		t.Fatalf("books with no EPUB: %v", c["convert"])
	}
	if c["remove_files"] != float64(1) { // the PDF on entry 12
		t.Fatalf("other formats beside an EPUB: %v", c["remove_files"])
	}
	if res, _ := admin.do("POST", "/api/admin/cleanup/refresh", map[string]string{}, true); res.StatusCode != 200 {
		t.Fatalf("refresh: %d", res.StatusCode)
	}
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	if res, _ := kid.do("GET", "/api/admin/cleanup", nil, false); res.StatusCode != 403 {
		t.Fatalf("admins only: %d", res.StatusCode)
	}
}
