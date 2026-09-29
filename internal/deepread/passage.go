package deepread

// The reader in Review: an admin opens the part a scan flagged and reads it
// in the book, with the text before and after it, to decide for themselves.

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/epub"
)

// Passage is a stretch of a book: the part (Text) with some text Before and
// After it, in paragraphs, and where it is (in words).
type Passage struct {
	Before string `json:"before"`
	Text   string `json:"text"`
	After  string `json:"after"`
	From   int    `json:"from"`
	To     int    `json:"to"`
	Total  int    `json:"total"`
}

// Around is how much text the reader shows before and after the part.
const Around = 400

// Passage reads words [from, to) of the book, with Around words each side.
// A part from an older scan has no word positions: its label ("about 55%
// in", "Chapter 12") finds it instead.
func (r *Runner) Passage(bookID int64, from, to int, label string) (Passage, error) {
	path, ok := r.EPUBFor(bookID)
	if !ok {
		return Passage{}, ErrNoEPUB
	}
	sections, err := epub.Read(path)
	if err != nil {
		return Passage{}, err
	}
	if to <= from {
		from, to = locate(sections, label, r.partSize())
	}
	return cut(sections, from, to, Around), nil
}

var pctLabel = regexp.MustCompile(`about (\d+)% in`)

// locate finds a part by its label: a position ("about 55% in") or the
// chapter it starts in.
func locate(sections []epub.Section, label string, size int) (int, int) {
	at := 0
	for _, p := range Split(sections, size) { // the same parts as the scan, if the part size is unchanged
		if p.Label == label {
			return at, at + p.Words
		}
		at += p.Words
	}
	total := epub.Words(sections)
	if m := pctLabel.FindStringSubmatch(label); m != nil {
		pct, _ := strconv.Atoi(m[1])
		from := total * pct / 100
		return from, min(total, from+size)
	}
	title := strings.TrimSpace(strings.SplitN(strings.SplitN(label, " – ", 2)[0], " (", 2)[0])
	at = 0
	for _, s := range sections {
		n := len(strings.Fields(s.Text))
		if title != "" && s.Title == title {
			return at, min(at+size, total)
		}
		at += n
	}
	return 0, min(size, total)
}

// cut returns words [from, to) and around words each side, keeping
// paragraph and chapter breaks.
func cut(sections []epub.Section, from, to, around int) Passage {
	p := Passage{From: from, To: to, Total: epub.Words(sections)}
	var before, text, after strings.Builder
	pick := func(i int) *strings.Builder {
		switch {
		case i >= from-around && i < from:
			return &before
		case i >= from && i < to:
			return &text
		case i >= to && i < to+around:
			return &after
		}
		return nil
	}
	i := 0
	for _, s := range sections {
		for _, para := range strings.Split(s.Text, "\n") {
			words := strings.Fields(para)
			var last *strings.Builder
			for _, w := range words {
				if b := pick(i); b != nil {
					if b.Len() > 0 && b != last {
						b.WriteString("\n\n") // a new paragraph in this stretch
					} else if b.Len() > 0 {
						b.WriteByte(' ')
					}
					b.WriteString(w)
					last = b
				}
				i++
			}
		}
	}
	p.Before, p.Text, p.After = before.String(), text.String(), after.String()
	return p
}
