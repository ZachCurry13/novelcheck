package content_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/content"
)

func TestCatalog(t *testing.T) {
	n := 0
	for _, g := range content.Groups {
		n += len(g.Items)
		for _, it := range g.Items {
			if content.GroupOf(it.Key) != g.Key || content.Normalize(it.Key) != it.Key || content.Normalize(it.Label) != it.Key {
				t.Errorf("%s doesn't round-trip", it.Key)
			}
		}
	}
	if n != 45 {
		t.Fatalf("%d items, want 45", n)
	}
	for _, k := range append(append([]string{}, content.YoungPreset...), content.StrictPreset...) {
		if content.GroupOf(k) == "" {
			t.Errorf("preset has unknown item %q", k)
		}
	}
}

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"War / battles": "war", "gun-violence": "gun_violence", " Sexual Assault ": "sexual_violence",
		"F-word": "strong_profanity", "LGBTQ+ themes": "lgbtq", "suicide": "self_harm", "romance": "", "": "",
	} {
		if got := content.Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRules(t *testing.T) {
	for k, ok := range map[string]bool{"murder": true, "g:gore": true, "flag:heavy_swearing": true,
		"g:romance": false, "nudity": false, "flag:": false, "flag:DROP TABLE": false} {
		if content.ValidRule(k) != ok {
			t.Errorf("ValidRule(%q) = %v", k, !ok)
		}
	}
	items, custom := content.Expand([]string{"g:gore", "blood", "murder", "flag:swearing", "bogus"})
	if len(items) != 9 || !slices.Contains(items, "murder") || strings.Join(custom, ",") != "swearing" {
		t.Fatalf("%v %v", items, custom)
	}
}

func TestAmount(t *testing.T) {
	for _, c := range []struct{ parts, total, want int }{
		{0, 30, content.None}, {1, 30, content.ALittle}, {2, 30, content.ALittle}, {3, 30, content.Some},
		{10, 30, content.Some}, {11, 30, content.ALot}, {2, 4, content.ALot}, {1, 3, content.ALittle},
	} {
		if got := content.Amount(c.parts, c.total); got != c.want {
			t.Errorf("Amount(%d, %d) = %d, want %d", c.parts, c.total, got, c.want)
		}
	}
}

func TestPromptList(t *testing.T) {
	list := content.PromptList()
	if strings.Count(list, "\n") != 4 || !strings.Contains(list, "gun_violence (people shot or shot at)") {
		t.Fatal(list)
	}
}
