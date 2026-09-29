package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The Deep Scan machine: re-rating on it needs it set up and switched on;
// low-confidence ratings show up under "Needs review".
func TestDeepMachineAndNeedsReview(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Unclear Book", "Author", "", "")
	_ = st.AddCopy(cal, id, "/c/u.epub", "epub", "1")
	two := 2
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &two, Confidence: "low", ContentSource: store.SourceAI, Model: "llama3.2"})

	if bs, total, _ := st.ListBooks(store.BookFilter{Spice: "review"}, nil); total != 1 || bs[0].Confidence != "low" {
		t.Fatalf("needs review: %d", total)
	}
	if res, _ := admin.do("POST", "/api/books/"+itoa(id)+"/rerate-big", nil, true); res.StatusCode != 400 {
		t.Fatalf("no Deep Scan machine: %d", res.StatusCode)
	}
	_ = st.SetSetting(store.KeyDeepEnabled, "true")
	_ = st.SetSetting(store.KeyDeepBaseURL, "http://127.0.0.1:1/v1") // nothing listens there
	_ = st.SetSetting(store.KeyDeepModels, "qwen2.5:14b")
	if res, _ := admin.do("POST", "/api/books/"+itoa(id)+"/rerate-big", nil, true); res.StatusCode != 409 {
		t.Fatalf("machine off: %d", res.StatusCode)
	}
	_, out := admin.do("GET", "/api/admin/aitools", nil, false)
	if out["deep_machine"] != true || out["deep_away"] != true {
		t.Fatalf("aitools: %v", out)
	}

	// A Deep Scan or a parent's rating is sure.
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &two, Model: store.DeepModelPrefix + "qwen2.5:14b"})
	if b, _ := st.BookByID(id, nil); b.Confidence != "high" {
		t.Fatalf("deep scan confidence: %q", b.Confidence)
	}
}
