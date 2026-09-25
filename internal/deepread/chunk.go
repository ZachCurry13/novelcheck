// Package deepread runs Deep Scans: the AI reads a book's whole EPUB in
// parts, notes what each part contains, and the parts are combined into a
// rating that replaces the one guessed from the blurb.
package deepread

import (
	"fmt"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/epub"
)

// Words per part: local models often have a small default context window
// (Ollama: a few thousand tokens), cloud models take much more at once.
const (
	LocalWordsPerPart = 2000
	CloudWordsPerPart = 8000
)

// Part is the slice of the book sent to the AI in one call.
type Part struct {
	Label string // e.g. "Chapter 3 – Chapter 5", or "about 40% in"
	Text  string
	Words int
}

// Split groups sections into parts of about size words, keeping chapters
// whole when they fit and cutting longer ones into pieces.
func Split(sections []epub.Section, size int) []Part {
	var parts []Part
	var cur Part
	var titles []string
	total, seen := 0, 0
	for _, s := range sections {
		total += len(strings.Fields(s.Text))
	}
	flush := func() {
		if cur.Words == 0 {
			return
		}
		cur.Label = label(titles, seen-cur.Words, total)
		parts = append(parts, cur)
		cur, titles = Part{}, nil
	}
	for _, s := range sections {
		words := strings.Fields(s.Text)
		if len(words) > size { // a long chapter: its own pieces
			flush()
			for i, n := 0, (len(words)+size-1)/size; i < n; i++ {
				piece := words[i*size : min((i+1)*size, len(words))]
				t := s.Title
				if t != "" {
					t = fmt.Sprintf("%s (%d/%d)", t, i+1, n)
				}
				cur, titles = Part{Text: strings.Join(piece, " "), Words: len(piece)}, []string{t}
				seen += len(piece)
				flush()
			}
			continue
		}
		if cur.Words+len(words) > size {
			flush()
		}
		if cur.Text != "" {
			cur.Text += "\n\n"
		}
		cur.Text += s.Text
		cur.Words += len(words)
		titles = append(titles, s.Title)
		seen += len(words)
	}
	flush()
	return parts
}

// label names a part by its first and last chapter headings, or by how far
// into the book it starts.
func label(titles []string, start, total int) string {
	var named []string
	for _, t := range titles {
		if t != "" {
			named = append(named, t)
		}
	}
	switch len(named) {
	case 0:
		return fmt.Sprintf("about %d%% in", start*100/max(total, 1))
	case 1:
		return named[0]
	default:
		return named[0] + " – " + named[len(named)-1]
	}
}

// Estimate is what a Deep Scan should cost before it starts.
type Estimate struct {
	Words  int `json:"words"`
	Parts  int `json:"parts"`
	Tokens int `json:"tokens"` // prompt + answer tokens, all calls
	Output int `json:"output"` // of which answer tokens
}

// Per call: instructions sent with every part, and the note that comes back.
const (
	promptTokens = 900
	answerTokens = 150
	wrapUpTokens = 700
)

// EstimateParts sums up the calls for these parts (about 4 tokens per 3 words).
func EstimateParts(parts []Part) Estimate {
	e := Estimate{Parts: len(parts)}
	for _, p := range parts {
		e.Words += p.Words
	}
	e.Output = len(parts)*answerTokens + 100
	e.Tokens = e.Words*4/3 + len(parts)*promptTokens + wrapUpTokens + e.Output
	return e
}
