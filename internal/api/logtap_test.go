package api_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
)

// A background failure in the server's log lands in the bug report's error list, once per ten minutes for the
// same failure; ordinary lines don't.
func TestBackgroundErrorsAreKept(t *testing.T) {
	var server *api.Server
	_, st := setupWith(t, func(s *api.Server) { server = s })
	var console bytes.Buffer
	w := server.LogWriter(&console)
	_, _ = w.Write([]byte("2026/11/17 10:01:02 hardcover ratings: 429 Too Many Requests: failed\n"))
	_, _ = w.Write([]byte("2026/11/17 10:01:03 hardcover ratings: 431 Too Many Requests: failed\n")) // the same failure again
	_, _ = w.Write([]byte("2026/11/17 10:01:04 calibre sync: read 3000 books, errors: 0\n"))
	_, _ = w.Write([]byte("2026/11/17 10:01:05 name finder: \"Dune\" couldn't be saved in Calib"))
	_, _ = w.Write([]byte("re\n")) // a line written in two pieces
	if !strings.Contains(console.String(), "errors: 0") {
		t.Fatal("every line still reaches the console")
	}
	var got []string
	for _, e := range st.RecentErrors(20) {
		if e.Method == "background" {
			got = append(got, e.Path+": "+e.Detail)
		}
	}
	if len(got) != 2 || !strings.Contains(strings.Join(got, "|"), "hardcover ratings") || !strings.Contains(strings.Join(got, "|"), "Calibre") {
		t.Fatalf("background errors kept: %q", got)
	}
}
