package store

import (
	"errors"
	"sort"
)

// A person's own level for a book ("I disagree"): the truth the AI is scored against.

// Feedback is one person's level for one book next to what the AI said when they gave it.
type Feedback struct {
	ID       int64  `db:"id" json:"id"`
	BookID   int64  `db:"book_id" json:"book_id"`
	Title    string `db:"title" json:"title"`
	Author   string `db:"author" json:"author"`
	User     string `db:"username" json:"user"`
	Level    int    `db:"level" json:"level"`
	AILevel  *int   `db:"ai_level" json:"ai_level"`
	Model    string `db:"model" json:"model"`
	Note     string `db:"note" json:"note"`
	Created  string `db:"created_at" json:"created_at"`
	NowLevel *int   `db:"now_level" json:"now_level"` // the book's level now (it may have been re-rated since)
}

// SaveFeedback records (or replaces) a person's level for a book, with the AI's level and model at that moment.
func (s *Store) SaveFeedback(userID, bookID int64, level int, note string) error {
	if level < 0 || level > MaxSpiceLevel {
		return errors.New("the level goes from 0 to 5")
	}
	if len(note) > 500 {
		note = note[:500]
	}
	_, err := s.DB.Exec(`INSERT INTO rating_feedback (book_id, user_id, level, ai_level, model, rules_version, note)
		SELECT id, ?, ?, spice_level, COALESCE(analysis_model, ''), COALESCE(rules_version, 0), ? FROM books WHERE id = ?
		ON CONFLICT(book_id, user_id) DO UPDATE SET level = excluded.level, ai_level = excluded.ai_level, model = excluded.model,
			rules_version = excluded.rules_version, note = excluded.note, created_at = CURRENT_TIMESTAMP`, userID, level, note, bookID)
	return err
}

// RecentFeedback lists the newest disagreements.
func (s *Store) RecentFeedback(n int) []Feedback {
	out := []Feedback{}
	_ = s.DB.Select(&out, `SELECT f.id, f.book_id, b.title, b.author, u.username, f.level, f.ai_level, f.model, f.note, f.created_at, b.spice_level AS now_level
		FROM rating_feedback f JOIN books b ON b.id = f.book_id JOIN users u ON u.id = f.user_id ORDER BY f.created_at DESC, f.id DESC LIMIT ?`, n)
	return out
}

// AccuracyRow is how one model did against people's own levels.
type AccuracyRow struct {
	Model   string  `json:"model"`
	N       int     `json:"n"`
	Exact   float64 `json:"exact"`   // share the AI got exactly right
	Within1 float64 `json:"within1"` // share within one level
	AvgOff  float64 `json:"avg_off"` // average distance, in levels
	Higher  int     `json:"higher"`  // times the AI said more spice than the person
	Lower   int     `json:"lower"`   // times it said less
}

// Accuracy scores every model that has people's levels to be compared with ("" = unknown), best first, with an
// "All" row first. Only books the AI had rated at the time count.
func (s *Store) Accuracy() []AccuracyRow {
	var rows []struct {
		Model string `db:"model"`
		Level int    `db:"level"`
		AI    int    `db:"ai_level"`
	}
	_ = s.DB.Select(&rows, `SELECT model, level, ai_level FROM rating_feedback WHERE ai_level IS NOT NULL`)
	by := map[string]*AccuracyRow{}
	add := func(key string, level, ai int) {
		r := by[key]
		if r == nil {
			r = &AccuracyRow{Model: key}
			by[key] = r
		}
		d := ai - level
		r.N++
		if d == 0 {
			r.Exact++
		}
		if d >= -1 && d <= 1 {
			r.Within1++
		}
		if d < 0 {
			d = -d
		}
		r.AvgOff += float64(d)
		if ai > level {
			r.Higher++
		} else if ai < level {
			r.Lower++
		}
	}
	for _, x := range rows {
		add("All", x.Level, x.AI)
		m := x.Model
		if m == "" {
			m = "(model not recorded)"
		}
		add(m, x.Level, x.AI)
	}
	var out []AccuracyRow
	for _, r := range by {
		n := float64(r.N)
		r.Exact, r.Within1, r.AvgOff = r.Exact/n, r.Within1/n, r.AvgOff/n
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Model == "All") != (out[j].Model == "All") {
			return out[i].Model == "All"
		}
		return out[i].Exact > out[j].Exact
	})
	return out
}

// SetSpiceByHand gives a book the level a person decided (an admin using someone's answer): the other findings stay, and
// the book counts as rated by hand, so the AI doesn't rate it again.
func (s *Store) SetSpiceByHand(bookID int64, level int, by string) error {
	if !ValidSpice(level) {
		return errors.New("the level goes from 0 to 5")
	}
	_, err := s.DB.Exec(`UPDATE books SET status = 'analyzed', spice_level = ?, classification = ?, confidence = 'high', analysis_model = ?,
		spice_reason = ?, analyzed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		level, ClassificationForSpice(level), "manual: "+by, "Set by "+by, bookID)
	return err
}

// UnresolvedFeedback counts the books where a person's level still differs from the book's level now.
func (s *Store) UnresolvedFeedback() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(DISTINCT f.book_id) FROM rating_feedback f JOIN books b ON b.id = f.book_id WHERE b.spice_level IS NULL OR b.spice_level != f.level`)
	return n
}
