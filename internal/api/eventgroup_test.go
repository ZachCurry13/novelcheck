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
