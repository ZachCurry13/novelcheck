package content

import (
	"regexp"
	"strings"
)

// Hide rules (a kid account's rules, or the Library's Hide boxes) are keys:
// an item ("murder"), a whole group ("g:violence", which also covers items
// added to it later) or one of the family's custom filters ("flag:swearing").
const (
	GroupPrefix  = "g:"
	CustomPrefix = "flag:"
)

var customKeyRE = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)

// ValidRule reports whether key is a hide rule.
func ValidRule(key string) bool {
	if g, ok := strings.CutPrefix(key, GroupPrefix); ok {
		return ItemsOf(g) != nil
	}
	if k, ok := strings.CutPrefix(key, CustomPrefix); ok {
		return customKeyRE.MatchString(k)
	}
	return items[key] != ""
}

// Expand turns hide rules into the item keys and custom filter keys they
// cover, without duplicates.
func Expand(rules []string) (itemKeys, customKeys []string) {
	seen := map[string]bool{}
	add := func(list *[]string, k string) {
		if !seen[k] {
			seen[k] = true
			*list = append(*list, k)
		}
	}
	for _, r := range rules {
		switch {
		case strings.HasPrefix(r, GroupPrefix):
			for _, k := range ItemsOf(strings.TrimPrefix(r, GroupPrefix)) {
				add(&itemKeys, k)
			}
		case strings.HasPrefix(r, CustomPrefix):
			add(&customKeys, strings.TrimPrefix(r, CustomPrefix))
		case items[r] != "":
			add(&itemKeys, r)
		}
	}
	return itemKeys, customKeys
}

// Presets are the starter sets the kids' one-click presets (and new kid
// accounts of an age group) hide. Parents can change them afterwards.
var (
	StrictPreset = []string{"strong_profanity", "sexual_language", "slurs", "torture", "sexual_violence",
		"child_violence", "dismemberment", "mutilation", "graphic_deaths", "drug_dealing", "overdose", "self_harm"}
	YoungPreset = append(append([]string{}, StrictPreset...), "profanity", "blasphemy", "crude_humor", "murder",
		"gun_violence", "domestic_violence", "graphic_injuries", "body_horror", "organs", "corpses",
		"underage_drinking", "vaping", "marijuana", "drugs", "prescription_misuse", "addiction",
		"eating_disorders", "abuse")
)

// Amounts of a group in a Deep-scanned book.
const (
	None = iota
	ALittle
	Some
	ALot
)

// AmountLabels names the amounts for display.
var AmountLabels = []string{"", "A little", "Some", "A lot"}

// Amount is how much of a book a group takes up: more than a third of its
// parts is "A lot", one or two parts "A little", anything between "Some".
func Amount(parts, total int) int {
	switch {
	case parts <= 0 || total <= 0:
		return None
	case parts*3 > total:
		return ALot
	case parts <= 2:
		return ALittle
	default:
		return Some
	}
}

// PromptList is the catalog for the AI: one line per group, each item as
// "key (what to look for)".
func PromptList() string {
	var b strings.Builder
	for _, g := range Groups {
		b.WriteString("- " + g.Label + ": ")
		for i, it := range g.Items {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(it.Key)
			if it.Hint != "" {
				b.WriteString(" (" + it.Hint + ")")
			}
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
