package api

import (
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// sameCalibreTitle says whether a title the Content server reports is the book NovelCheck means. The check
// is there so the wrong library is never touched; it must not trip over a subtitle or series note that only
// one side carries ("A Kiss of Shadows: A Merry Gentry Novel" is "A Kiss of Shadows").
//
// A Calibre entry made from a file name carries its author after a dash ("An American Sickness - Elisabeth
// Rosenthal", "Bad Beginning - Snicket, Lemony"): it is compared without that part too, and without a leading
// "The" on either side, since those are the very titles Tidy titles is meant to fix.
func sameCalibreTitle(calibre, ours string) bool {
	if sameTitleOnce(calibre, ours) {
		return true
	}
	bare := func(t string) string {
		t = store.TidyBookTitle(t)
		for _, a := range []string{"the ", "a ", "an "} {
			if len(t) > len(a) && strings.EqualFold(t[:len(a)], a) {
				return t[len(a):]
			}
		}
		return t
	}
	c := calibre
	if i := strings.LastIndex(c, " - "); i > 0 {
		c = c[:i]
	}
	return sameTitleOnce(bare(c), bare(ours))
}

func sameTitleOnce(calibre, ours string) bool {
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
