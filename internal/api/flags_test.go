package api_test

import (
	"strconv"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestCustomFilters(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	one := 1
	clean, _ := st.UpsertBook("Clean Book", "A", "", "")
	_ = st.SaveAnalysis(clean, store.Analysis{SpiceLevel: &one, Model: "gpt"})
	if _, s := admin.do("GET", "/api/admin/status", nil, false); s["rerate_candidates"].(float64) != 0 {
		t.Fatalf("nothing to re-rate yet: %v", s["rerate_candidates"])
	}

	res, f := admin.do("POST", "/api/admin/flags", map[string]string{"label": "Heavy swearing / language", "description": "Frequent strong profanity"}, true)
	if res.StatusCode != 201 || f["key"] != "heavy_swearing_language" {
		t.Fatalf("add filter: %d %v", res.StatusCode, f)
	}
	if res, _ := admin.do("POST", "/api/admin/flags", map[string]string{"label": ""}, true); res.StatusCode != 400 {
		t.Fatalf("a filter needs a name: %d", res.StatusCode)
	}
	// Books rated before the new filter are offered for re-rating.
	if _, s := admin.do("GET", "/api/admin/status", nil, false); s["rerate_candidates"].(float64) != 1 {
		t.Fatalf("new filter should offer a re-rate: %v", s["rerate_candidates"])
	}

	// A rating that matches the filter; the Library can hide it.
	sweary, _ := st.UpsertBook("Sweary Book", "B", "", "")
	_ = st.SaveAnalysis(sweary, store.Analysis{SpiceLevel: &one, Model: "gpt", CustomFlags: []string{"heavy_swearing_language", "unknown"}})
	_, b := admin.do("GET", "/api/books/"+itoa(sweary), nil, false)
	if b["book"].(map[string]any)["custom_flags"] != "heavy_swearing_language" {
		t.Fatalf("book flags: %v", b["book"])
	}
	if _, out := admin.do("GET", "/api/books?exclude=flag:heavy_swearing_language", nil, false); out["total"].(float64) != 1 {
		t.Fatalf("hide by custom filter: %v", out["total"])
	}
	if _, out := admin.do("GET", "/api/books?exclude=flag:nope", nil, false); out["total"].(float64) != 2 {
		t.Fatalf("unknown filter hides nothing: %v", out["total"])
	}

	// A parent can mark it by hand too.
	res, _ = admin.do("PUT", "/api/books/"+itoa(clean)+"/verdict", map[string]any{"spice_level": 1, "custom_flags": []string{"heavy_swearing_language"}}, true)
	if res.StatusCode != 200 {
		t.Fatalf("manual verdict: %d", res.StatusCode)
	}
	if _, out := admin.do("GET", "/api/books?exclude=flag:heavy_swearing_language", nil, false); out["total"].(float64) != 0 {
		t.Fatalf("hand-marked book should hide too: %v", out["total"])
	}

	// Renaming alone doesn't ask for a re-rate; new instructions do.
	id := strconv.Itoa(int(f["id"].(float64)))
	before := st.FlagsVersion()
	if res, _ := admin.do("PUT", "/api/admin/flags/"+id, map[string]string{"label": "Swearing", "description": "Frequent strong profanity"}, true); res.StatusCode != 200 || st.FlagsVersion() != before {
		t.Fatalf("rename: %d, version %d -> %d", res.StatusCode, before, st.FlagsVersion())
	}
	if admin.do("PUT", "/api/admin/flags/"+id, map[string]string{"label": "Swearing", "description": "Any swearing"}, true); st.FlagsVersion() != before+1 {
		t.Fatal("new instructions should ask for a re-rate")
	}
	if res, _ := admin.do("DELETE", "/api/admin/flags/"+id, nil, true); res.StatusCode != 200 {
		t.Fatalf("delete: %d", res.StatusCode)
	}
	if list, _ := st.CustomFlags(); len(list) != 0 {
		t.Fatalf("still listed: %v", list)
	}
	if _, out := admin.do("GET", "/api/books?exclude=flag:heavy_swearing_language", nil, false); out["total"].(float64) != 2 {
		t.Fatalf("marks go with the filter: %v", out["total"])
	}
	for i := 0; i < store.MaxCustomFlags; i++ {
		admin.do("POST", "/api/admin/flags", map[string]string{"label": "Filter"}, true)
	}
	if res, _ := admin.do("POST", "/api/admin/flags", map[string]string{"label": "One too many"}, true); res.StatusCode != 400 {
		t.Fatalf("limit: %d", res.StatusCode)
	}
	if list, _ := st.CustomFlags(); list[1].Key != "filter_2" {
		t.Fatalf("keys stay unique: %v", list[1])
	}
}
