package store

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Debug recording: an admin turns it on for a few minutes (never more than an hour) and what happens while it is
// on is kept in activity_log: for a week at most, and never more than 20,000 lines.
const (
	keyActivityUntil = "activity_until" // unix seconds; 0 or past = off
	maxActivityRows  = 20000
	MaxActivityMins  = 60
)

// ActivityEvent is one recorded line.
type ActivityEvent struct {
	ID     int64  `db:"id" json:"id"`
	At     string `db:"at" json:"at"` // UTC, "2006-01-02 15:04:05.000"
	User   string `db:"username" json:"user"`
	Source string `db:"source" json:"source"` // "browser" or "server"
	Kind   string `db:"kind" json:"kind"`
	Detail string `db:"detail" json:"detail"`
}

var (
	activityUntil atomic.Int64
	activityOnce  sync.Once
	activityAdds  atomic.Int64
)

func (s *Store) loadActivity() {
	activityOnce.Do(func() {
		n, _ := strconv.ParseInt(s.Setting(keyActivityUntil), 10, 64)
		activityUntil.Store(n)
	})
}

// ActivityUntil is when recording ends (unix seconds), or 0 when it is off.
func (s *Store) ActivityUntil() int64 {
	s.loadActivity()
	if u := activityUntil.Load(); u > time.Now().Unix() {
		return u
	}
	return 0
}

// ActivityOn says whether recording is on right now (cheap: no database read).
func (s *Store) ActivityOn() bool { return s.ActivityUntil() != 0 }

// StartActivity turns recording on for that many minutes (1 to 60); 0 turns it off. It returns when it ends.
func (s *Store) StartActivity(minutes int) (int64, error) {
	s.loadActivity()
	var until int64
	if minutes > 0 {
		until = time.Now().Add(time.Duration(min(minutes, MaxActivityMins)) * time.Minute).Unix()
	}
	activityUntil.Store(until)
	return until, s.SetSetting(keyActivityUntil, strconv.FormatInt(until, 10))
}

type activityRow struct {
	at                         time.Time
	user, source, kind, detail string
}

// AddActivity notes one line (when t is zero, now). It never waits for the database: lines are kept in memory
// and written in one go every second, so recording cannot slow a request down (NovelCheck has one database
// connection, and every request waits its turn for it).
func (s *Store) AddActivity(t time.Time, user, source, kind, detail string) {
	if t.IsZero() || t.After(time.Now().Add(time.Minute)) {
		t = time.Now()
	}
	detail = strings.TrimSpace(detail)
	if len(detail) > 600 {
		detail = detail[:600] + "…"
	}
	s.actOnce.Do(func() {
		go func() {
			for range time.Tick(time.Second) {
				s.flushActivity()
			}
		}()
	})
	s.actMu.Lock()
	if len(s.actPending) >= 5000 { // the database is far behind: drop the oldest rather than grow
		s.actPending = s.actPending[1:]
	}
	s.actPending = append(s.actPending, activityRow{t, user, source, kind, detail})
	s.actMu.Unlock()
}

// flushActivity writes the waiting lines in one transaction.
func (s *Store) flushActivity() {
	s.actMu.Lock()
	rows := s.actPending
	s.actPending = nil
	s.actMu.Unlock()
	if len(rows) == 0 {
		return
	}
	tx, err := s.DB.Beginx()
	if err != nil {
		return
	}
	for _, r := range rows {
		_, _ = tx.Exec(`INSERT INTO activity_log (at, username, source, kind, detail) VALUES (?, ?, ?, ?, ?)`,
			r.at.UTC().Format("2006-01-02 15:04:05.000"), r.user, r.source, r.kind, r.detail)
	}
	if tx.Commit() != nil {
		return
	}
	if activityAdds.Add(int64(len(rows)))%200 < int64(len(rows)) { // now and then: a week, and the newest 20,000
		_, _ = s.DB.Exec(`DELETE FROM activity_log WHERE at < ? OR id <= (SELECT MAX(id) - ? FROM activity_log)`,
			time.Now().Add(-7*24*time.Hour).UTC().Format("2006-01-02 15:04:05.000"), maxActivityRows)
	}
}

// ActivityLog returns the newest n lines, oldest first.
func (s *Store) ActivityLog(n int) []ActivityEvent {
	s.flushActivity()
	out := []ActivityEvent{}
	_ = s.DB.Select(&out, `SELECT id, at, username, source, kind, detail FROM (SELECT * FROM activity_log ORDER BY at DESC, id DESC LIMIT ?) ORDER BY at, id`, n)
	return out
}

// ClearActivity deletes every recorded line.
func (s *Store) ClearActivity() error {
	s.actMu.Lock()
	s.actPending = nil
	s.actMu.Unlock()
	_, err := s.DB.Exec(`DELETE FROM activity_log`)
	return err
}
