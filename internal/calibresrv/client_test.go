package calibresrv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestParseParamsAndNormalize(t *testing.T) {
	p := parseParams(`realm="calibre", nonce="ab:cd", algorithm="MD5", qop="auth"`)
	if p["realm"] != "calibre" || p["nonce"] != "ab:cd" || p["qop"] != "auth" {
		t.Fatalf("parse: %v", p)
	}
	if u, err := Normalize("192.168.1.5:8081/"); err != nil || u != "http://192.168.1.5:8081" {
		t.Fatalf("normalize: %q %v", u, err)
	}
	if _, err := Normalize("ftp://x"); err == nil {
		t.Fatal("ftp accepted")
	}
}

// TestDigestAgainstFake checks the Digest response matches RFC 2617 as the
// calibre server computes it.
func TestDigestAgainstFake(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if h == "" {
			w.Header().Set("WWW-Authenticate", `Digest realm="calibre", nonce="n1", algorithm="MD5", qop="auth"`)
			w.WriteHeader(401)
			return
		}
		p := parseParams(strings.TrimPrefix(h, "Digest "))
		ha1 := md5hex("nc:calibre:secret123")
		ha2 := md5hex(r.Method + ":" + r.URL.RequestURI())
		want := md5hex(strings.Join([]string{ha1, "n1", p["nc"], p["cnonce"], "auth", ha2}, ":"))
		if p["response"] != want || p["uri"] != r.URL.RequestURI() {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`{"library_map":{"lib":"lib"},"default_library":"lib"}`))
	}))
	defer srv.Close()
	c := &Client{URL: srv.URL, Username: "nc", Password: "secret123"}
	libs, def, err := c.Libraries(context.Background())
	if err != nil || def != "lib" || libs["lib"] != "lib" {
		t.Fatalf("libraries: %v %q %v", libs, def, err)
	}
	c.Password = "wrong"
	if _, _, err := c.Libraries(context.Background()); err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("expected login error, got %v", err)
	}
}

// TestRealCalibreServer runs against a live calibre-server when
// NOVELCHECK_TEST_CALIBRE_URL is set (see the README's Development section).
func TestRealCalibreServer(t *testing.T) {
	base := os.Getenv("NOVELCHECK_TEST_CALIBRE_URL")
	if base == "" {
		t.Skip("set NOVELCHECK_TEST_CALIBRE_URL to test against a real calibre-server")
	}
	ctx := context.Background()
	c := &Client{URL: base, Username: os.Getenv("NOVELCHECK_TEST_CALIBRE_USER"), Password: os.Getenv("NOVELCHECK_TEST_CALIBRE_PASS")}
	_, def, err := c.Libraries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c.Library = def
	titles, err := c.Titles(ctx, []int{1, 3, 999})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("titles: %v", titles)
	if titles[1] == "" || titles[3] == "" || titles[999] != "" {
		t.Fatalf("unexpected titles %v", titles)
	}
	if err := c.Remove(ctx, []int{3}); err != nil {
		t.Fatal(err)
	}
	if after, _ := c.Titles(ctx, []int{3}); after[3] != "" {
		t.Fatal("book 3 still present after remove")
	}
	ro := &Client{URL: base, Username: "reader", Password: "readpass1", Library: def}
	if err := ro.Remove(ctx, []int{1}); err == nil || !strings.Contains(err.Error(), "permission") {
		t.Fatalf("read-only user should be refused, got %v", err)
	}
}
