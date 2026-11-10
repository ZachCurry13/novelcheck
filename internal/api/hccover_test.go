package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/calibresrv"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
)

// A book with no cover gets one suggested from Hardcover (only a secure address); one with a cover never does.
func TestCoverSuggestionAndSetCover(t *testing.T) {
	hc := hardcover.BookInfo{Title: "Cinder", Image: "https://img.example/c.jpg"}
	has := func(cs []hcChange) bool {
		for _, c := range cs {
			if c.Field == "cover" {
				return true
			}
		}
		return false
	}
	if !has(hcChanges(calibresrv.Details{Title: "Cinder"}, hc)) {
		t.Fatal("no cover in Calibre: suggest Hardcover's")
	}
	if has(hcChanges(calibresrv.Details{Title: "Cinder", Cover: "/c/cover.jpg"}, hc)) {
		t.Fatal("a book that has a cover keeps it")
	}
	hc.Image = "http://img.example/c.jpg"
	if has(hcChanges(calibresrv.Details{Title: "Cinder"}, hc)) {
		t.Fatal("only secure picture addresses")
	}
	if _, err := fetchCover(context.Background(), "http://img.example/c.jpg"); err == nil {
		t.Fatal("fetchCover refuses a plain http address")
	}

	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = r.Method + " " + r.URL.Path + " " + string(b)
		io.WriteString(w, `{}`)
	}))
	defer srv.Close()
	c := &calibresrv.Client{URL: srv.URL, Library: "config"}
	if err := c.SetCover(context.Background(), 12, []byte("JPEG")); err != nil || got != "POST /cdb/set-cover/12/config JPEG" {
		t.Fatalf("set-cover request: %q %v", got, err)
	}
}
