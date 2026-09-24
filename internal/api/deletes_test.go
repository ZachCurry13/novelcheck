package api_test

import (
	"sync/atomic"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestDeleteRequests(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	kindle, _ := st.EnsureCatalog("Wife's Kindle", "drive")
	bad, _ := st.UpsertBook("Too Spicy", "A", "", "")
	_ = st.AddCopy(cal, bad, "/c/a.epub", "epub", "41")
	_ = st.SaveAnalysis(bad, store.Analysis{Classification: "No Spice"})
	onKindle, _ := st.UpsertBook("Kindle Only", "B", "", "")
	_ = st.AddCopy(kindle, onKindle, "documents/k.azw3", "azw3", "B00X")
	keep, _ := st.UpsertBook("Keep Me", "C", "", "")
	_ = st.AddCopy(cal, keep, "/c/k.epub", "epub", "42")

	admin.do("POST", "/api/admin/users", map[string]string{"username": "wife", "password": "wifepass12", "role": "editor"}, true)
	wife := login(t, srv, "wife", "wifepass12")
	for _, id := range []int64{bad, onKindle, keep} {
		if res, out := wife.do("POST", "/api/books/"+itoa(id)+"/delete-request", map[string]string{"reason": "not for us"}, true); res.StatusCode != 200 || out["request"] == nil {
			t.Fatalf("request %d: %d %v", id, res.StatusCode, out)
		}
	}
	// Asking twice updates the reason instead of duplicating.
	wife.do("POST", "/api/books/"+itoa(bad)+"/delete-request", map[string]string{"reason": "way too explicit"}, true)
	if _, b := wife.do("GET", "/api/books/"+itoa(bad), nil, false); b["my_delete_request"].(map[string]any)["reason"] != "way too explicit" {
		t.Fatalf("my request: %v", b["my_delete_request"])
	}
	// Editors can't review; admins see one group per book.
	if res, _ := wife.do("GET", "/api/admin/delete-requests", nil, false); res.StatusCode != 403 {
		t.Fatalf("editor review: %d", res.StatusCode)
	}
	_, list := admin.do("GET", "/api/admin/delete-requests", nil, false)
	if len(list["pending"].([]any)) != 3 {
		t.Fatalf("pending: %v", list["pending"])
	}
	items, _, _ := st.Notifications(10)
	if len(items) != 1 || items[0].Source != "delete-requests" {
		t.Fatalf("admin should be notified once: %+v", items)
	}

	// Delete from Calibre: only the Calibre book is removed; the Kindle one stays pending.
	var removed atomic.Int32
	cs := fakeCalibre(t, map[string]string{"41": "Too Spicy", "42": "Keep Me"}, &removed)
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)
	res, out := admin.do("POST", "/api/admin/delete-requests/decide", map[string]any{"book_ids": []int64{bad, onKindle}, "action": "delete"}, true)
	if res.StatusCode != 200 || out["removed"].(float64) != 1 || out["not_in_calibre"].(float64) != 1 || removed.Load() != 1 {
		t.Fatalf("delete: %d %v removed=%d", res.StatusCode, out, removed.Load())
	}
	admin.do("POST", "/api/admin/delete-requests/decide", map[string]any{"book_ids": []int64{keep}, "action": "dismiss"}, true)
	admin.do("POST", "/api/admin/delete-requests/decide", map[string]any{"book_ids": []int64{onKindle}, "action": "done"}, true)
	_, list = admin.do("GET", "/api/admin/delete-requests", nil, false)
	if len(list["pending"].([]any)) != 0 || len(list["recent"].([]any)) != 3 {
		t.Fatalf("after decisions: %v", list)
	}
	if _, unread, _ := st.Notifications(10); unread != 0 {
		t.Fatal("notice should clear when nothing is pending")
	}
}
