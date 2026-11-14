package api

// Friend accounts (store.RoleFriend) reach only the parts of NovelCheck that
// never show the family's libraries. Everything not listed here is refused,
// so a new feature stays closed to friends until it is added on purpose.

import (
	"net/http"
	"regexp"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

var friendRoutes = []struct {
	methods string // "" = any
	path    *regexp.Regexp
}{
	{"GET", regexp.MustCompile(`^/api/me$`)},
	{"PUT", regexp.MustCompile(`^/api/me/(password|pin|guide-seen|start-page|appearance)$`)},
	{"", regexp.MustCompile(`^/api/me/searches$`)},
	{"POST", regexp.MustCompile(`^/api/me/bug-report$`)},
	{"", regexp.MustCompile(`^/api/activity/(status|events)$`)},
	{"GET", regexp.MustCompile(`^/api/me/reading-stats$`)},
	{"", regexp.MustCompile(`^/api/me/libby(/[^/]+)?$`)},
	{"", regexp.MustCompile(`^/api/libby/(audio|check|search|cover|add)$`)},
	{"GET", regexp.MustCompile(`^/api/books/\d+/libby$`)},
	{"PUT", regexp.MustCompile(`^/api/books/\d+/(rating|finished)$`)},
	{"", regexp.MustCompile(`^/api/me/hardcover(/(sync|push))?$`)},
	{"GET", regexp.MustCompile(`^/api/books(/facets|/states|/\d+|/\d+/cover)?$`)},
	{"GET", regexp.MustCompile(`^/api/events(/\d+)?$`)},
	{"POST", regexp.MustCompile(`^/api/me/(events/\d+/books/\d+/claim|import)$`)},
	{"GET", regexp.MustCompile(`^/api/(catalogs|collections|age-groups|updates|flags|content|discover)$`)}, // collections: always none
	{"", regexp.MustCompile(`^/api/push(/(subscribe|unsubscribe|test))?$`)},
	{"POST", regexp.MustCompile(`^/api/problems$`)},
}

// friendGuard refuses a friend everything outside friendRoutes.
func friendGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u := auth.UserFrom(r); u.IsFriend() && !friendMay(r.Method, r.URL.Path) {
			writeErr(w, http.StatusForbidden, "not available on a friend's account")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func friendMay(method, path string) bool {
	for _, fr := range friendRoutes {
		if (fr.methods == "" || fr.methods == method) && fr.path.MatchString(path) {
			return true
		}
	}
	return false
}

// friendOnly is for the friend's own versions of parents' tools (claiming
// an event book, importing a reading list): they always use her library.
func friendOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.UserFrom(r).IsFriend() {
			writeErr(w, http.StatusForbidden, "this is for friends' accounts")
			return
		}
		h(w, r)
	}
}

// friendCopies keeps the copies in a friend's own libraries.
func (s *Server) friendCopies(u *store.User, copies []store.BookCopy) []store.BookCopy {
	mine := []store.BookCopy{}
	for _, c := range copies {
		if cat, err := s.Store.CatalogByID(c.CatalogID); err == nil && cat.OwnerID != nil && *cat.OwnerID == u.ID {
			mine = append(mine, c)
		}
	}
	return mine
}
