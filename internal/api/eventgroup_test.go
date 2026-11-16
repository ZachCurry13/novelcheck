package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A day's lists can be searched together: ?event=a,b gives the books of both, once each, list by list.
func TestEventsTogether(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	a, _, _ := st.CreateEvent("Thrillers · Nov 13", "", "admin", "", []store.EventEntry{{Title: "Gone Girl", Author: "Gillian Flynn", ASIN: "B1"}, {Title: "Shared One", Author: "Ann Both", ASIN: "B2"}})
	b, _, _ := st.CreateEvent("Mystery · Nov 13", "", "admin", "", []store.EventEntry{{Title: "Shared One", Author: "Ann Both", ASIN: "B2"}, {Title: "The Thursday Murder Club", Author: "Richard Osman", ASIN: "B3"}})
	titles := func(q string) []string {
		_, out := admin.do("GET", "/api/books?sort=list&event="+q, nil, false)
		var got []string
		for _, x := range out["books"].([]any) {
			got = append(got, x.(map[string]any)["title"].(string))
		}
		return got
	}
	both := titles(itoa(a) + "," + itoa(b))
	if len(both) != 3 || both[0] != "Gone Girl" || both[2] != "The Thursday Murder Club" {
		t.Fatalf("both lists, once each, in list order: %v", both)
	}
	if one := titles(itoa(a)); len(one) != 2 || one[0] != "Gone Girl" {
		t.Fatalf("one list still works: %v", one)
	}
}

// Several pepper levels at once: ?spice=0,2 gives the books at either level.
func TestSeveralPepperLevels(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	for i, lvl := range []int{0, 1, 2, 4} {
		id, _ := st.UpsertBook("Book "+itoa(int64(i)), "Ann Author", "", "")
		_ = st.AddCopy(cal, id, "/c/"+itoa(int64(i))+".epub", "epub", itoa(int64(10+i)))
		l := lvl
		_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &l, Model: "gpt"})
	}
	_, out := admin.do("GET", "/api/books?spice=0,2", nil, false)
	if n := len(out["books"].([]any)); n != 2 {
		t.Fatalf("levels 0 and 2: %d books", n)
	}
	_, out = admin.do("GET", "/api/books?spice=1", nil, false)
	if n := len(out["books"].([]any)); n != 1 {
		t.Fatalf("one level still works: %d", n)
	}
}

// Lists saved from one page share the event's name; it can be renamed for all of them at once.
func TestEventGroupName(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	book := []map[string]string{{"title": "Gone Girl", "author": "Gillian Flynn"}}
	_, a := admin.do("POST", "/api/events", map[string]any{"name": "Thrillers · Nov 13", "group": "Stuff Your Kindle · Nov 13", "books": book}, true)
	_, b := admin.do("POST", "/api/events", map[string]any{"name": "Mystery · Nov 13", "group": "Stuff Your Kindle · Nov 13", "books": book}, true)
	names := func() map[float64]string {
		_, evs := admin.doList("GET", "/api/events")
		out := map[float64]string{}
		for _, e := range evs {
			out[e["id"].(float64)] = e["group_name"].(string)
		}
		return out
	}
	if n := names(); n[a["id"].(float64)] != "Stuff Your Kindle · Nov 13" || n[b["id"].(float64)] != "Stuff Your Kindle · Nov 13" {
		t.Fatalf("saved together: %v", n)
	}
	if res, _ := admin.do("PUT", "/api/events/group", map[string]any{"ids": []float64{a["id"].(float64), b["id"].(float64)}, "name": "SYK Fall"}, true); res.StatusCode != 200 {
		t.Fatalf("rename: %d", res.StatusCode)
	}
	if n := names(); n[a["id"].(float64)] != "SYK Fall" || n[b["id"].(float64)] != "SYK Fall" {
		t.Fatalf("renamed together: %v", n)
	}
}

// A parent's default library for event claims: one of their own e-book libraries, never Calibre.
func TestClaimLibraryDefault(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	_, kindle := admin.do("POST", "/api/catalogs", map[string]string{"name": "Kindle (Amazon)", "source": "drive"}, true)
	if res, _ := admin.do("PUT", "/api/me/claim-library", map[string]int64{"catalog_id": cal}, true); res.StatusCode != 400 {
		t.Fatalf("Calibre as the claim library: %d", res.StatusCode)
	}
	if res, out := admin.do("PUT", "/api/me/claim-library", map[string]any{"catalog_id": kindle["id"]}, true); res.StatusCode != 200 {
		t.Fatalf("Kindle library: %d %v", res.StatusCode, out)
	}
	if _, me := admin.do("GET", "/api/me", nil, false); me["claim_catalog"] != kindle["id"] {
		t.Fatalf("kept on the account: %v", me["claim_catalog"])
	}
}
