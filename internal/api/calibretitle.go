package api

import (
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// sameCalibreTitle says whether a title the Content server reports is the book NovelCheck means. The check
// is there so the wrong library is never touched; it must not trip over a subtitle or series note that only
// one side carries ("A Kiss of Shadows: A Merry Gentry Novel" is "A Kiss of Shadows").
func sameCalibreTitle(calibre, ours string) bool {
	a, _, _ := strings.Cut(store.NormKey(calibre, ""), "|")
	b, _, _ := strings.Cut(store.NormKey(ours, ""), "|")
	if a == b || subtitled(calibre, ours) || subtitled(ours, calibre) {
		return true
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	return len(a) >= 5 && strings.Contains(b, a)
}

// subtitled: long is short plus a subtitle or a note ("Rant: An Oral Biography of Buster Casey" and "Rant";
// "It (Movie Tie-in)" and "It"). A short title must be followed by a separator, so "It Ends with Us" is not "It".
func subtitled(long, short string) bool {
	l, s := strings.ToLower(strings.TrimSpace(long)), strings.ToLower(strings.TrimSpace(short))
	if s == "" || !strings.HasPrefix(l, s) {
		return false
	}
	rest := strings.TrimSpace(l[len(s):])
	for _, sep := range []string{":", "-", "–", "—", "(", "[", ";"} {
		if strings.HasPrefix(rest, sep) {
			return true
		}
	}
	return false
}
