package api_test

import (
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Sharing a reading profile: private until switched on; what others see is
// limited to what they could see themselves; a kid's sharing is a parent's choice.
func TestReadersSharing(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]string{"username": "mom", "password": "editorpass1", "role": "editor"}, true)
	_, kidU := admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kidID := int64(kidU["id"].(float64))
	mom := login(t, srv, "mom", "editorpass1")
	kid := login(t, srv, "kid", "kidpass12")

	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	zero := 0
	safe, _ := st.UpsertBook("Safe Book", "A Author", "", "")
	_ = st.AddCopy(cat, safe, "/c/safe.epub", "epub", "1")
	_ = st.SaveAnalysis(safe, store.Analysis{SpiceLevel: &zero, Model: "gpt"})
	_ = st.SetApproved(safe, true, "mom") // a parent approved it, so a kid may see it
	unrated, _ := st.UpsertBook("Unrated Book", "B Author", "", "")
	_ = st.AddCopy(cat, unrated, "/c/unrated.epub", "epub", "2")

	now := time.Now().Unix()
	_, err := st.SaveKOReaderBooks(1, []store.KOReaderBook{
		{Title: "Safe Book", Authors: "A Author", MD5: "m1", Pages: 100, Percent: 0.5, ReadSeconds: 1200, LastOpen: now - 60, Highlights: 4, Notes: 2},
		{Title: "Unrated Book", Authors: "B Author", MD5: "m2", Pages: 100, Percent: 0.2, ReadSeconds: 600, LastOpen: now - 3600},
		{Title: "My Private Journal", Authors: "", MD5: "m3", Pages: 50, Percent: 0.1, ReadSeconds: 300, LastOpen: now - 7200},
	})
	if err != nil {
		t.Fatal(err)
	}
	hr := now / 3600 * 3600
	_ = st.SaveReadingHours(1, []store.ReadingHour{
		{Title: "Safe Book", Authors: "A Author", MD5: "m1", Hour: hr, Seconds: 1200, Pages: 20},
		{Title: "Unrated Book", Authors: "B Author", MD5: "m2", Hour: hr - 3600, Seconds: 600, Pages: 10},
		{Title: "My Private Journal", Authors: "", MD5: "m3", Hour: hr - 7200, Seconds: 300, Pages: 5},
	})

	readers := func(c *client) []any {
		_, out := c.do("GET", "/api/readers", nil, false)
		list, _ := out["readers"].([]any)
		return list
	}
	byName := func(list []any, name string) map[string]any {
		for _, x := range list {
			if m := x.(map[string]any); m["username"] == name {
				return m
			}
		}
		return nil
	}
	titles := func(c *client, id int64) (int, []string) {
		res, out := c.do("GET", "/api/readers/"+itoa(id)+"/stats", nil, false)
		var names []string
		if bs, ok := out["books"].([]any); ok {
			for _, b := range bs {
				names = append(names, b.(map[string]any)["title"].(string))
			}
		}
		return res.StatusCode, names
	}

	// Private by default: nobody sees the admin's reading, and asking looks like "no such reader".
	if len(readers(kid)) != 0 {
		t.Fatalf("nothing is shared yet: %v", readers(kid))
	}
	if got := readers(mom); len(got) != 1 || byName(got, "kid") == nil || byName(got, "admin") != nil { // a parent sees their kid, never an unshared adult
		t.Fatalf("mom sees only the kid: %v", got)
	}
	if code, _ := titles(mom, 1); code != 404 {
		t.Fatalf("a private profile: %d", code)
	}
	if code, names := titles(admin, 1); code != 200 || len(names) != 3 {
		t.Fatalf("your own stats are all there: %d %v", code, names)
	}

	// Sharing on: the family sees it, limited to what each may see.
	if res, _ := admin.do("PUT", "/api/me/sharing", map[string]bool{"profile": true, "current": false}, true); res.StatusCode != 200 {
		t.Fatalf("share: %d", res.StatusCode)
	}
	list := readers(mom)
	r0 := byName(list, "admin")
	if len(list) != 2 || r0 == nil || r0["month_sec"].(float64) <= 0 || r0["now"] != nil {
		t.Fatalf("reader row (not showing what they're reading now): %v", r0)
	}
	code, names := titles(mom, 1)
	if code != 200 || len(names) != 2 { // not the personal file that isn't in the library
		t.Fatalf("mom sees library books only: %d %v", code, names)
	}
	_, out := mom.do("GET", "/api/readers/1/stats", nil, false)
	for _, b := range out["books"].([]any) {
		if m := b.(map[string]any); m["highlights"].(float64) != 0 || m["notes"].(float64) != 0 {
			t.Fatalf("highlights and notes are personal: %v", m)
		}
	}
	if _, names := titles(kid, 1); len(names) != 1 || names[0] != "Safe Book" { // the kid's rules: nothing unrated
		t.Fatalf("a kid sees only what a kid may see: %v", names)
	}
	_, kout := kid.do("GET", "/api/readers/1/stats", nil, false)
	if sb := kout["books"].([]any)[0].(map[string]any); sb["spice"] != nil {
		t.Fatalf("kids never see pepper levels: %v", sb)
	}

	// "Reading now" is a separate choice, and still follows the viewer's rules.
	admin.do("PUT", "/api/me/sharing", map[string]bool{"profile": true, "current": true}, true)
	if now := byName(readers(mom), "admin")["now"]; now == nil || now.(map[string]any)["title"] != "Safe Book" {
		t.Fatalf("reading now: %v", now)
	}
	admin.do("PUT", "/api/me/sharing", map[string]bool{"profile": false, "current": true}, true) // current needs profile
	if got := readers(mom); byName(got, "admin") != nil {
		t.Fatalf("switched off again: %v", got)
	}

	// A kid cannot share on their own; a parent decides, and always sees their kids.
	if res, _ := kid.do("PUT", "/api/me/sharing", map[string]bool{"profile": true}, true); res.StatusCode != 403 {
		t.Fatalf("a kid switching it on: %d", res.StatusCode)
	}
	if kids := readers(mom); len(kids) != 1 || kids[0].(map[string]any)["username"] != "kid" || kids[0].(map[string]any)["shared"] != false {
		t.Fatalf("a parent sees their kid even when it is not shared: %v", kids)
	}
	if len(readers(admin)) != 1 {
		t.Fatalf("the admin is a parent too: %v", readers(admin))
	}
	if res, _ := mom.do("PUT", "/api/admin/users/"+itoa(kidID)+"/sharing", map[string]bool{"profile": true}, true); res.StatusCode != 200 {
		t.Fatalf("a parent shares a kid's profile: %d", res.StatusCode)
	}
	if got := readers(admin); len(got) != 1 || got[0].(map[string]any)["shared"] != true {
		t.Fatalf("shared kid: %v", got)
	}
	if res, _ := kid.do("PUT", "/api/admin/users/"+itoa(kidID)+"/sharing", map[string]bool{"profile": false}, true); res.StatusCode == 200 {
		t.Fatalf("a kid cannot use the parents' control: %d", res.StatusCode)
	}
	_, ks := mom.do("GET", "/api/admin/kids-sharing", nil, false)
	if kids, _ := ks["kids"].([]any); len(kids) != 1 {
		t.Fatalf("kids list: %v", ks)
	}
}
