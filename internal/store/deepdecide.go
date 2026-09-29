package store

// An admin's decision on a held Deep Scan beyond accept or keep: a level of
// their own choosing (the right answer can sit between the old rating and
// the scan's), and a note for parents on the book saying what the scan found
// and what was decided.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// DeepReadBook is the book a Deep Scan is for.
func (s *Store) DeepReadBook(id int64) (int64, error) {
	var b int64
	if err := s.DB.Get(&b, `SELECT book_id FROM deep_reads WHERE id = ?`, id); err != nil {
		return 0, ErrNotFound
	}
	return b, nil
}

// SetHeldLevel saves a held scan's findings with the level an admin chose.
// It reports false when the scan isn't waiting any more.
func (s *Store) SetHeldLevel(id int64, level int, by string) (bool, error) {
	if level < 0 || level > 5 {
		return false, errors.New("pick a level from 0 to 5")
	}
	var row struct {
		BookID   int64  `db:"book_id"`
		Proposal string `db:"proposal"`
	}
	if err := s.DB.Get(&row, `SELECT book_id, proposal FROM deep_reads WHERE id = ? AND held = 1`, id); err != nil {
		return false, nil
	}
	var a Analysis
	if err := json.Unmarshal([]byte(row.Proposal), &a); err != nil || a.SpiceLevel == nil {
		return false, errors.New("this Deep Scan's result can't be read; scan the book again")
	}
	if *a.SpiceLevel != level {
		a.SpiceLevel = &level
		a.SpiceReason = "Set by a parent after a Deep Scan"
		if level < 3 { // the sexual-content flags only stand with a sex scene
			a.Nudity, a.SoloActs, a.HeavyInnuendo = false, false, false
		}
	}
	if err := s.SaveAnalysis(row.BookID, a); err != nil {
		return false, err
	}
	_, err := s.DB.Exec(`UPDATE deep_reads SET held = 0, new_level = ?, approved_by = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, level, by, id)
	return err == nil, err
}

// deepNote is one flagged part as the scan's notes keep it.
type deepNote struct {
	Label string `json:"label"`
	Level int    `json:"level"`
	Note  string `json:"note"`
	Scene string `json:"scene"`
}

// NoteDeepDecision adds a parents-only note to the book: the scenes the
// scan found (level 3 and up) and the level the admin decided on.
func (s *Store) NoteDeepDecision(id, userID int64, by string, level int) error {
	var row struct {
		BookID int64  `db:"book_id"`
		Notes  string `db:"notes"`
	}
	if err := s.DB.Get(&row, `SELECT book_id, notes FROM deep_reads WHERE id = ?`, id); err != nil {
		return err
	}
	var notes []deepNote
	_ = json.Unmarshal([]byte(row.Notes), &notes)
	var lines []string
	for _, n := range notes {
		if n.Level < 3 {
			continue
		}
		what := n.Scene
		if what == "" {
			what = n.Note
		}
		lines = append(lines, fmt.Sprintf("• %s: %s", n.Label, what))
	}
	if len(lines) == 0 {
		return nil
	}
	head := "🧬 Deep Scan found a scene with sexual content"
	if len(lines) > 1 {
		head = fmt.Sprintf("🧬 Deep Scan found %d scenes with sexual content", len(lines))
	}
	body := fmt.Sprintf("%s:\n%s\n%s decided on Level %d.", head, strings.Join(lines, "\n"), by, level)
	if len(body) > MaxNoteLen { // bytes, like the note check: cut at a character boundary
		cut := MaxNoteLen - len("…")
		for cut > 0 && !utf8.RuneStart(body[cut]) {
			cut--
		}
		body = body[:cut] + "…"
	}
	_, err := s.AddNote(row.BookID, userID, body, "parents")
	return err
}

// HeldDeepReadIDs lists the scans waiting for an admin.
func (s *Store) HeldDeepReadIDs() []int64 {
	ids := []int64{}
	_ = s.DB.Select(&ids, `SELECT id FROM deep_reads WHERE held = 1 ORDER BY id`)
	return ids
}
