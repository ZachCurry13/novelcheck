package events

import (
	"context"
	"testing"
)

func TestParseHTMLAndText(t *testing.T) {
	page := `<div><h3>The Midnight Library &amp; Friends</h3><p>by Someone</p>
		<a class="btn" href="https://www.amazon.com/dp/B08XYZ1234?tag=x">Buy on Amazon</a></div>
	<ul><li><a href="https://www.amazon.com/Dragon-Rider-Cornelia-Funke/dp/0439456959/ref=sr_1_1">Dragon Rider by Cornelia Funke</a> FREE</li>
	<li><a href="https://amzn.to/3abcDEF"><img alt="Inkheart" src="x.jpg"></a></li>
	<li><a href="https://example.com/not-amazon">Not a book</a></li>
	<li><a href="https://www.amazon.com/dp/0439456959">Dragon Rider again</a></li></ul>`
	got := Parse(page, "")
	if len(got) != 3 {
		t.Fatalf("got %d: %+v", len(got), got)
	}
	if got[0].Title != "The Midnight Library & Friends" || got[0].ASIN != "B08XYZ1234" {
		t.Fatalf("heading title: %+v", got[0])
	}
	if got[1].Title != "Dragon Rider" || got[1].Author != "Cornelia Funke" || got[1].ASIN != "0439456959" {
		t.Fatalf("link text: %+v", got[1])
	}
	if got[2].Title != "Inkheart" || got[2].ASIN != "" || got[2].Link != "https://amzn.to/3abcDEF" {
		t.Fatalf("image alt: %+v", got[2])
	}

	text := "1. The Hobbit by J.R.R. Tolkien FREE\n• Charlotte's Web by E. B. White ($0.00)\n\nBuy now\nThe Hobbit by J.R.R. Tolkien"
	got = Parse("", text)
	if len(got) != 2 || got[0].Title != "The Hobbit" || got[0].Author != "J.R.R. Tolkien" || got[1].Title != "Charlotte's Web" {
		t.Fatalf("text: %+v", got)
	}
}

func TestFetchRefusesTheHomeNetwork(t *testing.T) {
	for _, u := range []string{"http://127.0.0.1:8080/list", "http://192.168.1.5/", "ftp://example.com/x", "not a url"} {
		if _, err := Fetch(context.Background(), u); err == nil {
			t.Fatalf("%s was fetched", u)
		}
	}
}
