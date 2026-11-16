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
