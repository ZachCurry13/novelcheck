package events

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// wishItem is one book the way an Amazon wish list (grid view) shows it: the
// cover link's text is "Quick View", the name is on the link and the picture.
func wishItem(asin, title string) string {
	return fmt.Sprintf(`<li data-itemid="I%[1]s"><a aria-label="%[2]s" class="a-link-normal wl-image-overlay" title="%[2]s" href="/dp/%[1]s/?coliid=I1&amp;colid=L1&amp;ref_=lig_pi_dp">
		<span><button aria-label="Quick View for %[2]s" type="button"> Quick View </button></span></a>
		<img alt="%[2]s" src="https://m.media-amazon.com/x.jpg"/><span class="a-offscreen">$0.00</span>
		<a href="/dp/%[1]s/?colid=L1&amp;coliid=I1&amp;ref_=lig_pab" class="a-button-text"> See all buying options </a></li>`, asin, title)
}

func TestFetchReadsAnAmazonWishList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hz/wishlist/ls/L1":
			fmt.Fprintf(w, `<html><head><title>Amazon.com</title></head><body>
				<span id="profile-list-name" role="heading">Free Book Friday - Mystery</span><a href="https://www.amazon.com/ap/signin?return_to=x">Hello, sign in</a>
				<a href="https://www.amazon.com/gp/goldbox">Today's Deals</a><ul>%s%s</ul>
				<input type="hidden" name="showMoreUrl" value="/hz/wishlist/slv/items?paginationToken=AB%%3D&amp;lid=L1" class="showMoreUrl"/>
				<a href="https://www.amazon.jobs" class="nav_a">Careers</a><a href="https://www.amazon.com/pr" class="nav_a">Press Center</a>
				<a href="https://www.amazon.com/b?node=1" class="nav_a">Accessibility</a>
				<a href="/dp/B0DVBL912R?plattr=ACOMFO&amp;ref_=footer_pay" class="nav_a">Amazon Business Card</a></body></html>`,
				wishItem("B07Q2J4M2Y", "Casino Witch Mysteries 1 &amp; 2 (Casino Witch Mysteries Boxset)"), wishItem("B01IMLH42A", "The First Book"))
		case "/hz/wishlist/slv/items":
			if r.URL.Query().Get("paginationToken") != "AB=" {
				http.NotFound(w, r)
				return
			}
			fmt.Fprint(w, wishItem("B0ABCDEF12", "A Later Book")+wishItem("B0ABCDEF13", "The Last Book"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := client
	client = srv.Client()
	defer func() { client = old }()

	page, err := Fetch(context.Background(), srv.URL+"/hz/wishlist/ls/L1?ref_=wl_share")
	if err != nil {
		t.Fatal(err)
	}
	got := Parse(page, "")
	if len(got) != 4 {
		t.Fatalf("found %d books: %+v", len(got), got)
	}
	if got[0].Title != "Casino Witch Mysteries 1 & 2 (Casino Witch Mysteries Boxset)" || got[0].ASIN != "B07Q2J4M2Y" ||
		!strings.HasPrefix(got[0].Link, "https://www.amazon.com/dp/B07Q2J4M2Y/") {
		t.Fatalf("first: %+v", got[0])
	}
	if got[3].Title != "The Last Book" || got[3].ASIN != "B0ABCDEF13" {
		t.Fatalf("from the next page: %+v", got[3])
	}
	if name, _ := Info(page); name != "Free Book Friday - Mystery" {
		t.Fatalf("name: %q", name)
	}
	for _, b := range got {
		if strings.Contains(b.Title, "Quick View") || strings.Contains(b.Title, "buying") {
			t.Fatalf("a button was taken for a book: %+v", b)
		}
	}
}

func TestStoreLinksAreNotBooks(t *testing.T) {
	site := "https://www.amazon.co.uk"
	for link, want := range map[string]bool{
		"/dp/B07Q2J4M2Y/?x=1":                  true,
		"/gp/help/customer/display.html":       false,
		"//www.amazon.com/pr":                  false,
		"https://www.amazon.jobs":              false,
		"https://www.amazon.com/ap/signin?x=1": false,
		"https://www.amazon.com/s?k=dragons":   true,
		"https://amzn.to/3abcDEF":              true,
		"https://example.com/dp/B07Q2J4M2Y":    false,
	} {
		full, ok := bookLink(link, site)
		if ok != want {
			t.Errorf("%s: book %v, want %v", link, ok, want)
		}
		if ok && strings.HasPrefix(link, "/") && full != site+link {
			t.Errorf("%s: full address %s", link, full)
		}
	}
}
