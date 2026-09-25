package deepread

// Checks on what the AI found in each part, so one misread part (a battle,
// a monster, a tense moment) can't decide a whole book:
//   - a part rated 3 or more gets a second, stricter look (yes/no questions
//     about what is on the page) and keeps only what that confirms;
//   - nudity, solo acts and heavy innuendo only count in parts with
//     confirmed sexual content;
//   - in books of more than 4 parts, a flag needs 2 parts;
//   - level 5 needs 3 parts of level 4 or more.

import (
	"context"
	"slices"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// confirm gives a part rated 3 or more the second look.
func (r *Runner) confirm(ctx context.Context, book *store.Book, p Part, res llm.PartResult) (llm.PartResult, error) {
	if res.Level >= 3 {
		c, _, err := ask(ctx, r, book.ID, llm.DeepConfirmSystem, llm.DeepConfirmUser(book.Title, book.Author, p.Label, p.Text), llm.ParseConfirm)
		if err != nil {
			return res, err
		}
		res.Level = llm.ConfirmedLevel(res.Level, c)
		if c.Evidence != "" {
			res.Evidence = c.Evidence
		}
	}
	if res.Level < 3 {
		res.Nudity, res.SoloActs, res.HeavyInnuendo = false, false, false
	}
	return res, nil
}

// combine turns the checked parts into one rating, plus notes for parents.
func combine(parts []Part, results []llm.PartResult) (store.Analysis, []Note) {
	need := 1
	if len(results) > 4 {
		need = 2
	}
	count := map[string]int{}
	level, explicitParts := 0, 0
	var notes []Note
	for i, res := range results {
		level = max(level, res.Level)
		if res.Level >= 4 {
			explicitParts++
		}
		for k, on := range map[string]bool{"nudity": res.Nudity, "solo": res.SoloActs, "innuendo": res.HeavyInnuendo,
			"fantasy": res.PlayfulFantasy, "occult": res.DarkOccult, "demonic": res.DemonicPresence, "lgbtq": res.LGBTQ} {
			if on {
				count[k]++
			}
		}
		for _, k := range res.CustomFlags {
			count["custom:"+k]++
		}
		if res.Note != "" || res.Level >= 2 {
			note := res.Note
			if res.Level >= 3 && res.Evidence != "" {
				note = res.Evidence
			}
			notes = append(notes, Note{Label: parts[i].Label, Level: res.Level, Note: note})
		}
	}
	if level == 5 && explicitParts < 3 {
		level = 4 // "very explicit" means frequent, not one scene
	}
	has := func(k string) bool { return count[k] >= need }
	a := store.Analysis{SpiceLevel: &level, Nudity: has("nudity"), SoloActs: has("solo"), HeavyInnuendo: has("innuendo"),
		PlayfulFantasy: has("fantasy"), DarkOccult: has("occult") || has("demonic"), DemonicPresence: has("demonic"), LGBTQContent: has("lgbtq")}
	for _, res := range results {
		for _, k := range res.CustomFlags {
			if has("custom:"+k) && !slices.Contains(a.CustomFlags, k) {
				a.CustomFlags = append(a.CustomFlags, k)
			}
		}
	}
	return a, notes
}

// bigJump reports whether a result raises a rated book by 2 or more
// levels; those wait for an admin instead of applying.
func bigJump(prev *int, level int) bool { return prev != nil && level >= *prev+2 }
