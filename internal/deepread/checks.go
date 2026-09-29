package deepread

// Checks on what the AI found in each part, so one misread part (a battle,
// a monster, a tense moment) can't decide a whole book:
//   - a part rated 3 or more gets a second, stricter look (how far the scene
//     goes on the page) and keeps only what that confirms: 4 or more needs a
//     sex scene described over 3 sentences that really are in the text, and
//     a second opinion that agrees;
//   - nudity, solo acts and heavy innuendo only count in parts with
//     confirmed sexual content;
//   - in books of more than 4 parts, a flag needs 2 parts;
//   - level 5 needs 3 parts of level 4 or more.
//   - content items (package content) count from a single part, and each
//     group gets an amount from how many parts it appears in.

import (
	"context"
	"slices"

	"github.com/zachcurry13/novelcheck/internal/content"
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
		res.Level = llm.ConfirmedLevel(res.Level, c, llm.VerifyOpenings(p.Text, c.SexSentences))
		if c.Evidence != "" {
			res.Evidence = c.Evidence
		}
		res.Scene = c.Scene
		if res.Level >= 4 {
			s, _, err := ask(ctx, r, book.ID, llm.DeepSecondSystem, llm.DeepSecondUser(book.Title, book.Author, p.Label, p.Text), llm.ParseSecond)
			if err != nil {
				return res, err
			}
			if !s.Described {
				res.Level = 3 // the second opinion sees it led up to, hinted at or cut away
			}
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
	var found []string
	groupParts := map[string]int{}
	level, explicitParts, at := 0, 0, 0
	var notes []Note
	for i, res := range results {
		from := at
		at += parts[i].Words
		level = max(level, res.Level)
		if res.Level >= 4 {
			explicitParts++
		}
		for k, on := range map[string]bool{"nudity": res.Nudity, "solo": res.SoloActs, "innuendo": res.HeavyInnuendo,
			"fantasy": res.PlayfulFantasy, "occult": res.DarkOccult, "demonic": res.DemonicPresence} {
			if on {
				count[k]++
			}
		}
		for _, k := range res.CustomFlags {
			count["custom:"+k]++
		}
		groups := map[string]bool{}
		for _, k := range res.Content {
			if !slices.Contains(found, k) {
				found = append(found, k) // one part is enough: a single serious scene still counts
			}
			groups[content.GroupOf(k)] = true
		}
		for g := range groups {
			groupParts[g]++
		}
		if res.Note != "" || res.Level >= 2 {
			note := res.Note
			if res.Level >= 3 && res.Evidence != "" {
				note = res.Evidence
			}
			n := Note{Label: parts[i].Label, Level: res.Level, Note: note, From: from, To: at}
			if res.Level >= 3 {
				n.Scene = res.Scene
			}
			notes = append(notes, n)
		}
	}
	if level == 5 && explicitParts < 3 {
		level = 4 // "very explicit" means frequent, not one scene
	}
	has := func(k string) bool { return count[k] >= need }
	a := store.Analysis{SpiceLevel: &level, Nudity: has("nudity"), SoloActs: has("solo"), HeavyInnuendo: has("innuendo"),
		PlayfulFantasy: has("fantasy"), DarkOccult: has("occult") || has("demonic"), DemonicPresence: has("demonic"),
		Content: found, ContentSource: store.SourceDeep, ContentAmounts: map[string]int{}}
	for g, n := range groupParts {
		a.ContentAmounts[g] = content.Amount(n, len(results))
	}
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

// onePassage reports whether a raise to 4 or more rests on a single part:
// an admin decides those too, whatever the size of the raise.
func onePassage(prev *int, level int, results []llm.PartResult) bool {
	if prev == nil || level < 4 || level <= *prev {
		return false
	}
	n := 0
	for _, r := range results {
		if r.Level >= 4 {
			n++
		}
	}
	return n == 1
}
