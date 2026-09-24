package enrich

import "testing"

func TestCleanISBN(t *testing.T) {
	for in, want := range map[string]string{
		"978-1-64937-404-2": "9781649374042",
		"0 306 40615 2":     "0306406152",
		"080442957X":        "080442957X",
		"97816X9374042":     "",
		"12345":             "",
		"Fourth Wing":       "",
		"":                  "",
	} {
		if got := CleanISBN(in); got != want {
			t.Errorf("CleanISBN(%q) = %q, want %q", in, got, want)
		}
	}
}
