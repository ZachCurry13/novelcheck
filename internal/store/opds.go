package store

import (
	"crypto/rand"
	"encoding/hex"
)

func newOPDSToken() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// OPDSToken is the person's KOReader feed token, made on first use.
func (s *Store) OPDSToken(userID int64) (string, error) {
	var tok string
	if err := s.DB.Get(&tok, `SELECT token FROM opds_tokens WHERE user_id = ?`, userID); err == nil {
		return tok, nil
	}
	tok = newOPDSToken()
	if _, err := s.DB.Exec(`INSERT INTO opds_tokens (user_id, token) VALUES (?, ?)
		ON CONFLICT(user_id) DO NOTHING`, userID, tok); err != nil {
		return "", err
	}
	err := s.DB.Get(&tok, `SELECT token FROM opds_tokens WHERE user_id = ?`, userID)
	return tok, err
}

// ResetOPDSToken replaces the token, so the old feed address stops working.
func (s *Store) ResetOPDSToken(userID int64) (string, error) {
	tok := newOPDSToken()
	_, err := s.DB.Exec(`INSERT INTO opds_tokens (user_id, token) VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET token = excluded.token, created_at = CURRENT_TIMESTAMP`, userID, tok)
	return tok, err
}

// UserByOPDSToken finds whose feed a token opens (ErrNotFound if none).
func (s *Store) UserByOPDSToken(token string) (*User, error) {
	var id int64
	if len(token) != 40 || s.DB.Get(&id, `SELECT user_id FROM opds_tokens WHERE token = ?`, token) != nil {
		return nil, ErrNotFound
	}
	return s.UserByID(id)
}
