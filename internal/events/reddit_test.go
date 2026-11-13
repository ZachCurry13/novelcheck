package events

import (
	"fmt"
	"html"
	"strings"
	"testing"
	"time"
)

// post is one r/FreeEBOOKS entry the way its feed carries it: the body is
// escaped HTML, and a single book is linked as "[link]".
func post(when, title, body string) string {
	return fmt.Sprintf(`<entry><author><name>/u/someone</name></author><content type="html">%s</content><id>t3_x</id>
		<published>%s</published><updated>%[2]s</updated><title>%s</title></entry>`,
		html.EscapeString(`<table><tr><td><a href="https://www.reddit.com/r/FreeEBOOKS/comments/1/x/"><img src="p.png"/></a></td><td>`+body+
			` submitted by <a href="https://www.reddit.com/user/someone"> /u/someone </a><br/><span><a href="https://www.reddit.com/r/FreeEBOOKS/comments/1/x/">[comments]</a></span></td></tr></table>`),
		when, html.EscapeString(title))
}

func TestRedditBooks(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-11-13T15:00:00Z")
	feed := `<?xml version="1.0"?><feed><title>newest submissions : FreeEBOOKS</title>` +
		post("2026-11-13T14:25:19+00:00", "[Kindle] The Island of Silent Answers — Meikel Morozov [Free until: November 17]",
			`<a href="https://www.amazon.com/dp/B0HQ79PLMP">[link]</a>`) +
		post("2026-11-13T13:58:47+00:00", "Seen by the Mountain Man", `<a href="https://amazon.com/dp/B0FN8DVYBY">[link]</a>`) +
		post("2026-11-13T13:00:00+00:00", "FELLOWS OF THE SNAKE - ebook free on November 12-14",
			`<p>Get it: <a href="https://www.amazon.com/dp/B0HFNTXS4D">US</a> <a href="https://www.amazon.co.uk/dp/B0HFNTXS4D">UK</a> <a href="https://www.amazon.ca/dp/B0HFNTXS4D">CA</a></p>`) +
		post("2026-11-13T12:13:53+00:00", "🎉 FREE Kindle Books for Kids This Weekend!",
			`<a href="https://www.amazon.com/dp/B0HQVWPYS9">Goal of Faith</a>, <a href="https://www.amazon.com/dp/B0HQQ18ZDX">11/15/26</a>`) +
		post("2026-11-13T11:00:00+00:00", "[[Free Kindle eBook] Another Angle to See the Hidden Threads", `<a href="https://www.amazon.com/dp/B0HQYQ924S">[link]</a>`) +
		post("2026-11-13T10:00:00+00:00", "My thoughts on free books", `no links here`) +
		post("2026-11-01T10:00:00+00:00", "An Old Book — Old Author", `<a href="https://www.amazon.com/dp/B000000OLD">[link]</a>`) +
		`</feed>`
	got := redditBooks(feed, now)
	var lines []string
	for _, b := range got {
		lines = append(lines, b.ASIN+" "+b.Title+" / "+b.Author)
	}
	want := []string{
		"B0HQ79PLMP The Island of Silent Answers / Meikel Morozov",
		"B0FN8DVYBY Seen by the Mountain Man / ",
		"B0HFNTXS4D FELLOWS OF THE SNAKE / ",
		"B0HQVWPYS9 Goal of Faith / ",
		"B0HQYQ924S Another Angle to See the Hidden Threads / ",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestUSDay(t *testing.T) {
	late, _ := time.Parse(time.RFC3339, "2026-11-14T03:00:00Z") // Friday evening in the US
	if usDay(late) != "2026-11-13" {
		t.Fatal(usDay(late))
	}
}
