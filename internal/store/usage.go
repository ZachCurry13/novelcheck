package store

import (
	"fmt"
	"time"
)

// UsageStats summarises LLM token consumption for the cost dashboard.
type UsageStats struct {
	LastHourTokens  int     `json:"last_hour_tokens"`
	TodayTokens     int     `json:"today_tokens"`
	TotalPrompt     int     `json:"total_prompt_tokens"`
	TotalCompletion int     `json:"total_completion_tokens"`
	TotalCalls      int     `json:"total_calls"`
	AvgPerBook      float64 `json:"avg_tokens_per_call"`
}

func (s *Store) RecordUsage(bookID int64, model string, prompt, completion int) error {
	_, err := s.DB.Exec(`INSERT INTO token_usage (book_id, model, prompt_tokens, completion_tokens)
		VALUES (?, ?, ?, ?)`, bookID, model, prompt, completion)
	return err
}

// RecordUsageCost records a call priced with that AI's own rates, so a paid
// backup and a free local model are both counted correctly.
func (s *Store) RecordUsageCost(bookID int64, model string, prompt, completion int, ai AIConfig) error {
	cost := float64(prompt)*ai.PriceIn/1e6 + float64(completion)*ai.PriceOut/1e6
	_, err := s.DB.Exec(`INSERT INTO token_usage (book_id, model, prompt_tokens, completion_tokens, cost)
		VALUES (?, ?, ?, ?, ?)`, bookID, model, prompt, completion, cost)
	return err
}

// SpentUSD totals AI spending. Calls recorded before per-call costs existed
// are priced with the main AI's current rates.
func (s *Store) SpentUSD() float64 {
	var v float64
	_ = s.DB.Get(&v, `SELECT COALESCE(SUM(COALESCE(cost, prompt_tokens * ? / 1e6 + completion_tokens * ? / 1e6)), 0)
		FROM token_usage`, s.SettingFloat(KeyPriceInputPerM), s.SettingFloat(KeyPriceOutputPerM))
	return v
}

// TokensSince returns prompt+completion tokens used in the trailing window,
// expressed as an SQLite datetime modifier such as "-1 hour".
func (s *Store) TokensSince(modifier string) (int, error) {
	var n int
	err := s.DB.Get(&n, `SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0)
		FROM token_usage WHERE at >= datetime('now', ?)`, modifier)
	return n, err
}

// SecondsUntilWindowFrees returns seconds until the oldest in-window usage row
// ages out, letting the rate limiter sleep precisely.
func (s *Store) SecondsUntilWindowFrees() (int, error) {
	var secs int
	err := s.DB.Get(&secs, `SELECT COALESCE(CAST(3600 - (strftime('%s','now') - strftime('%s', MIN(at))) AS INTEGER), 0)
		FROM token_usage WHERE at >= datetime('now', '-1 hour')`)
	return secs, err
}

func (s *Store) Usage() (UsageStats, error) {
	var u UsageStats
	var err error
	if u.LastHourTokens, err = s.TokensSince("-1 hour"); err != nil {
		return u, err
	}
	if err = s.DB.Get(&u.TodayTokens, `SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0)
		FROM token_usage WHERE at >= date('now')`); err != nil {
		return u, err
	}
	row := struct {
		P int `db:"p"`
		C int `db:"c"`
		N int `db:"n"`
	}{}
	if err = s.DB.Get(&row, `SELECT COALESCE(SUM(prompt_tokens), 0) AS p,
		COALESCE(SUM(completion_tokens), 0) AS c, COUNT(*) AS n FROM token_usage`); err != nil {
		return u, err
	}
	u.TotalPrompt, u.TotalCompletion, u.TotalCalls = row.P, row.C, row.N
	if row.N > 0 {
		u.AvgPerBook = float64(row.P+row.C) / float64(row.N)
	}
	return u, nil
}

// DayTokens is one day's AI token use.
type DayTokens struct {
	Day    string `db:"day" json:"day"`
	Tokens int    `db:"tokens" json:"tokens"`
	Calls  int    `db:"calls" json:"calls"`
}

// DailyTokens returns token use per day for the last n days (UTC), oldest
// first, with zero rows for quiet days.
func (s *Store) DailyTokens(n int) ([]DayTokens, error) {
	var rows []DayTokens
	if err := s.DB.Select(&rows, `SELECT date(at) AS day, SUM(prompt_tokens + completion_tokens) AS tokens,
		COUNT(*) AS calls FROM token_usage WHERE at >= date('now', ?) GROUP BY day`,
		fmt.Sprintf("-%d days", n-1)); err != nil {
		return nil, err
	}
	byDay := map[string]DayTokens{}
	for _, r := range rows {
		byDay[r.Day] = r
	}
	out := make([]DayTokens, 0, n)
	today := time.Now().UTC()
	for i := n - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i).Format("2006-01-02")
		r, ok := byDay[d]
		if !ok {
			r = DayTokens{Day: d}
		}
		out = append(out, r)
	}
	return out, nil
}
