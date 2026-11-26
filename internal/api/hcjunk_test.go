package api

import (
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/calibresrv"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
)

// Hardcover's promotional tags are not suggested as genres; real ones are.
func TestJunkTagsAreNotSuggested(t *testing.T) {
	for tag, want := range map[string]bool{"Hottest Books of the Season": true, "Best Books of the Year": true, "Top 10": true, "New Releases": true,
		"Fantasy": false, "Romantasy": false, "Non-Fiction, History": false} {
		if junkTag(tag) != want {
			t.Errorf("%q: junk=%v", tag, !want)
		}
	}
	ch := hcChanges(calibresrv.Details{Title: "1929"}, hardcover.BookInfo{Title: "1929", Tags: []string{"History", "Hottest Books of the Season"}})
	for _, c := range ch {
		if c.Field == "tags" && (strings.Contains(c.Shown, "Hottest") || !strings.Contains(c.Shown, "History")) {
			t.Fatalf("genres suggested: %q", c.Shown)
		}
	}
}
