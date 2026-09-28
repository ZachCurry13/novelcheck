package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestContentAPI(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")

	if res, cat := admin.do("GET", "/api/content", nil, false); res.StatusCode != 200 || len(cat["groups"].([]any)) != 5 ||
		len(cat["presets"].(map[string]any)["young"].([]any)) != 30 {
		t.Fatalf("catalog: %d %v", res.StatusCode, cat)
	}

	// Kids' rules: items, groups and custom filters; anything else is refused.
	_, kid := admin.do("POST", "/api/admin/users", map[string]any{"username": "kiddo", "password": "kidpass12"}, true)
	path := "/api/admin/users/" + itoa(int64(kid["id"].(float64)))
	kid["hidden_content"] = []string{"g:gore", "bogus"}
	if res, _ := admin.do("PUT", path, kid, true); res.StatusCode != 400 {
		t.Fatalf("a bogus rule must be refused: %d", res.StatusCode)
	}
	kid["hidden_content"] = []string{"g:gore", "murder", "flag:swearing"}
	if res, out := admin.do("PUT", path, kid, true); res.StatusCode != 200 || len(out["hidden_content"].([]any)) != 3 {
		t.Fatalf("save rules: %d %v", res.StatusCode, out)
	}

	// Edit rating: ticked items are the parent's; ticking nothing on a book
	// never checked leaves it for the AI to check.
	id, _ := st.UpsertBook("Unchecked", "A", "", "")
	verdict := "/api/books/" + itoa(id) + "/verdict"
	if res, b := admin.do("PUT", verdict, map[string]any{"spice_level": 1, "content": []string{}}, true); res.StatusCode != 200 || b["content_version"].(float64) != 0 {
		t.Fatalf("empty ticks: %d %v", res.StatusCode, b)
	}
	if res, b := admin.do("PUT", verdict, map[string]any{"spice_level": 1, "content": []string{"war", "blood"}}, true); res.StatusCode != 200 ||
		b["content_version"].(float64) != 1 || b["content"] != "blood:parent,war:parent" && b["content"] != "war:parent,blood:parent" {
		t.Fatalf("ticked items: %d %v", res.StatusCode, b)
	}
	if res, b := admin.do("PUT", verdict, map[string]any{"spice_level": 1, "content": []string{}}, true); res.StatusCode != 200 || b["content"] != "" {
		t.Fatalf("clearing a checked book's items: %d %v", res.StatusCode, b)
	}
	if b, _ := st.BookByID(id, nil); !store.ContentChecked(b) {
		t.Fatal("a checked book stays checked")
	}
}
