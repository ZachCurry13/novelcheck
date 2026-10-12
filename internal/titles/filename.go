package titles

import (
	"regexp"
	"strings"
)

// Guess is a possible title and author read from a file name.
type Guess struct {
	Title, Author string
}

var (
	uuidTail   = regexp.MustCompile(`(?i)[\s_-]*[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\s*$`)
	bracketEnd = regexp.MustCompile(`\s*[\(\[][^\)\]]*(?:z-lib|libgen|epub|retail|ebook|oceanofpdf)[^\)\]]*[\)\]]\s*$`)
	fileExt    = regexp.MustCompile(`(?i)\.(epub|pdf|mobi|azw3?|fb2|cbz|txt|djvu)$`)
)

// FromFilename guesses what a book is from its file name: the name as a
// title, and, when it has "Author - Title" or "Title - Author" parts, both
// readings. The most likely reading comes first; callers should only accept
// a guess when the library agrees with the author too.
func FromFilename(name string) []Guess {
	s := fileExt.ReplaceAllString(strings.TrimSpace(name), "")
	s = uuidTail.ReplaceAllString(s, "")
	s = bracketEnd.ReplaceAllString(s, "")
	s = strings.NewReplacer("_", " ").Replace(s)
	s = strings.TrimSpace(spaces.ReplaceAllString(s, " "))
	if s == "" {
		return nil
	}
	out := []Guess{}
	parts := strings.Split(s, " - ")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) >= 2 && parts[0] != "" && parts[len(parts)-1] != "" {
		first, last := parts[0], parts[len(parts)-1]
		out = append(out, Guess{Title: first, Author: last}, Guess{Title: last, Author: first})
		if len(parts) > 2 { // "Author - Series 01 - Title"
			out = append(out, Guess{Title: parts[len(parts)-1], Author: first}, Guess{Title: parts[1], Author: first})
		}
	}
	return append(out, Guess{Title: s})
}
