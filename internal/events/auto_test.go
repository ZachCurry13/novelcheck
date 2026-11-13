package events

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func testStore(t *testing.T) *store.Store {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return store.New(d)
}

// fakeSites serves a Stuff Your Kindle home with today's day and a coming
// one, and a free-books feed whose posts can be changed.
func fakeSites(t *testing.T, feed *string) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<a href="/days/mystery-november-2026">Mystery</a><a href="https://elsewhere.example/days/x">x</a>
				<a href="/days/romance-december-2026">Romance</a><a href="/days/archive">Past days</a>`)
		case "/days/mystery-november-2026":
			var b strings.Builder
			b.WriteString(`<title>Mystery Stuff Your Kindle Day — November 13, 2026</title>`)
			for i := 1; i <= 5; i++ {
				b.WriteString(row(i, fmt.Sprintf("https://www.amazon.com/dp/B00000%04d", i)))
			}
			fmt.Fprint(w, b.String())
		case "/days/romance-december-2026":
			fmt.Fprint(w, `<title>Romance Stuff Your Kindle Day — December 4, 2026</title>`+row(1, "https://www.amazon.com/dp/B000009999"))
		case "/feed":
			fmt.Fprint(w, *feed)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	old, home, rf := client, sykHome, redditFeed
	client, sykHome, redditFeed = srv.Client(), srv.URL+"/", srv.URL+"/feed"
	t.Cleanup(func() { client, sykHome, redditFeed = old, home, rf })
}

func TestStuffYourKindleDayBecomesAnEvent(t *testing.T) {
	st := testStore(t)
	fakeSites(t, new(string))
	now, _ := time.Parse(time.RFC3339, "2026-11-13T15:00:00Z")
	var queued []int64
	enqueue := func(_ bool, ids ...int64) { queued = append(queued, ids...) }
	for range 2 { // the second look makes nothing new
		if err := stuffYourKindle(context.Background(), st, enqueue, now); err != nil {
			t.Fatal(err)
		}
	}
	evs, err := st.ListEvents(&store.User{Role: "admin"})
	if err != nil || len(evs) != 1 {
		t.Fatalf("events: %+v %v", evs, err)
	}
	if evs[0].Name != "Mystery Stuff Your Kindle Day" || evs[0].Books != 5 || len(queued) != 5 || !strings.HasPrefix(evs[0].EndsAt, "2026-11-14T08:00") {
		t.Fatalf("event: %+v, queued %d", evs[0], len(queued))
	}
}

func TestFreeRightNowFollowsTheFeed(t *testing.T) {
	st := testStore(t)
	feed := `<feed>` + post("2026-11-13T14:00:00+00:00", "First Book — Ann Writer", `<a href="https://www.amazon.com/dp/B0AAAAAAA1">[link]</a>`) +
		post("2026-11-13T13:00:00+00:00", "Second Book", `<a href="https://www.amazon.com/dp/B0AAAAAAA2">[link]</a>`) + `</feed>`
	fakeSites(t, &feed)
	now, _ := time.Parse(time.RFC3339, "2026-11-13T15:00:00Z")
	var queued []int64
	enqueue := func(_ bool, ids ...int64) { queued = append(queued, ids...) }
	if err := freeRightNow(context.Background(), st, enqueue, now); err != nil {
		t.Fatal(err)
	}
	feed = `<feed>` + post("2026-11-14T09:00:00+00:00", "Third Book", `<a href="https://www.amazon.com/dp/B0AAAAAAA3">[link]</a>`) +
		post("2026-11-13T14:00:00+00:00", "First Book — Ann Writer", `<a href="https://www.amazon.com/dp/B0AAAAAAA1">[link]</a>`) + `</feed>`
	if err := freeRightNow(context.Background(), st, enqueue, now.Add(20*time.Hour)); err != nil {
		t.Fatal(err)
	}
	evs, _ := st.ListEvents(&store.User{Role: "admin"})
	if len(evs) != 1 || evs[0].Name != "Free right now" || !evs[0].Pinned || evs[0].Books != 2 {
		t.Fatalf("event: %+v", evs)
	}
	if len(queued) != 3 { // each book once, though the first stayed on the list
		t.Fatalf("queued %v", queued)
	}
}
