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
	if a == b {
		return true
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	return len(a) >= 5 && strings.Contains(b, a)
}
