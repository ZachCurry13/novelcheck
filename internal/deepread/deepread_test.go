package deepread

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/epub"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func words(n int, w string) string { return strings.TrimSpace(strings.Repeat(w+" ", n)) }

func TestSplitKeepsChaptersAndCutsLongOnes(t *testing.T) {
	secs := []epub.Section{
		{Title: "Chapter 1", Text: words(40, "a")},
		{Title: "Chapter 2", Text: words(40, "b")},
		{Title: "Chapter 3", Text: words(250, "c")}, // longer than a part
		{Title: "", Text: words(30, "d")},
	}
	parts := Split(secs, 100)
	var labels []string
	for _, p := range parts {
		labels = append(labels, p.Label)
		if p.Words > 100 {
			t.Fatalf("part too big: %d", p.Words)
		}
	}
	want := []string{"Chapter 1 – Chapter 2", "Chapter 3 (1/3)", "Chapter 3 (2/3)", "Chapter 3 (3/3)", "about 91% in"}
	if strings.Join(labels, "|") != strings.Join(want, "|") {
		t.Fatalf("labels %q, want %q", labels, want)
	}
	e := EstimateParts(parts)
	if e.Words != 360 || e.Parts != 5 || e.Tokens != 360*4/3+5*promptTokens+wrapUpTokens+e.Output {
		t.Fatalf("estimate: %+v", e)
	}
}

// fakeAI answers each part by what's in it, and the final wrap-up.
func fakeAI(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body := string(raw)
		answer := `{"level": 1, "note": "A sweet first meeting.", "lgbtq_content": false}`
		switch {
		case strings.Contains(body, "spice_reason") && strings.Contains(body, "Highest pepper level"):
			answer = `{"spice_reason": "Explicit scene in Chapter 2", "summary_verdict": "Mostly sweet, with one explicit chapter."}`
		case strings.Contains(body, "spicy"):
			answer = `{"level": 4, "note": "An explicit scene.", "nudity": true}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": answer}}},
			"usage":   map[string]int{"prompt_tokens": 1000, "completion_tokens": 50},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeBook(t *testing.T, dir string) string {
	p := filepath.Join(dir, "Author", "Book (1)", "book.epub")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	f, _ := os.Create(p)
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="content.opf"/></rootfiles></container>`,
		"content.opf": `<package><manifest><item id="a" href="a.xhtml" media-type="application/xhtml+xml"/>
			<item id="b" href="b.xhtml" media-type="application/xhtml+xml"/></manifest>
			<spine><itemref idref="a"/><itemref idref="b"/></spine></package>`,
		"a.xhtml": "<html><body><h1>Chapter 1</h1><p>" + words(3000, "sweet") + "</p></body></html>",
		"b.xhtml": "<html><body><h1>Chapter 2</h1><p>" + words(3000, "spicy") + "</p></body></html>",
	} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = f.Close()
	return p
}

func TestScanRaisesRatingAndLogsIt(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	ai := fakeAI(t)
	_ = st.SetSetting(store.KeyLLMBaseURL, ai.URL)
	_ = st.SetSetting(store.KeyLLMModel, "big-model")
	calibreDir := t.TempDir()
	path := writeBook(t, calibreDir)
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	id, _ := st.UpsertBook("Mixed Book", "Author", "", "")
	_ = st.AddCopy(cat, id, path, "epub", "5")
	two := 2
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &two, Model: "gpt", SummaryVerdict: "From the blurb."})

	r := New(st, calibreDir)
	if n, e := r.Queue([]int64{id}, "admin", "admin"); n != 1 || e.Words != 6004 || e.Parts != 4 { // each 3,002-word chapter is two local-size parts
		t.Fatalf("queue: %d %+v", n, e) // ai.URL is on 127.0.0.1, so parts are the local size
	}
	next, ok := st.NextDeepRead()
	if !ok {
		t.Fatal("nothing queued")
	}
	if err := r.scan(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	b, _ := st.BookByID(id, nil)
	if *b.SpiceLevel != 4 || !b.Nudity || b.AnalysisModel != "deep: big-model" || b.SpiceReason != "Explicit scene in Chapter 2" {
		t.Fatalf("book after scan: level %d nudity %v model %q reason %q", *b.SpiceLevel, b.Nudity, b.AnalysisModel, b.SpiceReason)
	}
	dr := st.LatestDeepRead(id)
	if dr.Status != "done" || *dr.PrevLevel != 2 || *dr.NewLevel != 4 || !strings.Contains(dr.Notes, `"label":"Chapter 2 (1/2)"`) {
		t.Fatalf("deep read: %+v", dr)
	}
	items, _, _ := st.Notifications(5)
	if len(items) == 0 || items[0].Source != "deep-scan" || items[0].Level != "warning" || !strings.Contains(items[0].Message, "Level 2 to Level 4") {
		t.Fatalf("escalation notice: %+v", items)
	}
	// Re-rates leave a Deep Scan alone, and it isn't picked again.
	if ids, _ := st.AIRatedIDs(); len(ids) != 0 {
		t.Fatalf("deep ratings must not be re-rated: %v", ids)
	}
	if _, err := st.RequestDeepRead(id, "admin", "admin", "", true, 1, 1, 1); err != nil {
		t.Fatalf("a finished scan doesn't block a new one: %v", err)
	}
}

func TestDeepUsersSetting(t *testing.T) {
	if got := DeepUsers(" 3, x, 7,0, 9, 11"); len(got) != 3 || got[0] != 3 || got[2] != 9 {
		t.Fatalf("up to 3 valid ids: %v", got)
	}
}
