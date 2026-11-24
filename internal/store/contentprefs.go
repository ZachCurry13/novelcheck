package store

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/content"
)

// What each grown-up chooses per content label (Profile → What to hide or flag): Hide (the book stays out of their
// pages: the existing hide switches, spice limit and content rules), Heads-up (the book shows, with a notice at the
// top of its page) or nothing ("Don't mention"). Kids' accounts keep the rules a parent sets.

// BookFlagKeys are the labels that are a rating's own yes/no answers rather than content items.
var BookFlagKeys = []string{"nudity", "solo_acts", "heavy_innuendo", "dark_occult"}

// ContentPrefs is a person's choices: content keys (items, "g:<group>", "flag:<custom>", BookFlagKeys, "unrated").
type ContentPrefs struct {
	Hide      []string `json:"hide"`
	Warn      []string `json:"warn"`
	HideAbove int      `json:"hide_above"` // books above this spice level are hidden; -1 = none
	WarnFrom  int      `json:"warn_from"`  // a heads-up from this spice level on; -1 = none
}

func validPref(k string) bool {
	return content.ValidRule(k) || k == "unrated" || contains(BookFlagKeys, k)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ContentPrefsOf reads a person's choices from their account.
func ContentPrefsOf(u *User) ContentPrefs {
	p := ContentPrefs{Hide: []string{}, Warn: []string{}, HideAbove: u.MaxSpice, WarnFrom: u.WarnSpice}
	for k, on := range map[string]bool{"nudity": u.HideNudity, "solo_acts": u.HideSoloActs, "heavy_innuendo": u.HideInnuendo,
		"dark_occult": u.HideDarkOccult, "unrated": u.HideUnrated} {
		if on {
			p.Hide = append(p.Hide, k)
		}
	}
	p.Hide = append(p.Hide, u.HiddenContent...)
	for _, k := range strings.Split(u.ContentWarn, ",") {
		if k = strings.TrimSpace(k); k != "" {
			p.Warn = append(p.Warn, k)
		}
	}
	if u.HideOpenDoor && (p.HideAbove < 0 || p.HideAbove > 3) {
		p.HideAbove = 3 // the old "Hide open door" switch is spice above 3
	}
	return p
}

// SaveContentPrefs stores a person's choices (unknown keys are dropped; a key both hidden and flagged is hidden).
func (s *Store) SaveContentPrefs(userID int64, p ContentPrefs) error {
	if p.HideAbove < -1 || p.HideAbove > 5 || p.WarnFrom < -1 || p.WarnFrom > 5 {
		return fmt.Errorf("spice levels go from 0 to 5")
	}
	hide := map[string]bool{}
	var rules []string
	for _, k := range p.Hide {
		if validPref(k) && !hide[k] {
			hide[k] = true
			if content.ValidRule(k) {
				rules = append(rules, k)
			}
		}
	}
	var warn []string
	for _, k := range p.Warn {
		if validPref(k) && !hide[k] && !contains(warn, k) {
			warn = append(warn, k)
		}
	}
	if _, err := s.DB.Exec(`UPDATE users SET hide_nudity = ?, hide_solo_acts = ?, hide_innuendo = ?, hide_dark_occult = ?, hide_unrated = ?,
		hide_open_door = ?, max_spice = ?, content_warn = ?, warn_spice = ? WHERE id = ?`,
		hide["nudity"], hide["solo_acts"], hide["heavy_innuendo"], hide["dark_occult"], hide["unrated"],
		p.HideAbove >= 0 && p.HideAbove <= 3, p.HideAbove, strings.Join(warn, ","), p.WarnFrom, userID); err != nil {
		return err
	}
	return s.setHiddenContent(userID, rules)
}

// HeadsUp lists what in b this person asked to be told about: plain labels, and for spice a Deep Scan's places.
func (s *Store) HeadsUp(u *User, b *Book) []string {
	var out []string
	if u.WarnSpice >= 0 && b.SpiceLevel != nil && *b.SpiceLevel >= u.WarnSpice {
		line := fmt.Sprintf("Spice level %d", *b.SpiceLevel)
		if where := s.deepPlaces(b.ID, u.WarnSpice); where != "" {
			line += ": " + where
		}
		out = append(out, line)
	}
	if u.ContentWarn == "" {
		return out
	}
	have := map[string]bool{}
	for _, kv := range strings.Split(b.Content, ",") {
		if k, _, _ := strings.Cut(kv, ":"); k != "" {
			have[k] = true
		}
	}
	flags := map[string]bool{}
	for _, f := range strings.Split(b.CustomFlags, ",") {
		flags[f] = f != ""
	}
	labels := map[string]string{}
	if list, err := s.CustomFlags(); err == nil {
		for _, f := range list {
			labels[f.Key] = f.Label
		}
	}
	yes := map[string]bool{"nudity": b.Nudity, "solo_acts": b.SoloActs, "heavy_innuendo": b.HeavyInnuendo, "dark_occult": b.DarkOccult || b.DemonicPresence}
	names := map[string]string{"nudity": "Nudity", "solo_acts": "Solo acts", "heavy_innuendo": "Heavy innuendo", "dark_occult": "Dark occult"}
	for _, k := range strings.Split(u.ContentWarn, ",") {
		switch {
		case names[k] != "":
			if yes[k] {
				out = append(out, names[k])
			}
		case strings.HasPrefix(k, content.CustomPrefix):
			if key := strings.TrimPrefix(k, content.CustomPrefix); flags[key] {
				out = append(out, labels[key])
			}
		case strings.HasPrefix(k, content.GroupPrefix):
			var found []string
			for _, it := range content.ItemsOf(strings.TrimPrefix(k, content.GroupPrefix)) {
				if have[it] {
					found = append(found, content.Label(it))
				}
			}
			if len(found) > 0 {
				out = append(out, content.Label(strings.TrimPrefix(k, content.GroupPrefix))+": "+strings.Join(found, ", "))
			}
		case have[k]:
			out = append(out, content.Label(k))
		}
	}
	return out
}

// deepPlaces says where a Deep Scan found parts at this spice level or higher ("around 42% in (Chapter 12)").
func (s *Store) deepPlaces(bookID int64, from int) string {
	d := s.LatestDeepRead(bookID)
	if d == nil || d.Status != "done" || d.Words <= 0 || d.Notes == "" {
		return ""
	}
	var notes []struct {
		Label string `json:"label"`
		Level int    `json:"level"`
		From  int    `json:"from"`
	}
	if json.Unmarshal([]byte(d.Notes), &notes) != nil {
		return ""
	}
	var at []string
	for _, n := range notes {
		if n.Level >= from && len(at) < 4 {
			place := fmt.Sprintf("around %d%% in", n.From*100/d.Words)
			if n.Label != "" {
				place += " (" + n.Label + ")"
			}
			at = append(at, place)
		}
	}
	return strings.Join(at, "; ")
}
