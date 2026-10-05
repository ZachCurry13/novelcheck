package api_test

import (
	"fmt"
	"strings"
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

// An old held scan stays on the Deep Scan page however many newer scans
// finished since, and "Keep all" turns every held scan down at once.
func TestHeldScansAlwaysListed(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	four := 4
	id, _ := st.UpsertBook("Old Held", "Author", "", "")
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: new(int), Model: "gpt"})
	scan, _ := st.RequestDeepRead(id, "admin", "admin", "", true, 1000, 1, 1)
	_ = st.HoldDeepRead(scan, `[]`, store.Analysis{SpiceLevel: &four}, 4)
	for i := range 70 {
		b, _ := st.UpsertBook(fmt.Sprintf("Newer %d", i), "Author", "", "")
		d, _ := st.RequestDeepRead(b, "auto", "", "", true, 1000, 1, 1)
		_ = st.FinishDeepRead(d, "done", "[]", "", &four)
	}
	_, out := admin.do("GET", "/api/admin/deep-scans", nil, false)
	scans := out["scans"].([]any)
	if first := scans[0].(map[string]any); first["title"] != "Old Held" || first["held"] != true {
		t.Fatalf("held scan not listed first: %v", first["title"])
	}
	if res, r := admin.do("POST", "/api/admin/deep-scans/keep-all", nil, true); res.StatusCode != 200 || r["kept"].(float64) != 1 || st.HeldDeepReads() != 0 {
		t.Fatalf("keep all: %d %v", res.StatusCode, r)
	}
}

// "Accept all" applies every held raise, and deciding a scan clears its
// book's "suggests raising" notice; a notice can also be dismissed.
func TestAcceptAllAndReviewNotices(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	four := 4
	var scans []int64
	for _, title := range []string{"First", "Second", "Third"} {
		id, _ := st.UpsertBook(title, "Author", "", "")
		_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: new(int), Model: "gpt"})
		scan, _ := st.RequestDeepRead(id, "admin", "admin", "", true, 1000, 1, 1)
		_ = st.HoldDeepRead(scan, `[]`, store.Analysis{SpiceLevel: &four}, 4)
		st.Notify("warning", "deep-scan-review", fmt.Sprintf("Deep Scan suggests raising “%s” from Level 0 to Level 4. Review it before it applies.", title), "#/deepscan?review")
		scans = append(scans, scan)
	}
	unread := func() int {
		_, n, _ := st.Notifications(10)
		return n
	}
	if res, _ := admin.do("POST", fmt.Sprintf("/api/admin/deep-scans/%d/keep", scans[0]), nil, true); res.StatusCode != 200 || unread() != 2 {
		t.Fatalf("keep: %d, unread %d", res.StatusCode, unread())
	}
	res, out := admin.do("POST", "/api/admin/deep-scans/accept-all", nil, true)
	if res.StatusCode != 200 || out["accepted"].(float64) != 2 || st.HeldDeepReads() != 0 || unread() != 0 {
		t.Fatalf("accept all: %d %v, held %d, unread %d", res.StatusCode, out, st.HeldDeepReads(), unread())
	}
	if b, _ := st.BookByID(st.MatchBook("Third", "Author"), nil); *b.SpiceLevel != 4 {
		t.Fatalf("accepted level: %d", *b.SpiceLevel)
	}

	items, _, _ := st.Notifications(10)
	res, out = admin.do("POST", "/api/notifications/dismiss", map[string]any{"ids": []int64{items[0].ID, items[1].ID}}, true)
	if left, _, _ := st.Notifications(10); res.StatusCode != 200 || len(left) != 1 || len(out["items"].([]any)) != 1 {
		t.Fatalf("dismiss: %d, left %d", res.StatusCode, len(left))
	}
}

// An admin can pick a level of their own; each decision leaves a note for
// parents on the book with what the scan found.
func TestDeepScanSetLevelAndParentNote(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	zero, four := 0, 4
	id, _ := st.UpsertBook("Fade Out", "Author", "", "")
	_ = st.AddCopy(cat, id, "/calibre/fade.epub", "epub", "1")
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &zero, Model: "small-model"})
	scan, _ := st.RequestDeepRead(id, "admin", "admin", "", true, 1000, 1, 1)
	_ = st.HoldDeepRead(scan, `[{"label":"about 55% in","level":4,"note":"an intimate scene","scene":"Two adults kiss in a parked car; the chapter ends before anything is described.","from":100,"to":200}]`,
		store.Analysis{SpiceLevel: &four, Nudity: true, Model: store.DeepModelPrefix + "qwen2.5:7b"}, 4)

	if res, _ := admin.do("POST", fmt.Sprintf("/api/admin/deep-scans/%d/set", scan), map[string]int{"level": 9}, true); res.StatusCode != 400 {
		t.Fatalf("level 9: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", fmt.Sprintf("/api/admin/deep-scans/%d/set", scan), map[string]int{"level": 3}, true); res.StatusCode != 200 {
		t.Fatalf("set: %d", res.StatusCode)
	}
	b, _ := st.BookByID(id, nil)
	if *b.SpiceLevel != 3 || !b.Nudity || b.SpiceReason != "Set by a parent after a Deep Scan" {
		t.Fatalf("book: level %d nudity %v reason %q", *b.SpiceLevel, b.Nudity, b.SpiceReason)
	}
	parent, _ := st.UserByName("admin")
	notes, _ := st.BookNotes(id, parent)
	// Where and how high, not what happens (no spoilers in the book's notes).
	if len(notes) != 1 || notes[0].Visibility != "parents" || !strings.Contains(notes[0].Body, "about 55% in: Level") ||
		strings.Contains(notes[0].Body, "Two adults kiss") || !strings.Contains(notes[0].Body, "admin decided on Level 3") {
		t.Fatalf("parents' note: %+v", notes)
	}
	if st.LatestDeepRead(id).Held {
		t.Fatal("still held")
	}
}
