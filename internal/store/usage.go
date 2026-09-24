package store

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
