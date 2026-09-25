package calibre

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestChangeTimes(t *testing.T) {
	for in, want := range map[string]string{
		"2024-03-01 18:23:45.123456+00:00": "2024-03-01 18:23:45",
		"2024-03-01T18:23:45+00:00":        "2024-03-01 18:23:45",
		"":                                 "",
		"junk":                             "",
	} {
		if got := changeTime(in); got != want {
			t.Errorf("changeTime(%q) = %q, want %q", in, got, want)
		}
	}
	p := filepath.Join(t.TempDir(), "b.epub")
	_ = os.WriteFile(p, []byte("x"), 0o644)
	when := time.Date(2025, 5, 6, 7, 8, 9, 0, time.UTC)
	_ = os.Chtimes(p, when, when)
	if got := fileTime(p); got != "2025-05-06 07:08:09" {
		t.Fatalf("fileTime: %q", got)
	}
	if fileTime(p+"-missing") != "" || latest("2024-01-01 00:00:00", "2025-05-06 07:08:09") != "2025-05-06 07:08:09" {
		t.Fatal("missing file / latest")
	}
}
