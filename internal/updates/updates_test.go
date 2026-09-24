package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/version"
)

func TestStatus(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write([]byte(`[
		 {"tag_name":"v1.2.0-beta","prerelease":true},
		 {"tag_name":"v1.1.0","name":"NovelCheck v1.1.0","body":"## New\n- Editor role","html_url":"https://x/1.1.0"},
		 {"tag_name":"v1.0.0","body":"First"}]`))
	}))
	defer srv.Close()
	old := version.Version
	version.Version = "1.0.0"
	defer func() { version.Version = old }()

	c := &Checker{URL: srv.URL, HTTP: srv.Client()}
	st := c.Status(context.Background(), true)
	if !st.UpdateAvailable || st.Latest != "v1.1.0" || len(st.Releases) != 2 {
		t.Fatalf("unexpected status %+v", st)
	}
	if !st.Releases[0].Newer || !st.Releases[1].Installed {
		t.Fatalf("release flags wrong: %+v", st.Releases)
	}
	c.Status(context.Background(), true) // cached
	if calls.Load() != 1 {
		t.Fatalf("expected 1 GitHub call, got %d", calls.Load())
	}
	if st := c.Status(context.Background(), false); st.UpdateAvailable || len(st.Releases) != 0 {
		t.Fatal("disabled checks must not report updates")
	}
}
