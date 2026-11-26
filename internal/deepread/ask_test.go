package deepread

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestParsePartKeepsTheDigest(t *testing.T) {
	res, err := llm.ParsePart(`{"romance": "none", "level": 0, "note": "", "content": [], "digest": "  Sam (main) and Alex (minor, mentioned once) appear. "}`)
	if err != nil || res.Digest != "Sam (main) and Alex (minor, mentioned once) appear." {
		t.Fatalf("digest: %q %v", res.Digest, err)
	}
	if res, err = llm.ParsePart(`{"romance": "none", "level": 0}`); err != nil || res.Digest != "" {
		t.Fatalf("no digest is fine: %q %v", res.Digest, err)
	}
}

// A question is answered from the notes a full read kept, and says so; without them it says it is a best guess.
func TestAnswerUsesTheDigest(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	var prompt string
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		prompt = string(raw)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": "A main character."}}},
			"usage":   map[string]int{"prompt_tokens": 10, "completion_tokens": 5},
		})
	}))
	defer ai.Close()
	_ = st.SetSetting(store.KeyLLMBaseURL, ai.URL)
	_ = st.SetSetting(store.KeyLLMModel, "m")
	id, _ := st.UpsertBook("A Book", "An Author", "", "")
	b, _ := st.BookByID(id, nil)
	r := New(st, t.TempDir())

	ans, basis, err := r.Answer(context.Background(), b, "Is the LGBTQ+ character a main character?")
	if err != nil || ans != "A main character." || basis != "ratings and summary" {
		t.Fatalf("no digest: %q %q %v", ans, basis, err)
	}
	if err := st.SaveDigest(id, []store.DigestPart{{Label: "Chapter 1", Text: "Sam (main) and Alex (a main character, LGBTQ+)."}, {Label: "Chapter 2", Text: ""}}, "m"); err != nil {
		t.Fatal(err)
	}
	_, basis, err = r.Answer(context.Background(), b, "Is the LGBTQ+ character a main character?")
	if err != nil || basis != "full read" || !strings.Contains(prompt, "Alex (a main character, LGBTQ+)") || strings.Contains(prompt, "Chapter 2") {
		t.Fatalf("with a digest: %q %v\n%s", basis, err, prompt)
	}
	if _, _, err := r.Answer(context.Background(), b, "  "); err == nil {
		t.Fatal("an empty question is refused")
	}
}
