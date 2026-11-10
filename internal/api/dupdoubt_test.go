package api

import (
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestDuplicateDoubts(t *testing.T) {
	f := func(format string, size int64) store.DupFile { return store.DupFile{Format: format, Size: size} }
	kept := store.DupEntry{CalibreID: "1", Files: []store.DupFile{f("EPUB", 1_000_000), f("MOBI", 1_200_000)}}
	for name, c := range map[string]struct {
		extra store.DupEntry
		want  string
	}{
		"the same book again":   {store.DupEntry{CalibreID: "2", Files: []store.DupFile{f("EPUB", 1_100_000)}}, ""},
		"another e-book format": {store.DupEntry{CalibreID: "2", Files: []store.DupFile{f("AZW3", 1_500_000)}}, ""},
		"an omnibus":            {store.DupEntry{CalibreID: "2", Files: []store.DupFile{f("EPUB", 6_000_000)}}, "differ a lot"},
		"a PDF only there":      {store.DupEntry{CalibreID: "2", Files: []store.DupFile{f("EPUB", 1_000_000), f("PDF", 9_000_000)}}, "has a PDF"},
		"sizes unknown":         {store.DupEntry{CalibreID: "2", Files: []store.DupFile{f("EPUB", 0)}}, "couldn't be compared"},
	} {
		entries := []store.DupEntry{kept, c.extra}
		markDoubts(entries, "1")
		got := entries[1].Doubt
		if (c.want == "") != (got == "") || !strings.Contains(got, c.want) {
			t.Errorf("%s: doubt %q, want %q", name, got, c.want)
		}
		if entries[0].Doubt != "" {
			t.Errorf("%s: the copy that stays has no doubt", name)
		}
	}
}
