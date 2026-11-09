package store

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"time"
)

// The link switches itself off after a while (it can be extended), and notes when it was last opened.
const (
	KeyBugLinkExpires = "bug_link_expires"
	KeyBugLinkSeen    = "bug_link_seen"
	BugLinkDays       = 7
)

// KeyBugLink is the secret in the public bug-report link (/bugs/<secret>).
// Empty means the link is off.
const KeyBugLink = "bug_link_token"

// BugLinkToken is the current secret ("" when the link is off).
func (s *Store) BugLinkToken() string { return s.Setting(KeyBugLink) }

// NewBugLinkToken turns the link on with a fresh secret (the old link stops working).
func (s *Store) NewBugLinkToken() (string, error) {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	if err := s.SetSetting(KeyBugLink, tok); err != nil {
		return "", err
	}
	_ = s.SetSetting(KeyBugLinkSeen, "")
	return tok, s.ExtendBugLink()
}

// ExtendBugLink lets the link work for another week from now.
func (s *Store) ExtendBugLink() error {
	return s.SetSetting(KeyBugLinkExpires, time.Now().UTC().Add(BugLinkDays*24*time.Hour).Format(time.RFC3339))
}

// BugLinkExpired reports whether the link has run out (a link with no date, made before this, counts as running).
func (s *Store) BugLinkExpired() bool {
	t, err := time.Parse(time.RFC3339, s.Setting(KeyBugLinkExpires))
	return err == nil && time.Now().After(t)
}

// NoteBugLinkSeen remembers that someone opened the link.
func (s *Store) NoteBugLinkSeen() {
	_ = s.SetSetting(KeyBugLinkSeen, time.Now().UTC().Format(time.RFC3339))
}

// ClearBugLink turns the link off.
func (s *Store) ClearBugLink() error {
	_ = s.SetSetting(KeyBugLinkExpires, "")
	return s.SetSetting(KeyBugLink, "")
}

// BugLinkOK reports whether tok is the current secret.
func (s *Store) BugLinkOK(tok string) bool {
	cur := s.BugLinkToken()
	if cur != "" && s.Setting(KeyBugLinkExpires) == "" {
		_ = s.ExtendBugLink() // a link made before links expired gets its week from now
	}
	return cur != "" && len(tok) == len(cur) && subtle.ConstantTimeCompare([]byte(tok), []byte(cur)) == 1 && !s.BugLinkExpired()
}

// RecentLogs are the newest device and browser logs with their text.
type RecentLog struct {
	DeviceLog
	Body string `db:"body" json:"body"`
}

func (s *Store) RecentLogs(n int) ([]RecentLog, error) {
	out := []RecentLog{}
	err := s.DB.Select(&out, `SELECT id, username, device, version, LENGTH(body) AS size, created_at, body FROM device_logs ORDER BY id DESC LIMIT ?`, n)
	return out, err
}

// DeleteLogs removes the named reports (ids), or all of them when all is true.
func (s *Store) DeleteLogs(ids []int64, all bool) (int64, error) {
	if all {
		res, err := s.DB.Exec(`DELETE FROM device_logs`)
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}
	var n int64
	for _, id := range ids {
		res, err := s.DB.Exec(`DELETE FROM device_logs WHERE id = ?`, id)
		if err != nil {
			return n, err
		}
		c, _ := res.RowsAffected()
		n += c
	}
	return n, nil
}
