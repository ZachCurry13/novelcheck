package events

// Free books found by themselves: each Stuff Your Kindle day becomes an
// event on its day, and "Free right now" follows r/FreeEBOOKS. Both can be
// switched off in Settings → Discover.

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

const byApp = "NovelCheck"

var (
	sykHome  = "https://www.stuffyourkindle.com/" // a test points it at its own site
	sykDayRe = regexp.MustCompile(`href\s*=\s*["']([^"']*/days/([a-z0-9-]+))["']`)
)

// poke wakes Watch early (settings for it changed).
var poke = make(chan struct{}, 1)

// Poke has Watch look again now, for example after its settings changed.
func Poke() {
	select {
	case poke <- struct{}{}:
	default:
	}
}

// Watch looks for free books every three hours (first a few minutes after
// start, and soon after its settings change). enqueue rates new books;
// first says whether ahead of the rest. ask reads BookBub's email.
func Watch(ctx context.Context, st *store.Store, ask Asker, enqueue func(first bool, ids ...int64)) {
	wait := 3 * time.Minute
	told := map[string]string{} // the last problem told per source, so the bell isn't filled
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		case <-poke:
			time.Sleep(5 * time.Second) // let the rest of a settings save land
		}
		wait = 3 * time.Hour
		for _, src := range []struct {
			key, name string
			run       func() error
		}{
			{store.KeyEventsAutoSYK, "Stuff Your Kindle", func() error { return stuffYourKindle(ctx, st, enqueue, time.Now()) }},
			{store.KeyEventsAutoReddit, "Free right now", func() error { return freeRightNow(ctx, st, enqueue, time.Now()) }},
			{store.KeyEventsAutoBookBub, "BookBub email", func() error { return bookBub(ctx, st, ask, enqueue, time.Now()) }},
		} {
			if !st.SettingBool(src.key) {
				continue
			}
			err := src.run()
			if err == nil {
				told[src.name] = ""
				continue
			}
			log.Printf("events: %s: %v", src.name, err)
			if src.name == "BookBub email" && told[src.name] != err.Error() { // the one an admin can fix
				st.Notify("warning", "events", "BookBub email: "+err.Error(), "#/admin?tab=delivery")
			}
			told[src.name] = err.Error()
		}
	}
}

// usDay is the day it is in the US, where these events run (Eastern time,
// without daylight saving: close enough to pick the day).
func usDay(now time.Time) string { return now.In(time.FixedZone("US", -5*3600)).Format("2006-01-02") }

// stuffYourKindle makes today's Stuff Your Kindle days into events, once each.
func stuffYourKindle(ctx context.Context, st *store.Store, enqueue func(bool, ...int64), now time.Time) error {
	home, err := get(ctx, sykHome, "text/html")
	if err != nil {
		return err
	}
	base, _ := url.Parse(sykHome)
	seen := map[string]bool{}
	for _, m := range sykDayRe.FindAllStringSubmatch(home, -1) {
		u := sameSite(base, m[1])
		if u == nil || m[2] == "archive" || seen[u.String()] {
			continue
		}
		link := u.String()
		seen[link] = true
		if _, _, found := st.EventBySource(link); found {
			continue // made already (or made by hand from the same page)
		}
		page, err := Fetch(ctx, link)
		if err != nil {
			continue
		}
		name, day := Info(page)
		if day != usDay(now) {
			continue // a coming day: its event is made on the day
		}
		books := Parse(page, "")
		if len(books) < 3 {
			continue // the list isn't up yet; look again later
		}
		end, _ := time.Parse("2006-01-02", day)
		ends := end.Add(32 * time.Hour).UTC().Format(time.RFC3339) // the end of the day in the US
		id, unrated, err := st.CreateEvent(name, link, byApp, ends, books)
		if err != nil {
			return err
		}
		if len(unrated) > 0 {
			enqueue(true, unrated...)
		}
		st.Notify("info", "events", fmt.Sprintf("Today is %s: %d free books are in Events.", strings.TrimSpace(name), len(books)),
			fmt.Sprintf("#/library?event=%d", id))
	}
	return nil
}
