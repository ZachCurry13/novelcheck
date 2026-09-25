package api_test

import (
	"fmt"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestDeepScanReview(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	zero, four := 0, 4
	book := func(title string) int64 {
		id, _ := st.UpsertBook(title, "Author", "", "")
		_ = st.AddCopy(cat, id, "/calibre/"+title+".epub", "epub", title)
		_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &zero, Model: "small-model"})
		return id
	}
	held := func(id int64) int64 {
		scan, _ := st.RequestDeepRead(id, "admin", "admin", "", true, 1000, 1, 1)
		if err := st.HoldDeepRead(scan, `[{"label":"Chapter 9","level":4,"note":"a couple has sex"}]`,
			store.Analysis{SpiceLevel: &four, Nudity: true, Model: store.DeepModelPrefix + "qwen2.5:7b"}, 4); err != nil {
			t.Fatal(err)
		}
		return scan
	}
	accept, keep := book("Accepted"), book("Kept")
	a, k := held(accept), held(keep)

	_, s := admin.do("GET", "/api/admin/status", nil, false)
	if s["deep_review"].(float64) != 2 {
		t.Fatalf("waiting for review: %v", s["deep_review"])
	}
	if b, _ := st.BookByID(accept, nil); *b.SpiceLevel != 0 {
		t.Fatal("a held result doesn't apply by itself")
	}
	if res, _ := admin.do("POST", fmt.Sprintf("/api/admin/deep-scans/%d/accept", a), nil, true); res.StatusCode != 200 {
		t.Fatalf("accept: %d", res.StatusCode)
	}
	if b, _ := st.BookByID(accept, nil); *b.SpiceLevel != 4 || !b.Nudity {
		t.Fatalf("accepted: %+v", b)
	}
	if res, _ := admin.do("POST", fmt.Sprintf("/api/admin/deep-scans/%d/keep", k), nil, true); res.StatusCode != 200 {
		t.Fatalf("keep: %d", res.StatusCode)
	}
	if b, _ := st.BookByID(keep, nil); *b.SpiceLevel != 0 || st.HeldDeepReads() != 0 || st.LatestDeepRead(keep).Status != "declined" {
		t.Fatalf("kept the old rating: %+v", b)
	}
	if res, _ := admin.do("POST", fmt.Sprintf("/api/admin/deep-scans/%d/accept", k), nil, true); res.StatusCode != 409 {
		t.Fatal("already decided")
	}

	// A rating from a Deep Scan made with the old checks is found; the new one isn't.
	old := book("Old Scan")
	_, _ = st.DB.Exec(`INSERT INTO deep_reads (book_id, status, checks) VALUES (?, 'done', 1)`, old)
	_ = st.SaveAnalysis(old, store.Analysis{SpiceLevel: &four, Model: store.DeepModelPrefix + "llama3.2"})
	if ids := st.OldDeepRatings(); len(ids) != 1 || ids[0] != old {
		t.Fatalf("old deep ratings: %v", ids)
	}
}
