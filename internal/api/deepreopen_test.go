package api_test

import (
	"testing"
)

// Books whose Deep Scan found Level 1 or 2 but were kept lower (while the review hid Level 1 and 2 parts) are
// listed for a second look and can be put back in the review; a Level 3 finding, a book already rated as high
// as the scan, and a scan nobody decided are left alone.
func TestDeepScanSecondLook(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	add := func(title string, found int, status string, newLevel any, rated int) int64 {
		id, _ := st.UpsertBook(title, "A. Writer", "", "")
		_, _ = st.DB.Exec(`UPDATE books SET spice_level = ? WHERE id = ?`, rated, id)
		_, _ = st.DB.Exec(`INSERT INTO deep_reads (book_id, status, checks, held, proposed_level, proposal, approved_by, new_level, prev_level)
			VALUES (?, ?, 1, 0, ?, '{"spice_level": 2}', 'admin', ?, 0)`, id, status, found, newLevel)
		return id
	}
	add("Eragon", 2, "declined", nil, 0)          // kept the old rating: listed
	add("Set Too Low", 1, "done", 0, 0)           // set to 0 below the scan's 1: listed
	add("Brave New World", 3, "declined", nil, 0) // a Level 3 finding showed its parts: left alone
	add("Fixed Since", 2, "declined", nil, 2)     // already rated 2 now: left alone
	_, out := admin.do("GET", "/api/admin/deep-scans/kept-below", nil, false)
	books, _ := out["books"].([]any)
	if len(books) != 2 {
		t.Fatalf("two books for a second look: %v", out)
	}
	if _, out = admin.do("POST", "/api/admin/deep-scans/kept-below", nil, true); out["reopened"] != float64(2) {
		t.Fatalf("both put back in the review: %v", out)
	}
	var held int
	_ = st.DB.Get(&held, `SELECT COUNT(*) FROM deep_reads WHERE held = 1`)
	if held != 2 {
		t.Fatalf("two scans wait for a decision again: %d", held)
	}
	if _, out = admin.do("GET", "/api/admin/deep-scans/kept-below", nil, false); len(out["books"].([]any)) != 0 {
		t.Fatalf("nothing left to look at again: %v", out)
	}
}
