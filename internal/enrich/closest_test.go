package enrich

import "testing"

func TestClosestTitle(t *testing.T) {
	cornwell := []Found{
		{Title: "Unnatural Exposure", Author: "Patricia Cornwell"},
		{Title: "Postmortem", Author: "Patricia Cornwell"},
		{Title: "Point of Origin: A Scarpetta Novel", Author: "Patricia Cornwell"},
		{Title: "Cruel and Unusual", Author: "Patricia Cornwell"},
		{Title: "Unnatural Causes", Author: "P. D. James"},
		{Title: "The Last Precinct", Author: "Patricia Cornwell"},
	}
	cases := []struct{ title, want string }{
		{"Unnatural.Exprosure", "Unnatural Exposure"},
		{"unnatural exposure", "Unnatural Exposure"},
		{"Point_of_Orgin", "Point of Origin"},
		{"Last.Precinct", "The Last Precinct"},
		{"Cruel.&.Unusal", "Cruel and Unusual"},
		{"Body of Evidence", ""}, // not in the list
		{"Unnatural", ""},        // too far from any
	}
	for _, c := range cases {
		got, ok := ClosestTitle(c.title, "Patricia Cornwell", cornwell)
		if (c.want == "" && ok) || (c.want != "" && (!ok || got.Title != c.want)) {
			t.Errorf("%q: got %+v %v, want %q", c.title, got, ok, c.want)
		}
	}
	if _, ok := ClosestTitle("Unnatural Causes", "Patricia Cornwell", cornwell); ok {
		t.Error("another author's book was taken")
	}
}
