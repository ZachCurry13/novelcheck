package store

import "time"

// Reading summaries (api/summaries.go): who wants which notice, what was sent, and what a period of reading adds up to.

// SummaryPrefs is which summaries a person wants.
type SummaryPrefs struct {
	Week  bool `json:"week"`
	Month bool `json:"month"`
	Year  bool `json:"year"`
}

// SummaryPrefsOf is a person's choice (all three on until they change it).
func (s *Store) SummaryPrefsOf(userID int64) SummaryPrefs {
	var r struct {
		Week, Month, Year int
	}
	if err := s.DB.Get(&r, `SELECT week, month, year FROM summary_prefs WHERE user_id = ?`, userID); err != nil {
		return SummaryPrefs{Week: true, Month: true, Year: true}
	}
	return SummaryPrefs{Week: r.Week == 1, Month: r.Month == 1, Year: r.Year == 1}
}

// SetSummaryPrefs keeps a person's choice.
func (s *Store) SetSummaryPrefs(userID int64, p SummaryPrefs) error {
	b := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	_, err := s.DB.Exec(`INSERT INTO summary_prefs (user_id, week, month, year) VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET week = excluded.week, month = excluded.month, year = excluded.year`,
		userID, b(p.Week), b(p.Month), b(p.Year))
	return err
}

// SummarySent says whether the notice for this period (w:2026-11-09, m:2026-10, y:2026) already went to this person.
func (s *Store) SummarySent(userID int64, period string) bool {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM summary_sent WHERE user_id = ? AND period = ?`, userID, period)
	return n > 0
}

// MarkSummarySent notes that the notice for this period was dealt with (sent, or nothing to say).
func (s *Store) MarkSummarySent(userID int64, period string) {
	_, _ = s.DB.Exec(`INSERT OR IGNORE INTO summary_sent (user_id, period, sent_at) VALUES (?, ?, ?)`, userID, period, time.Now().Unix())
}

// SummaryUsers are the people with reading by the hour stored (the notices are for them).
func (s *Store) SummaryUsers() []int64 {
	var ids []int64
	_ = s.DB.Select(&ids, `SELECT DISTINCT user_id FROM reading_hours`)
	return ids
}

// ReadingBetween adds up someone's reading in [from, to): the seconds, the pages, and on how many days (in loc).
func (s *Store) ReadingBetween(userID int64, from, to time.Time, loc *time.Location) (seconds int64, pages int, days int) {
	var rows []struct {
		Hour    int64 `db:"hour"`
		Seconds int64 `db:"seconds"`
		Pages   int   `db:"pages"`
	}
	_ = s.DB.Select(&rows, `SELECT hour, seconds, pages FROM reading_hours WHERE user_id = ? AND hour >= ? AND hour < ?`, userID, from.Unix(), to.Unix())
	seen := map[string]bool{}
	for _, r := range rows {
		seconds += r.Seconds
		pages += r.Pages
		if r.Seconds > 0 {
			seen[time.Unix(r.Hour, 0).In(loc).Format("2006-01-02")] = true
		}
	}
	return seconds, pages, len(seen)
}
