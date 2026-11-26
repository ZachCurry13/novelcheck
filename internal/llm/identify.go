package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// IdentifySystem asks which book a file is, from its first pages.
const IdentifySystem = `You are given the first pages of an e-book file: a cover page, the title page, the copyright page, the start of the text. Say which book it is.
Answer with one JSON object: {"title": "...", "author": "...", "certain": true}.
Use the title and the author exactly as the pages print them (the title page or the copyright page), without a series name or a subtitle that is not part of the title.
If the pages do not say what the book is (the file starts straight in the story, or only has a chapter), answer {"title": "", "author": "", "certain": false}. Never guess from the story alone, and never use a title that is not written in the text.`

// IdentifyUser is the question for one file: what its package says (it may be wrong) and its first pages.
func IdentifyUser(hintTitle, hintAuthor, text string) string {
	return fmt.Sprintf("The file's own label says: %q by %q (it may be wrong).\n\nFirst pages:\n%s", hintTitle, hintAuthor, text)
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
