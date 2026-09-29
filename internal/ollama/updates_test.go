package ollama

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOutdated(t *testing.T) {
	newManifest := []byte(`{"schemaVersion":2,"layers":["new"]}`)
	oldManifest := []byte(`{"schemaVersion":2,"layers":["old"]}`)
	digest := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
	reg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/library/qwen2.5/manifests/7b":
			_, _ = w.Write(newManifest)
		case "/v2/library/llama3.2/manifests/latest":
			_, _ = w.Write(oldManifest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer reg.Close()
	ol := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{
			{"name": "qwen2.5:7b", "digest": digest(oldManifest)},
			{"name": "llama3.2:latest", "digest": digest(oldManifest)},
			{"name": "my-own:1", "digest": "abc"},
		}})
	}))
	defer ol.Close()
	Registry = reg.URL
	got, err := Outdated(context.Background(), ol.URL, []string{"qwen2.5:7b", "llama3.2", "my-own:1", "not-installed:3b"})
	if err != nil || len(got) != 1 || got[0].Model != "qwen2.5:7b" || got[0].Digest != digest(newManifest) {
		t.Fatalf("%+v %v", got, err)
	}
}
