package store

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
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
	return tok, s.SetSetting(KeyBugLink, tok)
}

// ClearBugLink turns the link off.
func (s *Store) ClearBugLink() error { return s.SetSetting(KeyBugLink, "") }

// BugLinkOK reports whether tok is the current secret.
func (s *Store) BugLinkOK(tok string) bool {
	cur := s.BugLinkToken()
	return cur != "" && len(tok) == len(cur) && subtle.ConstantTimeCompare([]byte(tok), []byte(cur)) == 1
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
