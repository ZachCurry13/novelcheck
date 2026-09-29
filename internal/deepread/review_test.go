package deepread

import (
	"context"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A raise to 4 that rests on one part waits for an admin, even when it's
// only one level; two explicit parts don't need that (unless the jump is big).
func TestOnePassageIsHeld(t *testing.T) {
	three, one := 3, 1
	single := []llm.PartResult{{Level: 1}, {Level: 4}, {Level: 3}}
	double := []llm.PartResult{{Level: 4}, {Level: 4}}
	if !onePassage(&three, 4, single) || onePassage(&three, 4, double) || onePassage(&three, 3, single) || onePassage(nil, 4, single) {
		t.Fatal("one passage")
	}
	if bigJump(&three, 4) || !bigJump(&one, 4) {
		t.Fatal("big jump")
	}
}

// Notes say where each part is in the book, in words, and carry the scene
// description for parts at 3 or more.
func TestNotesHavePositionsAndScenes(t *testing.T) {
	parts := []Part{{Label: "a", Words: 100}, {Label: "b", Words: 50}, {Label: "c", Words: 70}}
	results := []llm.PartResult{{Level: 2, Note: "a kiss"}, {Level: 0}, {Level: 3, Evidence: "implied", Scene: "They go to bed; it cuts away."}}
	_, notes := combine(parts, results)
	if len(notes) != 2 || notes[0].From != 0 || notes[0].To != 100 || notes[0].Scene != "" ||
		notes[1].From != 150 || notes[1].To != 220 || notes[1].Scene == "" {
		t.Fatalf("notes: %+v", notes)
	}
}

// The second opinion can overrule: a scene it doesn't see described counts as 3.
func TestSecondOpinionCapsAtThree(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	ai := fakeAIWith(t, true, false)
	_ = st.SetSetting(store.KeyLLMBaseURL, ai.URL)
	_ = st.SetSetting(store.KeyLLMModel, "small-model")
	_ = st.SetSetting(store.KeyLocalContext, "32768")
	calibreDir := t.TempDir()
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	id, _ := st.UpsertBook("Fade Book", "Author", "", "")
	_ = st.AddCopy(cat, id, writeBook(t, calibreDir), "epub", "7")
	two := 2
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &two, Model: "gpt"})
	r := New(st, calibreDir)
	r.Queue([]int64{id}, "admin", "admin")
	next, _ := st.NextDeepRead()
	if err := r.scan(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	if b, _ := st.BookByID(id, nil); *b.SpiceLevel != 3 || st.LatestDeepRead(id).Held {
		t.Fatalf("level %d, want 3 and applied", *b.SpiceLevel)
	}

	// The reader finds a part by its words, or by its label.
	p, err := r.Passage(id, 3010, 3020, "")
	if err != nil || len(strings.Fields(p.Text)) != 10 || len(strings.Fields(p.Before)) != Around || p.Total != 6004 {
		t.Fatalf("passage: %v %d %d %d", err, len(strings.Fields(p.Text)), len(strings.Fields(p.Before)), p.Total)
	}
	if p, _ = r.Passage(id, 0, 0, "Chapter 2 (1/2)"); !strings.HasPrefix(p.Text, "Chapter 2") && !strings.HasPrefix(p.Text, "spicy scene number 1 ") {
		t.Fatalf("by label: %.40q", p.Text)
	}
}
