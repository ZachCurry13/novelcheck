package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestAndFilesAreWritten(t *testing.T) {
	dir := t.TempDir()
	write(dir, "https://example.test/dl/", "kindle-app", "1.2.3", "1.0.0", []string{"a.js"}, func(n string) ([]byte, bool) { return []byte("x"), n == "a.js" })
	raw, err := os.ReadFile(filepath.Join(dir, "kindle-app.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil || m.Version != "1.2.3" || m.MinServer != "1.0.0" || m.Prefix != "kindle-app-" || len(m.Files) != 1 {
		t.Fatalf("%v %+v", err, m)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "kindle-app-a.js")); err != nil || string(b) != "x" {
		t.Fatalf("file: %v %q", err, b)
	}
}
