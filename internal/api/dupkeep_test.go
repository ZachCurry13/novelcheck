package api

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestKeepReason(t *testing.T) {
	f := func(format string, size int64) store.DupFile { return store.DupFile{Format: format, Size: size} }
	epub := store.DupEntry{CalibreID: "1", Files: []store.DupFile{f("epub", 100)}}
	mobi := store.DupEntry{CalibreID: "2", Files: []store.DupFile{f("mobi", 100)}}
	if got := keepReason([]store.DupEntry{epub, mobi}, "1"); got != "it is the one with an EPUB" {
		t.Errorf("the EPUB copy: %q", got)
	}
	big := store.DupEntry{CalibreID: "3", Files: []store.DupFile{f("epub", 900)}}
	if got := keepReason([]store.DupEntry{big, epub}, "3"); got != "its file is the larger one" {
		t.Errorf("the larger copy: %q", got)
	}
	twin := store.DupEntry{CalibreID: "4", Files: []store.DupFile{f("epub", 100)}}
	if got := keepReason([]store.DupEntry{epub, twin}, "1"); got == "" || got == "it is the one with an EPUB" {
		t.Errorf("alike copies: %q", got)
	}
}
