package events

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// treePage is a Linktree the way it is served: each list a link named by its
// aria-label, inside a box whose id the short link points at.
func treePage() string {
	var b strings.Builder
	b.WriteString(`<html><head><title>@dailydeals | Linktree</title></head><body><h2>FREE BOOK FRIDAY</h2>
		<a href="https://www.instagram.com/x" aria-label="Instagram">Instagram</a>`)
	for i, l := range []struct{ id, name string }{{"9ZMDH4VWWM2", "Kids"}, {"14HPPTSUAF5NE", "Mystery"}, {"3K5J2Y4JGSTFC", "Thrillers "}} {
		fmt.Fprintf(&b, `<div data-testid="NewLinkContainer" id="5382734%d"><div class="invisible" id="5382734%[1]d"></div>
			<a href="https://www.amazon.com/hz/wishlist/ls/%[2]s?ref_=wl_share" aria-label="%[3]s" target="_blank"><div><svg><title>%[3]s</title></svg>%[3]s</div></a></div>`, i, l.id, l.name)
	}
	b.WriteString(`</body></html>`)
	return b.String()
}

func TestAShortLinkLandsOnItsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path { // tr.ee sends the reader to the tree, marked with the link, then on again
		case "/B9XJoAkx4A":
			http.Redirect(w, r, "/p/3f38?lt_utm_source=lt_share_link#53827341", http.StatusFound)
			return
		case "/p/3f38":
			http.Redirect(w, r, "/dailydeals?lt_utm_source=lt_share_link", http.StatusFound)
			return
		}
		fmt.Fprint(w, treePage())
	}))
	defer srv.Close()
	old := client
	client = srv.Client()
	defer func() { client = old }()

	page, focus, err := FetchLanding(context.Background(), srv.URL+"/B9XJoAkx4A")
	if err != nil || focus != "53827341" {
		t.Fatalf("focus %q, %v", focus, err)
	}
	if got := Parse(page, ""); len(got) != 0 {
		t.Fatalf("a list was taken for a book: %+v", got)
	}
	lists := Lists(page, focus)
	if len(lists) != 3 || lists[1].Name != "Mystery" || lists[2].Name != "Thrillers" {
		t.Fatalf("lists: %+v", lists)
	}
	if lists[0].Picked || !lists[1].Picked || lists[2].Picked {
		t.Fatalf("the shared list should be ticked: %+v", lists)
	}
	if l := Lists(page, ""); l[1].Picked {
		t.Fatal("nothing is ticked without a mark")
	}
}

func TestFetchListsOnlyReadsAmazonLists(t *testing.T) {
	for _, links := range [][]string{{"https://example.com/hz/wishlist/ls/ABC"}, {"http://192.168.1.5/x"}, make([]string, MaxLists+1)} {
		if _, err := FetchLists(context.Background(), links); err == nil {
			t.Fatalf("%v was read", links)
		}
	}
}
