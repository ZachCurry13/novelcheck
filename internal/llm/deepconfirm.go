package llm

// A second, stricter look at a part the AI rated 3 or more peppers, so one
// misread part (a battle, a monster, a scene that fades out) can't make a
// whole book "explicit". The AI names how far the scene goes on a fixed
// scale, and for a described sex scene gives the first words of the
// sentences that describe it; NovelCheck checks those words are really in
// the text (VerifyOpenings) before a part can stay at 4 or more. A second
// opinion (deepsecond.go) must agree too.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// DeepConfirmSystem is the instruction for the second look.
const DeepConfirmSystem = `You check one part of a book for a parent. Say how far the romantic or sexual content goes ON THE PAGE in this text.
Only romantic and sexual content counts. Violence, fighting, killing, monsters, death, injury, fear, magic, science and religion are NOT sexual content.
A sex scene that stops, cuts away (a new chapter, "the next morning") or only hints ("we fell into bed", "we didn't sleep until dawn") is implied_sex, not described.
Kissing, touching and undressing are not the sex act itself. Don't guess from what might happen later, earlier or off the page.
No explicit or graphic language in your own words. JSON only.`

// HowFar is the scale the second look answers on, least to most.
var HowFar = []string{"none", "romance", "kissing", "making_out", "sexual_touching", "implied_sex", "brief_sex", "detailed_sex"}

// DeepConfirmUser hands over the part and the questions.
func DeepConfirmUser(title, author, label, text string) string {
	return fmt.Sprintf(`Book: %s by %s
Part: %s

TEXT:
%s

Answer with JSON only:
{"how_far": "%s", "sex_sentences": ["first four words", ...], "evidence": "...", "scene": "..."}
- how_far: the furthest this text goes, one of:
  none; romance (crushes, attraction, flirting, dating); kissing; making_out (heavy kissing, clear sexual desire);
  sexual_touching (undressing, touching under clothes, foreplay); implied_sex (sex happens but off the page, cut away or only hinted);
  brief_sex (the sex act itself is described in one or two sentences); detailed_sex (the sex act itself is described over three or more sentences)
- sex_sentences: only for brief_sex or detailed_sex: for each sentence that describes the sex act itself, its first four words exactly as written in the TEXT (at most 8); otherwise []
- evidence: unless none, romance or kissing, one modest phrase naming what happens; otherwise ""
- scene: unless none, 2 or 3 modest sentences for a parent: who is involved, what they do, how far it goes, and whether it is described or cut away; otherwise ""`,
		title, orUnknown(author), label, text, strings.Join(HowFar, `" | "`))
}

// Confirmation is the answer to the second look.
type Confirmation struct {
	HowFar       string   `json:"how_far"`
	SexSentences []string `json:"sex_sentences"`
	Evidence     string   `json:"evidence"`
	Scene        string   `json:"scene"`
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
	c.HowFar = strings.ToLower(strings.TrimSpace(c.HowFar))
	c.Evidence = ShortNote(c.Evidence)
	c.Scene = ShortScene(c.Scene)
	return c, nil
}

// MinDescribed is how many sentences (found in the text) a sex scene needs
// to count as described on the page.
const MinDescribed = 3

// ConfirmedLevel is the pepper level a part keeps after the second look:
// its own level only for a sex scene described over MinDescribed sentences
// that are really in the text (verified); a scene cut away, implied,
// described in a line or two, or sexual touching or making out (named)
// makes it at most 3; kissing 2; romance 1; nothing 0.
func ConfirmedLevel(claimed int, c Confirmation, verified int) int {
	named := c.Evidence != "" || c.Scene != ""
	switch c.HowFar {
	case "detailed_sex":
		if verified >= MinDescribed {
			return claimed
		}
		return min(claimed, 3) // said to be described, but the text doesn't show it
	case "brief_sex", "implied_sex", "sexual_touching", "making_out":
		if named {
			return min(claimed, 3)
		}
		return min(claimed, 2)
	case "kissing":
		return min(claimed, 2)
	case "romance":
		return min(claimed, 1)
	}
	return 0
}

// ShortScene caps the scene description for a parent.
func ShortScene(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 400 {
		s = string(r[:399]) + "…"
	}
	return s
}
