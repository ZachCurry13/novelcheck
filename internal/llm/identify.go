package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// IdentifySystem asks which book a file is, from its first pages.
const IdentifySystem = `You are given the pages of an e-book file that say what it is: its title page, its copyright page and, when it has neither, the opening of the text. Say which book it is.
Answer with one JSON object: {"title": "...", "author": "...", "certain": true}.
Use the title and the author exactly as the pages print them (the title page or the copyright page), without a series name or a subtitle that is not part of the title.
If the pages do not say what the book is (the file starts straight in the story, or only has a chapter), answer {"title": "", "author": "", "certain": false}. Never guess from the story alone, and never use a title that is not written in the text.`

// IdentifyUser is the question for one file: what its package says (it may be wrong) and the pages that say what it is.
func IdentifyUser(metaTitle, metaAuthor, publisher, titlePage, copyrightPage, opening string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "The file's own label says: title %q, author %q, publisher %q (it may be wrong).\n", metaTitle, metaAuthor, publisher)
	for _, part := range []struct{ name, text string }{{"TITLE PAGE", titlePage}, {"COPYRIGHT PAGE", copyrightPage}, {"OPENING OF THE TEXT", opening}} {
		if strings.TrimSpace(part.text) != "" {
			fmt.Fprintf(&b, "\n%s:\n%s\n", part.name, part.text)
		}
	}
	return b.String()
}

// Identity is the AI's answer.
type Identity struct {
	Title   string `json:"title"`
	Author  string `json:"author"`
	Certain bool   `json:"certain"`
}

// ParseIdentity reads the answer.
func ParseIdentity(out string) (Identity, error) {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return Identity{}, errors.New("no JSON object in the answer")
	}
	var v Identity
	if err := json.Unmarshal([]byte(out[start:end+1]), &v); err != nil {
		return Identity{}, fmt.Errorf("invalid JSON: %w", err)
	}
	v.Title, v.Author = strings.TrimSpace(v.Title), strings.TrimSpace(v.Author)
	return v, nil
}
