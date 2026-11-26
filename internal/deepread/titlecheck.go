package deepread

import (
	"strings"
	"unicode"
)

// A Deep Scan reads the book's EPUB and rates what it finds; if that file is a different book from the one NovelCheck
// has under this name (a wrong file in Calibre, a mislabeled upload), the rating would land on the wrong book. The file's
// own title is compared with the library's before any reading: it must fit, loosely.

var titleStop = map[string]bool{"the": true, "a": true, "an": true, "of": true, "and": true, "book": true, "novel": true, "series": true}

// titleWords is the lowercase words of a title without the little ones ("The Gunslinger: Dark Tower #1" -> gunslinger dark tower 1).
func titleWords(t string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(t), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
		if !titleStop[w] {
			out = append(out, w)
		}
	}
	return out
}

// titleFits says whether the title inside an EPUB can be the library's book. It is lenient on purpose (a subtitle, a
// series note, "Dune" and "Dune (Dune Chronicles #1)"); a file with no real title of its own always fits.
func titleFits(library, file string) bool {
	f := titleWords(file)
	switch strings.ToLower(strings.TrimSpace(file)) {
	case "", "unknown", "untitled", "title", "book", "ebook", "document", "calibre":
		return true
	}
	l := titleWords(library)
	if len(f) == 0 || len(l) == 0 {
		return true
	}
	in := func(a, b []string) int { // how many of a's words b has
		have := map[string]bool{}
		for _, w := range b {
			have[w] = true
		}
		n := 0
		for _, w := range a {
			if have[w] {
				n++
			}
		}
		return n
	}
	small, big := f, l
	if len(small) > len(big) {
		small, big = big, small
	}
	return float64(in(small, big)) >= 0.5*float64(len(small))
}

// authorFits says whether the author inside the EPUB can be the library's author: they share a word of two letters or more
// ("Stephen King" and "King, Stephen"). A file with no author of its own fits.
func authorFits(library, file string) bool {
	f := titleWords(file)
	if len(f) == 0 {
		return true
	}
	have := map[string]bool{}
	for _, w := range titleWords(library) {
		have[w] = true
	}
	for _, w := range f {
		if len([]rune(w)) >= 2 && have[w] {
			return true
		}
	}
	return false
}

// wrongFile compares what the file says with the library's book: stop is true when both the title and the author differ
// (a different book altogether); warn is true when only the title differs (the same author: maybe another title for the
// same book, maybe the wrong book of a series).
func wrongFile(libTitle, libAuthor, fileTitle, fileAuthor string) (stop, warn bool) {
	if titleFits(libTitle, fileTitle) {
		return false, false
	}
	if !authorFits(libAuthor, fileAuthor) {
		return true, false
	}
	return false, true
}

// TitleFits is titleFits for other packages: can the title read from a file be this library title?
func TitleFits(library, file string) bool { return titleFits(library, file) }

// SameTitleWords says whether two titles are the same apart from little words and punctuation ("The Gunslinger" and
// "Gunslinger, The"): the strict test for joining two books, where TitleFits is the loose one.
func SameTitleWords(a, b string) bool {
	x, y := titleWords(a), titleWords(b)
	if len(x) == 0 || len(x) != len(y) {
		return false
	}
	have := map[string]int{}
	for _, w := range x {
		have[w]++
	}
	for _, w := range y {
		if have[w]--; have[w] < 0 {
			return false
		}
	}
	return true
}

// AuthorFits is authorFits for other packages.
func AuthorFits(library, file string) bool { return authorFits(library, file) }
