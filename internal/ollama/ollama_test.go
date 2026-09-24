package ollama

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeOllama serves /api/version, /api/tags and a streaming /api/pull.
func fakeOllama(t *testing.T) (*httptest.Server, *[]string) {
	var mu sync.Mutex
	models := []string{"llama3.2:latest"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			fmt.Fprint(w, `{"version":"0.9.0"}`)
		case "/api/tags":
			mu.Lock()
			defer mu.Unlock()
			parts := []string{}
			for _, m := range models {
				parts = append(parts, fmt.Sprintf(`{"name":%q}`, m))
			}
			fmt.Fprintf(w, `{"models":[%s]}`, strings.Join(parts, ","))
		case "/api/pull":
			fl := w.(http.Flusher)
			for _, line := range []string{
				`{"status":"pulling manifest"}`,
				`{"status":"pulling abc","total":1000,"completed":500}`,
				`{"status":"pulling abc","total":1000,"completed":1000}`,
				`{"status":"success"}`,
			} {
				fmt.Fprintln(w, line)
				fl.Flush()
			}
			mu.Lock()
			models = append(models, "qwen2.5:7b")
			mu.Unlock()
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &models
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"192.168.1.50:11434":            "http://192.168.1.50:11434",
		"http://192.168.1.50:11434/v1/": "http://192.168.1.50:11434",
		"https://ollama.local/":         "https://ollama.local",
	}
	for in, want := range cases {
		if got, err := Normalize(in); err != nil || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "ftp://x", "http://"} {
		if _, err := Normalize(bad); err == nil {
			t.Errorf("Normalize(%q) should fail", bad)
		}
	}
}

func TestProbeDiscoverAndPull(t *testing.T) {
	srv, _ := fakeOllama(t)
	s, err := Probe(context.Background(), srv.URL)
	if err != nil || s.Version != "0.9.0" || len(s.Models) != 1 {
		t.Fatalf("probe: %+v %v", s, err)
	}
	found := Discover(context.Background(), srv.URL)
	if len(found) == 0 || found[0].URL != srv.URL {
		t.Fatalf("discover should include the explicit address: %+v", found)
	}

	var p Puller
	if err := p.Start(srv.URL, "bad name; rm -rf"); err == nil {
		t.Fatal("invalid model name accepted")
	}
	if err := p.Start(srv.URL, "qwen2.5:7b"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for p.Status().Active && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	st := p.Status()
	if !st.Done || st.Percent != 100 || st.Error != "" {
		t.Fatalf("pull status %+v", st)
	}
	s, _ = Probe(context.Background(), srv.URL)
	if len(s.Models) != 2 {
		t.Fatalf("model not added: %+v", s.Models)
	}
}
