package store

import (
	"regexp"
	"testing"
)

func TestSyncWords(t *testing.T) {
	seen := map[string]bool{}
	plain := regexp.MustCompile(`^[a-z]{3,8}$`)
	for _, w := range syncWords {
		if !plain.MatchString(w) || seen[w] {
			t.Fatalf("bad or repeated word %q", w)
		}
		seen[w] = true
	}
	if len(syncWords) < 250 {
		t.Fatalf("only %d words", len(syncWords))
	}
	if c := newSyncCode(); !regexp.MustCompile(`^[a-z]{9,24}$`).MatchString(c) {
		t.Fatalf("code %q", c)
	}
}
