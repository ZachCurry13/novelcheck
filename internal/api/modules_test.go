package api_test

import (
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestFeatureSwitches(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")

	// Everything is on by default, and every user learns which features are on.
	_, me := admin.do("GET", "/api/me", nil, false)
	mods, _ := me["modules"].(map[string]any)
	for _, m := range []string{"queue", "send_to_kindle", "koreader", "import", "parents"} {
		if mods[m] != true {
			t.Fatalf("%s should start on: %v", m, me["modules"])
		}
	}
	if me["username"] != "admin" {
		t.Fatalf("user fields must stay at the top level: %v", me)
	}
	if res, _ := admin.do("PUT", "/api/admin/settings", map[string]string{"module_queue": "maybe"}, true); res.StatusCode != 400 {
		t.Fatalf("switches take true/false: %d", res.StatusCode)
	}

	id, _ := st.UpsertBook("Queued Book", "A", "", "")
	zero := 0
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &zero, Model: "gpt"})
	if res, _ := admin.do("POST", "/api/queue", map[string]int64{"book_id": id}, true); res.StatusCode != 201 {
		t.Fatalf("enqueue: %d", res.StatusCode)
	}

	// KOReader off: Start Reading still works, just without the sync flag,
	// and parents see who started what.
	admin.do("PUT", "/api/me/delivery", map[string]string{"delivery_method": "koreader"}, true)
	admin.do("PUT", "/api/admin/settings", map[string]string{"module_koreader": "false"}, true)
	items, _ := st.ListQueue(1, false)
	if _, out := admin.do("POST", "/api/queue/"+itoa(items[0].ID)+"/start", nil, true); !strings.Contains(out["delivery_note"].(string), "turned off") {
		t.Fatalf("delivery with KOReader off: %v", out)
	}
	if list, _, _ := st.Notifications(5); len(list) != 1 || list[0].Source != "reading" || !strings.Contains(list[0].Message, "Queued Book") {
		t.Fatalf("started-reading notice: %+v", list)
	}

	// Queue and import off: their API calls are refused.
	admin.do("PUT", "/api/admin/settings", map[string]string{"module_queue": "false", "module_import": "false"}, true)
	if res, out := admin.do("GET", "/api/queue", nil, false); res.StatusCode != 403 || !strings.Contains(out["error"].(string), "turned off") {
		t.Fatalf("queue off: %d %v", res.StatusCode, out)
	}
	if res, _ := admin.do("POST", "/api/import/drive", map[string]any{"catalog_name": "K", "books": []any{}}, true); res.StatusCode != 403 {
		t.Fatalf("import off: %d", res.StatusCode)
	}
	if _, me = admin.do("GET", "/api/me", nil, false); me["modules"].(map[string]any)["queue"] != false {
		t.Fatalf("me reports queue off: %v", me["modules"])
	}
}
