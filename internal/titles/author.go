package titles

import (
	"regexp"
	"strings"
	"unicode"
)

// Titles that carry their author, the way file names and some stores write
// them: "Cornwell, Patricia_From Potter's Field", "Patricia Cornwell -
// Postmortem", "Postmortem by Patricia Cornwell". The author is taken off
// only when it is the book's own author, so a title that merely starts with
// a name ("Eleanor Oliphant Is Completely Fine") is left alone.

var (
	sepAfter  = `\s*(?:[_:;|]+|\s[-–—]+\s|[-–—]{1,2}|,|\.\s)\s*`                 // what joins a leading author to the title
	sepBefore = `\s*(?:[_|]+|\s[-–—]+\s|\s+by\s+|,\s*by\s+|\s*[(\[]\s*by\s+)\s*` // what joins a title to a trailing author
)

// authorForms are the ways an author's name is written: "Patricia
// Cornwell", "Cornwell, Patricia", "Cornwell Patricia", "Cornwell, P.",
// "P. Cornwell", and the last name alone.
func authorForms(author string) []string {
	author = strings.TrimSpace(strings.Split(author, "&")[0]) // the first of several
	words := strings.Fields(strings.NewReplacer(",", " ").Replace(author))
	if len(words) < 2 {
		return nil // one word could be anything in a title
	}
	first, last := words[0], words[len(words)-1]
	if strings.Contains(author, ",") { // stored "Cornwell, Patricia"
		first, last = words[1], words[0]
	}
	middle := ""
	if len(words) > 2 && !strings.Contains(author, ",") {
		middle = strings.Join(words[1:len(words)-1], " ") + " "
	}
	initial := string([]rune(first)[0]) + "."
	return []string{
		first + " " + middle + last, last + ", " + first + " " + strings.TrimSpace(middle), last + ", " + first, last + " " + first,
		last + ", " + initial, initial + " " + last, last,
	}
}

// WithoutAuthor takes the book's author off the front or the end of a
// title, and turns file-name underscores into spaces. A title that would be
// left without letters is kept as it was.
func WithoutAuthor(title, author string) string {
	t := strings.TrimSpace(title)
	fileName := !strings.Contains(t, " ") // "The_Hunger_Games": underscores stand for spaces
	for _, form := range authorForms(author) {
		f := regexp.QuoteMeta(strings.TrimSpace(form))
		f = strings.ReplaceAll(f, `\ `, `\s+`)
		f = strings.ReplaceAll(f, ` `, `\s+`)
		lead := regexp.MustCompile(`(?i)^` + f + sepAfter)
		if m := lead.FindString(t); m != "" && hasLetter(t[len(m):]) && startsWord(t[len(m):]) {
			t = t[len(m):]
			break
		}
		tail := regexp.MustCompile(`(?i)` + sepBefore + f + `\s*[)\]]?\s*$`)
		if loc := tail.FindStringIndex(t); loc != nil && hasLetter(t[:loc[0]]) {
			t = t[:loc[0]]
			break
		}
	}
	if fileName {
		t = strings.ReplaceAll(t, "_", " ")
	}
	t = strings.TrimSpace(spaces.ReplaceAllString(t, " "))
	t = strings.Trim(t, " -–—:,;|_")
	if !hasLetter(t) {
		return strings.TrimSpace(title)
	}
	return t
}

func startsWord(s string) bool {
	for _, r := range strings.TrimSpace(s) {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '"' || r == '\'' || r == '“' || r == '‘'
	}
	return false
}
