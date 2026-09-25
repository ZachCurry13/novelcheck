package llm

// A second, stricter look at a part the AI rated 3 or more peppers, so one
// misread part (a battle, a monster, a death) can't make a whole book
// "explicit". It asks plain yes/no questions about what is on the page and
// maps the answers onto the pepper scale.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// DeepConfirmSystem is the instruction for the second look.
const DeepConfirmSystem = `You check one part of a book for a parent. Answer plain yes/no questions about what happens ON THE PAGE in this text.
Only romantic and sexual content counts. Violence, fighting, killing, monsters, death, injury, fear, magic, science and religion are NOT sexual content.
Don't guess from what might happen later, earlier or off the page. No explicit or graphic language. JSON only.`

// DeepConfirmUser hands over the part and the questions.
func DeepConfirmUser(title, author, label, text string) string {
	return fmt.Sprintf(`Book: %s by %s
Part: %s

TEXT:
%s

Answer with JSON only:
{"sex_on_page": true | false, "foreplay_on_page": true | false, "kissing": true | false, "romance": true | false, "evidence": "..."}
- sex_on_page: characters have sex in this text and it is described, not just implied
- foreplay_on_page: heavy sexual foreplay or clearly sexual touching in this text, short of sex
- kissing: characters kiss in this text
- romance: crushes, attraction, flirting or dating in this text
- evidence: if sex_on_page or foreplay_on_page, one modest phrase naming it; otherwise ""`, title, orUnknown(author), label, text)
}

// Confirmation is the answer to the second look.
type Confirmation struct {
	SexOnPage bool   `json:"sex_on_page"`
	Foreplay  bool   `json:"foreplay_on_page"`
	Kissing   bool   `json:"kissing"`
	Romance   bool   `json:"romance"`
	Evidence  string `json:"evidence"`
}

// ParseConfirm reads the answer.
func ParseConfirm(out string) (Confirmation, error) {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return Confirmation{}, errors.New("no JSON object in the answer")
	}
	var c Confirmation
	if err := json.Unmarshal([]byte(out[start:end+1]), &c); err != nil {
		return Confirmation{}, fmt.Errorf("invalid JSON: %w", err)
	}
	c.Evidence = ShortNote(c.Evidence)
	return c, nil
}

// ConfirmedLevel is the pepper level a part keeps after the second look:
// its own level only with sex described on the page (and named); heavy
// foreplay alone makes it at most 3, kissing 2, romance 1, and nothing 0.
func ConfirmedLevel(claimed int, c Confirmation) int {
	switch {
	case c.SexOnPage && c.Evidence != "":
		return claimed
	case c.Foreplay && c.Evidence != "":
		return min(claimed, 3)
	case c.Kissing:
		return min(claimed, 2)
	case c.Romance:
		return min(claimed, 1)
	}
	return 0
}
