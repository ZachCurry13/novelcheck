package discover

import (
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The lists NovelCheck keeps. NYT lists need a key; the Open Library
// subjects fill in without one ("ol:classics" is always fetched).
const (
	listFiction    = "nyt:combined-print-and-e-book-fiction"
	listNonfiction = "nyt:combined-print-and-e-book-nonfiction"
	listTeen       = "nyt:young-adult-hardcover"
	listMiddle     = "nyt:childrens-middle-grade-hardcover"
	listPicture    = "nyt:picture-books"
	listClassics   = "ol:classics"
	listTrending   = "ol:trending"
	listTeenOL     = "ol:young_adult_fiction"
	listKidsOL     = "ol:juvenile_fiction"
)

// nytLists are fetched, in this order, when there's a key.
var nytLists = []string{listFiction, listNonfiction, listTeen, listMiddle, listPicture}

// fallbackLists stand in for the NYT rows without a key.
var fallbackLists = []string{listTrending, listTeenOL, listKidsOL}

// Row is one strip on the Discover tab.
type Row struct {
	Key   string               `json:"key"`
	Icon  string               `json:"icon"`
	Title string               `json:"title"`
	Books []store.DiscoverBook `json:"books"`
}

type rowDef struct {
	key, icon, title string
	lists            []string // shown together, in this order
	fallback         string   // used when the lists are empty
}

var rowDefs = []rowDef{
	{"popular", "🔥", "Popular now", []string{listFiction}, listTrending},
	{"new", "🆕", "New on the best-seller lists", nil, ""},
	{"nonfiction", "📰", "Popular nonfiction", []string{listNonfiction}, ""},
	{"new_in_library", "📚", "New in your libraries", nil, ""},
	{"family", "❤️", "Popular in the family", nil, ""},
	{"teen", "🧑‍🎓", "Top teen books", []string{listTeen}, listTeenOL},
	{"kids", "🧒", "Top kids' books", []string{listMiddle, listPicture}, listKidsOL},
	{"classics", "🏛️", "All-time classics", []string{listClassics}, ""},
}

// kidOrder puts the rows for a kid's age first: young kids and middle grade
// see kids' books first, teens see teen books first.
var kidOrder = map[int][]string{
	1: {"kids", "classics", "family", "new_in_library", "teen"},
	2: {"kids", "teen", "classics", "family", "new_in_library"},
	3: {"teen", "kids", "classics", "family", "new_in_library"},
	4: {"teen", "classics", "family", "new_in_library"},
}

const perRow = 20

// olFetch is how many books an Open Library list asks for: more than a row
// shows, because books the family already has are left out of the rows.
const olFetch = 60

// Compose builds the rows the viewer sees from the listed books and the
// family's own new and favorite books. Empty rows are left out. Discover is
// for finding books, so the outside lists leave out books the family already
// has unless withOwned; owned says how many listed books were left out.
func Compose(items, newInLibrary, family []store.DiscoverBook, u *store.User, withOwned bool) (rows []Row, owned int) {
	byList := map[string][]store.DiscoverBook{}
	var fresh []store.DiscoverBook
	ownedIDs := map[int64]bool{}
	for _, b := range items {
		if b.Owned && !withOwned {
			ownedIDs[b.ID] = true
			continue
		}
		byList[b.List] = append(byList[b.List], b)
		if strings.HasPrefix(b.List, "nyt:") && b.Weeks == 1 {
			fresh = append(fresh, b)
		}
	}
	books := map[string][]store.DiscoverBook{"new": fresh, "new_in_library": newInLibrary, "family": family}
	for _, d := range rowDefs {
		if d.lists == nil {
			continue
		}
		var bs []store.DiscoverBook
		for _, l := range d.lists {
			bs = append(bs, byList[l]...)
		}
		if len(bs) == 0 && d.fallback != "" {
			bs = byList[d.fallback]
		}
		books[d.key] = bs
	}
	for _, d := range ordered(u) {
		if bs := unique(books[d.key]); len(bs) > 0 {
			rows = append(rows, Row{Key: d.key, Icon: d.icon, Title: d.title, Books: bs})
		}
	}
	return rows, len(ownedIDs)
}

// ordered returns the row definitions in the order the viewer sees them.
func ordered(u *store.User) []rowDef {
	first := []string(nil)
	if u != nil && u.Role == store.RoleRestricted {
		first = kidOrder[u.AgeLevel]
	}
	var out []rowDef
	for _, k := range first {
		for _, d := range rowDefs {
			if d.key == k {
				out = append(out, d)
			}
		}
	}
	for _, d := range rowDefs {
		if !contains(first, d.key) {
			out = append(out, d)
		}
	}
	return out
}

// unique drops repeats (a book on two lists) and caps the row.
func unique(bs []store.DiscoverBook) []store.DiscoverBook {
	seen := map[int64]bool{}
	var out []store.DiscoverBook
	for _, b := range bs {
		if !seen[b.ID] && len(out) < perRow {
			seen[b.ID] = true
			out = append(out, b)
		}
	}
	return out
}

// FromNYT reports whether any row shows New York Times data (their terms ask
// for a credit line).
func FromNYT(rows []Row) bool {
	for _, r := range rows {
		for _, b := range r.Books {
			if strings.HasPrefix(b.List, "nyt:") {
				return true
			}
		}
	}
	return false
}
