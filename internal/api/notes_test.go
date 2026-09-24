package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestNotesAndAgeAPI(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]string{"username": "wife", "password": "editorpass1", "role": "editor"}, true)
	res, kid := admin.do("POST", "/api/admin/users", map[string]any{"username": "tween", "password": "kidpass12", "age_level": 2}, true)
	if res.StatusCode != 201 || kid["age_level"].(float64) != 2 || kid["hide_unrated"] != true {
		t.Fatalf("create age-group kid: %d %v", res.StatusCode, kid)
	}
	ed := login(t, srv, "wife", "editorpass1")
	tw := login(t, srv, "tween", "kidpass12")

	id, _ := st.UpsertBook("Mystery Book", "A. Writer", "", "")
	one := 1 // Sweet Romance: fine for middle grade
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &one})
	path := "/api/books/" + itoa(id)

	// Editor rates it for teens: the middle-grade kid can no longer see it.
	if res, b := ed.do("PUT", path+"/age", map[string]int{"age_level": 3}, true); res.StatusCode != 200 || b["age_set_by"] != "wife" {
		t.Fatalf("set age: %d %v", res.StatusCode, b)
	}
	if res, _ := tw.do("GET", path, nil, false); res.StatusCode != 404 {
		t.Fatalf("book above the kid's age group must be hidden: %d", res.StatusCode)
	}
	ed.do("PUT", path+"/age", map[string]int{"age_level": 2}, true)

	// Notes: one for everyone, one for parents only.
	if res, out := ed.do("POST", path+"/notes", map[string]string{"body": "Great for 10+.", "visibility": "everyone"}, true); res.StatusCode != 200 || len(out["notes"].([]any)) != 1 {
		t.Fatalf("add note: %d %v", res.StatusCode, out)
	}
	_, out := ed.do("POST", path+"/notes", map[string]string{"body": "Sad ending, talk about it.", "visibility": "parents"}, true)
	parentNote := out["notes"].([]any)[1].(map[string]any)
	if res, out := tw.do("GET", path, nil, false); res.StatusCode != 200 || len(out["notes"].([]any)) != 1 {
		t.Fatalf("kid should see only the shared note: %d %v", res.StatusCode, out["notes"])
	}
	if res, _ := tw.do("POST", path+"/notes", map[string]string{"body": "hi", "visibility": "everyone"}, true); res.StatusCode != 403 {
		t.Fatalf("kids can't write notes: %d", res.StatusCode)
	}
	noteID := itoa(int64(parentNote["id"].(float64)))
	// The admin may edit anyone's note; a second editor may not.
	admin.do("POST", "/api/admin/users", map[string]string{"username": "dad", "password": "editorpass2", "role": "editor"}, true)
	dad := login(t, srv, "dad", "editorpass2")
	if res, _ := dad.do("PUT", "/api/notes/"+noteID, map[string]string{"body": "x", "visibility": "everyone"}, true); res.StatusCode != 403 {
		t.Fatalf("editing someone else's note: %d", res.StatusCode)
	}
	if res, _ := ed.do("DELETE", "/api/notes/"+noteID, nil, true); res.StatusCode != 200 {
		t.Fatalf("delete own note: %d", res.StatusCode)
	}
	if _, out := admin.do("GET", "/api/books?age=2", nil, false); out["total"].(float64) != 1 {
		t.Fatalf("age filter: %v", out["total"])
	}
}
