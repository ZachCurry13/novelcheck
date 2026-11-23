package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// The browser refuses the whole service worker (no offline app, no offline books) when a precached file is
// listed twice or is missing.
func TestShellFilesOnceAndPresent(t *testing.T) {
	src, err := fs.ReadFile(FS(), "sw.js")
	if err != nil {
		t.Fatal(err)
	}
	shell := string(src)[strings.Index(string(src), "const SHELL = ["):]
	shell = shell[:strings.Index(shell, "\n];")]
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s+"(/[^"]*)",$`).FindAllStringSubmatch(shell, -1) {
		p := m[1]
		if seen[p] {
			t.Errorf("%s is listed twice", p)
		}
		seen[p] = true
		if p == "/" {
			continue
		}
		if _, err := fs.Stat(FS(), strings.TrimPrefix(p, "/")); err != nil {
			t.Errorf("%s is listed but missing", p)
		}
	}
	if len(seen) < 100 {
		t.Fatalf("only %d files read from SHELL", len(seen))
	}
	// A page script imported at load (not import() later) must be precached, or its page can't open offline.
	imp := regexp.MustCompile(`(?m)^import [^;]*? from "\./([a-z0-9]+\.js)";`)
	scripts, _ := fs.Glob(FS(), "js/*.js")
	for _, f := range scripts {
		if !seen["/"+f] {
			continue // only what the offline app loads
		}
		code, _ := fs.ReadFile(FS(), f)
		for _, m := range imp.FindAllStringSubmatch(string(code), -1) {
			if !seen["/js/"+m[1]] {
				t.Errorf("%s imports %s, which isn't in SHELL", f, m[1])
			}
		}
	}
}
