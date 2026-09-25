package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/llm"
)

const sample = `{"classification":"Closed Door","content_elements":{"nudity":false,"solo_acts":false,"heavy_innuendo":true},
"spiritual_elements":{"playful_fantasy":true,"dark_occult":false,"demonic_presence":true},
"lgbtq_content":false,"summary_verdict":"Tension builds but intimacy happens off-page."}`

func TestParseVerdictToleratesFences(t *testing.T) {
	v, err := llm.ParseVerdict("Here you go:\n```json\n" + sample + "\n```")
	if err != nil {
		t.Fatal(err)
	}
	if v.Classification != "Closed Door" || !v.ContentElements.HeavyInnuendo {
		t.Fatalf("bad parse %+v", v)
	}
	if !v.SpiritualElements.DarkOccult {
		t.Fatal("demonic presence should imply dark_occult")
	}
}

func TestParseVerdictNormalizesAndRejects(t *testing.T) {
	v, err := llm.ParseVerdict(`{"classification":"open-door"}`)
	if err != nil || v.Classification != "Open Door" {
		t.Fatalf("normalize failed: %v %+v", err, v)
	}
	if _, err := llm.ParseVerdict(`{"classification":"Spicy"}`); err == nil {
		t.Fatal("expected invalid classification error")
	}
	if _, err := llm.ParseVerdict(`no json here`); err == nil {
		t.Fatal("expected missing JSON error")
	}
}

func TestClientComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			http.Error(w, "bad request", 400)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "gpt-4o-mini" || body["response_format"] == nil {
			http.Error(w, "bad body", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": sample}}},
			"usage":   map[string]int{"prompt_tokens": 700, "completion_tokens": 90},
		})
	}))
	defer srv.Close()
	c := &llm.Client{BaseURL: srv.URL + "/v1/", APIKey: "k", JSONMode: true}
	out, usage, err := c.Complete(context.Background(), "gpt-4o-mini", llm.SystemPrompt, llm.UserPrompt("T", "A", "B"))
	if err != nil {
		t.Fatal(err)
	}
	if usage.Total() != 790 || !strings.Contains(out, "Closed Door") {
		t.Fatalf("unexpected %d %q", usage.Total(), out)
	}
}

func TestUserPromptTruncatesRuneSafe(t *testing.T) {
	p := llm.UserPrompt("T", "", strings.Repeat("é", 5000))
	if !strings.Contains(p, "Author: Unknown") || !strings.HasSuffix(p, "…") {
		t.Fatalf("unexpected prompt tail: %q", p[len(p)-20:])
	}
}

func TestParseSpiceLevel(t *testing.T) {
	cases := map[string]struct {
		level int
		class string
	}{
		`{"spice_level": 0, "summary_verdict": "x"}`:       {0, "No Spice"},
		`{"spice_level": "1", "summary_verdict": "x"}`:     {1, "No Spice"},
		`{"spice_level": "2", "summary_verdict": "x"}`:     {2, "Closed Door"},
		`{"spice_level": 3.0}`:                             {3, "Closed Door"},
		`{"spice_level": 5, "classification": "No Spice"}`: {5, "Open Door"},
		"```json\n{\"spice_level\": 4}\n```":               {4, "Open Door"},
	}
	for in, want := range cases {
		v, err := llm.ParseVerdict(in)
		if err != nil || v.SpiceLevel == nil || *v.SpiceLevel != want.level || v.Classification != want.class {
			t.Errorf("%s: got %+v err %v", in, v, err)
		}
	}
	for _, bad := range []string{`{"spice_level": 7}`, `{"spice_level": "hot"}`, `{"summary_verdict": "no level"}`} {
		if _, err := llm.ParseVerdict(bad); err == nil {
			t.Errorf("%s should be rejected", bad)
		}
	}
	// Older-format answers still work, without a pepper level.
	if v, err := llm.ParseVerdict(`{"classification": "Closed Door"}`); err != nil || v.SpiceLevel != nil || v.Classification != "Closed Door" {
		t.Errorf("legacy: %+v %v", v, err)
	}
}

func TestSpiceReason(t *testing.T) {
	v, err := llm.ParseVerdict(`{"spice_level": 3, "spice_reason": "  \"Heavy innuendo,\n on-page foreplay.\" "}`)
	if err != nil || v.ToAnalysis("m").SpiceReason != "Heavy innuendo, on-page foreplay" {
		t.Fatalf("reason: %+v %v", v, err)
	}
	if v, _ := llm.ParseVerdict(`{"spice_level": 1}`); v.SpiceReason != "" {
		t.Fatalf("missing reason should stay empty: %q", v.SpiceReason)
	}
	long := llm.ShortReason(strings.Repeat("très ", 40))
	if r := []rune(long); len(r) != 80 || !strings.HasSuffix(long, "…") {
		t.Fatalf("long reason not capped rune-safe: %d %q", len(r), long)
	}
	for _, want := range []string{"spice_reason", "Mild / Closed Door", "Steamy Closed Door / Heavy Tension"} {
		if !strings.Contains(llm.SystemPrompt, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}
}

func TestParseGenres(t *testing.T) {
	out := "Sure!\n```json\n{\"books\": [{\"id\": 1, \"genres\": [\"Fantasy\", \"made-up\", \"ya\", \"romance\", \"horror\"], \"fiction\": true},\n" +
		"{\"id\": 2, \"genres\": [\"history\"], \"fiction\": false}, {\"id\": 99, \"genres\": [\"fantasy\"]}]}\n```"
	res, err := llm.ParseGenres(out, map[int64]bool{1: true, 2: true})
	if err != nil || len(res) != 2 || strings.Join(res[1].Keys, ",") != "fantasy,ya,romance" || res[1].Kind != "fiction" || res[2].Kind != "nonfiction" {
		t.Fatalf("%+v %v", res, err)
	}
	if !strings.Contains(llm.GenreSystem(), "truecrime: True Crime") {
		t.Fatal("the list is in the prompt")
	}
}
