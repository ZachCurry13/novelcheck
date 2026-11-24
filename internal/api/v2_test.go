package api_test

import (
	"net/http"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Home shows a person their own things; a parent also gets the asks waiting for them; an admin the inbox and health.
func TestHomeForEachKindOfPerson(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	_, ah := admin.do("GET", "/api/home", nil, false)
	if ah["inbox"] == nil || ah["health"] == nil || ah["requests"] == nil {
		t.Fatalf("an admin gets the inbox, health and asks: %v", ah)
	}
	_, kh := kid.do("GET", "/api/home", nil, false)
	if kh["inbox"] != nil || kh["health"] != nil || kh["requests"] != nil || kh["next_up"] == nil {
		t.Fatalf("a kid gets only their own things: %v", kh)
	}
	if res, _ := kid.do("GET", "/api/admin/health", nil, false); res.StatusCode != http.StatusForbidden {
		t.Fatalf("health is for admins: %d", res.StatusCode)
	}
	_ = st
}

// A kid asks for a book; a parent says yes: the book is OK for kids and on the kid's Nightstand. A book not in the library is approved and waits.
func TestRequestsAndApprovals(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleQueue, "true")
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	have, _ := st.UpsertBook("Divergent", "Veronica Roth", "", "")
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	_ = st.AddCopy(cat, have, "/c/divergent.epub", "epub", "1")

	if res, out := kid.do("POST", "/api/requests", map[string]any{"title": "Divergent", "author": "Veronica Roth"}, true); res.StatusCode != 200 {
		t.Fatalf("ask: %d %v", res.StatusCode, out)
	}
	if res, _ := kid.do("POST", "/api/requests", map[string]any{"title": "Divergent"}, true); res.StatusCode != 400 {
		t.Fatalf("the same ask twice: %d", res.StatusCode)
	}
	kid.do("POST", "/api/requests", map[string]any{"title": "A Book Nobody Has", "author": "Nobody"}, true)
	_, open := admin.do("GET", "/api/requests", nil, false)
	list, _ := open["open"].([]any)
	if len(list) != 2 {
		t.Fatalf("a parent sees both asks: %v", open["open"])
	}
	if res, _ := kid.do("POST", "/api/requests/"+itoa(int64(list[0].(map[string]any)["id"].(float64)))+"/decide", map[string]any{"action": "approve"}, true); res.StatusCode != 403 {
		t.Fatalf("a kid can't answer: %d", res.StatusCode)
	}
	first, second := int64(list[0].(map[string]any)["id"].(float64)), int64(list[1].(map[string]any)["id"].(float64))
	if res, out := admin.do("POST", "/api/requests/"+itoa(first)+"/decide", map[string]any{"action": "approve"}, true); res.StatusCode != 200 {
		t.Fatalf("approve: %d %v", res.StatusCode, out)
	}
	kidUser, _ := st.UserByName("kid")
	if items, _ := st.ListQueue(kidUser.ID, false); len(items) != 1 {
		t.Fatalf("the approved book should be on the kid's Nightstand: %v", items)
	}
	if b, _ := st.BookByID(have, nil); !b.Approved {
		t.Fatal("the book should be marked OK for kids")
	}
	admin.do("POST", "/api/requests/"+itoa(second)+"/decide", map[string]any{"action": "approve"}, true)
	_, open = admin.do("GET", "/api/requests", nil, false)
	if list, _ = open["open"].([]any); len(list) != 1 || list[0].(map[string]any)["status"] != "approved" {
		t.Fatalf("the book to get stays in the inbox: %v", open["open"])
	}
}

// "I disagree" is kept with what the AI said, and the Accuracy page scores it.
func TestDisagreeAndAccuracy(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	id, _ := st.UpsertBook("Fourth Wing", "Rebecca Yarros", "", "")
	lvl := 3
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &lvl, Model: "test-model"})
	if res, _ := kid.do("POST", "/api/books/"+itoa(id)+"/disagree", map[string]any{"level": 4}, true); res.StatusCode != 403 {
		t.Fatalf("a kid doesn't give levels: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/books/"+itoa(id)+"/disagree", map[string]any{"level": 9}, true); res.StatusCode != 400 {
		t.Fatalf("level 9: %d", res.StatusCode)
	}
	if res, out := admin.do("POST", "/api/books/"+itoa(id)+"/disagree", map[string]any{"level": 4, "note": "it is open door"}, true); res.StatusCode != 200 {
		t.Fatalf("disagree: %d %v", res.StatusCode, out)
	}
	_, acc := admin.do("GET", "/api/admin/accuracy", nil, false)
	models, _ := acc["models"].([]any)
	if len(models) < 2 {
		t.Fatalf("models: %v", acc)
	}
	all := models[0].(map[string]any)
	if all["model"] != "All" || all["n"] != float64(1) || all["exact"] != float64(0) || all["within1"] != float64(1) || all["lower"] != float64(1) {
		t.Fatalf("the AI said 3, the person 4: %v", all)
	}
	recent, _ := acc["recent"].([]any)
	fid := int64(recent[0].(map[string]any)["id"].(float64))
	if res, _ := admin.do("POST", "/api/admin/accuracy/"+itoa(fid)+"/use", nil, true); res.StatusCode != 200 {
		t.Fatalf("use: %d", res.StatusCode)
	}
	if b, _ := st.BookByID(id, nil); b.SpiceLevel == nil || *b.SpiceLevel != 4 {
		t.Fatalf("the book should have the person's level: %v", b.SpiceLevel)
	}
}
