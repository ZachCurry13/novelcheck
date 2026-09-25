// Package titles tidies book titles that carry series numbering, such as
// "01 - The Hobbit", "Book 2: Dune Messiah" or "Dune Messiah (Dune, #2)",
// so NovelCheck can show the plain title and the book's number in its series.
//
// It is deliberately cautious: a number is only taken out when something
// marks it as numbering (a leading zero, a separator, "Book", "#"), so titles
// like "1984", "13 Reasons Why" or "4-Hour Workweek" are left alone.
package titles

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Parsed is a title with its series numbering taken out.
type Parsed struct {
	Title  string  // the plain title
	Series string  // series name, when the title names one
	Index  float64 // number in the series; 0 when none was found
}

var wordNums = map[string]float64{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
	"seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13,
	"fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17, "eighteen": 18, "nineteen": 19, "twenty": 20}

const (
	digits = `\d{1,3}(?:\.\d{1,2})?`
	number = `(` + digits + `|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty)`
	label  = `(?:book|bk\.?|volume|vol\.?|novel|tome)`
	dash   = `[-–—]`
)

var (
	// "01 Title", "01 - Title", "002_Title": a leading zero means a track number.
	zeroLead = regexp.MustCompile(`^(0\d{1,2})(?:\s*(?:` + dash + `|[_.:)])\s*|\s+)`)
	// "1 - Title", "1. Title", "1_Title", "1) Title" (not "13 Reasons Why").
	plainLead = regexp.MustCompile(`^(` + digits + `)(?:\s+` + dash + `\s*|\s*` + dash + `\s+|\s*_+\s*|\.\s+|\)\s*)`)
	// "#2 Title", "#2: Title".
	hashLead = regexp.MustCompile(`^#\s*(` + digits + `)\s*(?:` + dash + `|[_.:)])?\s+`)
	// "Book 2: Title", "Vol. 3 - Title", "Book Two Title".
	labelLead = regexp.MustCompile(`(?i)^` + label + `\s*` + number + `(?:\s*(?:` + dash + `|[_.:)])\s*|\s+)`)
	// "Title (Series, #2)", "Title (Series Book 2)", "Title [Book 2 of 3]".
	parenTail = regexp.MustCompile(`(?i)\s*[(\[]\s*(?:([^()\[\]]*?)[\s,;:–—-]*)?(?:#|\b` + label + `|\bno\.)\s*` + number + `(?:\s+of\s+\w+)?\s*[)\]]\s*$`)
	// "Title, Book 2", "Title: Book Two", "Title - Vol 2".
	labelTail = regexp.MustCompile(`(?i)(?:\s*[,:;]|\s+` + dash + `)\s*` + label + `\s*` + number + `\s*$`)
	spaces    = regexp.MustCompile(`\s+`)
)

// Parse takes the series numbering out of a title. When nothing applies,
// Title is the original (trimmed) and Index is 0.
func Parse(raw string) Parsed {
	title := strings.TrimSpace(spaces.ReplaceAllString(raw, " "))
	out := Parsed{Title: title}
	rest := title
	if m := parenTail.FindStringSubmatch(rest); m != nil && hasLetter(rest[:len(rest)-len(m[0])]) {
		rest, out.Index = rest[:len(rest)-len(m[0])], toNumber(m[2])
		out.Series = strings.Trim(m[1], " ,;:-–—")
	} else if m := labelTail.FindStringSubmatch(rest); m != nil && hasLetter(rest[:len(rest)-len(m[0])]) {
		rest, out.Index = rest[:len(rest)-len(m[0])], toNumber(m[1])
	}
	for _, re := range []*regexp.Regexp{zeroLead, hashLead, labelLead, plainLead} {
		if m := re.FindStringSubmatch(rest); m != nil && hasLetter(rest[len(m[0]):]) {
			if out.Index == 0 {
				out.Index = toNumber(m[1])
			}
			rest = rest[len(m[0]):]
			break
		}
	}
	rest = strings.Trim(rest, " -–—_:,;")
	if !hasLetter(rest) {
		return Parsed{Title: title}
	}
	out.Title = rest
	return out
}

// Clean returns just the plain title.
func Clean(raw string) string { return Parse(raw).Title }

// FormatIndex writes a series number the way Calibre shows it: 2, 2.5.
func FormatIndex(i float64) string { return strconv.FormatFloat(i, 'f', -1, 64) }

func toNumber(s string) float64 {
	if n, ok := wordNums[strings.ToLower(s)]; ok {
		return n
	}
	n, _ := strconv.ParseFloat(s, 64)
	return n
}

func hasLetter(s string) bool {
	return strings.IndexFunc(s, unicode.IsLetter) >= 0
}
