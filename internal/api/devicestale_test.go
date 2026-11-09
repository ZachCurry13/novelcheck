package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A Kindle book the person matched to a book that was later merged away must
// not break the next report (it used to fail with a 500), and the choice
// follows the merge.
func TestDeviceChoiceSurvivesMerge(t *testing.T) {
	_, st := setup(t)
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	a, _ := st.UpsertBook("Scarlet", "Marissa Meyer", "", "")
	_ = st.AddCopy(cal, a, "/c/a.epub", "epub", "1")
	b, _ := st.UpsertBook("Scarlet", "", "", "")
	_ = st.AddCopy(cal, b, "/c/b.epub", "epub", "2")
	u, _ := st.UserByName("admin")
	if err := st.ApplyMatch(u.ID, store.MatchRef{Kind: "device", Device: "Kindle", Filename: "s.epub", MD5: "ccc"}, b, false); err != nil {
		t.Fatal(err)
	}
	keep, err := st.MergeBooks(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveDeviceBooks(u.ID, "Kindle", []store.DeviceBook{{Filename: "s.epub", MD5: "ccc", Title: "Scarlet", Author: "Marissa Meyer"}}); err != nil {
		t.Fatalf("a report after a merge must save: %v", err)
	}
	if ids := st.DeviceBookIDs(u.ID, "Kindle"); len(ids) != 1 || ids[0] != keep {
		t.Fatalf("the choice follows the merge: %v (kept %d)", ids, keep)
	}
}
