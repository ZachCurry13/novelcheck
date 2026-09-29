package store

import (
	"database/sql"
	"errors"
)

// Family devices: a shared tablet or computer where everyone picks their
// profile ("Who's reading?") instead of typing a password. Kids tap their
// name; parents give their PIN (or password). Anyone can have a PIN.

// FamilyDevice is one shared device.
type FamilyDevice struct {
	ID        int64  `db:"id" json:"id"`
	Name      string `db:"name" json:"name"`
	CreatedBy string `db:"created_by" json:"created_by"`
	CreatedAt string `db:"created_at" json:"created_at"`
	LastUsed  string `db:"last_used" json:"last_used"`
}

// AddFamilyDevice records a device by the digest of its private key.
func (s *Store) AddFamilyDevice(tokenHash, name, by string) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO family_devices (token_hash, name, created_by) VALUES (?, ?, ?)`, tokenHash, name, by)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FamilyDeviceByToken finds a device by its key's digest and notes its use.
func (s *Store) FamilyDeviceByToken(tokenHash string) (*FamilyDevice, error) {
	var d FamilyDevice
	err := s.DB.Get(&d, `SELECT id, name, created_by, created_at, last_used FROM family_devices WHERE token_hash = ?`, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err == nil {
		_, _ = s.DB.Exec(`UPDATE family_devices SET last_used = CURRENT_TIMESTAMP WHERE id = ? AND last_used < datetime('now', '-1 hour')`, d.ID)
	}
	return &d, err
}

// FamilyDevices lists the shared devices, last used first.
func (s *Store) FamilyDevices() ([]FamilyDevice, error) {
	out := []FamilyDevice{}
	err := s.DB.Select(&out, `SELECT id, name, created_by, created_at, last_used FROM family_devices ORDER BY last_used DESC`)
	return out, err
}

// RemoveFamilyDevice stops a device being a family device.
func (s *Store) RemoveFamilyDevice(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM family_devices WHERE id = ?`, id)
	return err
}

// RemoveFamilyDeviceByToken is RemoveFamilyDevice for the device in hand.
func (s *Store) RemoveFamilyDeviceByToken(tokenHash string) error {
	_, err := s.DB.Exec(`DELETE FROM family_devices WHERE token_hash = ?`, tokenHash)
	return err
}

// PinHash is someone's PIN digest ("" = no PIN).
func (s *Store) PinHash(userID int64) string {
	var h string
	_ = s.DB.Get(&h, `SELECT pin_hash FROM users WHERE id = ?`, userID)
	return h
}

// SetPinHash sets or ("" ) clears someone's PIN.
func (s *Store) SetPinHash(userID int64, hash string) error {
	_, err := s.DB.Exec(`UPDATE users SET pin_hash = ? WHERE id = ?`, hash, userID)
	return err
}

// Profile is a person on the "Who's reading?" screen.
type Profile struct {
	ID       int64  `db:"id" json:"id"`
	Username string `db:"username" json:"username"`
	Role     string `db:"role" json:"role"`
	AgeLevel int    `db:"age_level" json:"age_level"`
	HasPin   bool   `db:"has_pin" json:"has_pin"`
}

// Profiles lists everyone: parents first, then kids, by name.
func (s *Store) Profiles() ([]Profile, error) {
	out := []Profile{}
	err := s.DB.Select(&out, `SELECT id, username, role, age_level, pin_hash != '' AS has_pin FROM users
		ORDER BY CASE role WHEN 'restricted' THEN 1 ELSE 0 END, username COLLATE NOCASE`)
	return out, err
}
