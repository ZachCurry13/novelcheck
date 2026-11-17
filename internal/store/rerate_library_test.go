package store_test

import (
	"slices"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The re-rate count for "my libraries" leaves out books only on Discover lists and events, which the Library
// and the other counts never show; the count for all of them still has them.
func TestRerateLibraryLeavesOutDiscoverOnly(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	lib, _ := st.EnsureCatalog("Calibre Main", "calibre")
	disc, _ := st.EnsureCatalog(store.DiscoverCatalog, "custom")
	ev, _ := st.EnsureCatalog(store.EventsCatalog, "custom")
	mine, _ := st.UpsertBook("Mine", "A", "", "")
	listed, _ := st.UpsertBook("Listed", "B", "", "")
	event, _ := st.UpsertBook("Event Book", "C", "", "")
	both, _ := st.UpsertBook("Owned And Listed", "D", "", "")
	_ = st.AddCopy(lib, mine, "/c/mine.epub", "epub", "1")
	_ = st.AddCopy(disc, listed, "discover:1", "", "1")
	_ = st.AddCopy(ev, event, "event:1", "", "1")
	_ = st.AddCopy(lib, both, "/c/both.epub", "epub", "2")
	_ = st.AddCopy(disc, both, "discover:2", "", "2")
	one := 1
	for _, id := range []int64{mine, listed, event, both} {
		_ = st.SaveAnalysis(id, store.Analysis{ContentSource: store.SourceAI, SpiceLevel: &one, Model: "gpt"})
	}
	// Rated under older rules.
	_, _ = d.Exec(`UPDATE books SET rules_version = 1`)
	all, _ := st.RerateCandidates()
	inLib, _ := st.RerateCandidatesLibrary()
	if len(all) != 4 {
		t.Fatalf("all four are offered: %v", all)
	}
	if len(inLib) != 2 || !slices.Contains(inLib, mine) || !slices.Contains(inLib, both) {
		t.Fatalf("only the library's books: %v", inLib)
	}
}
