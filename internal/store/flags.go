package store

import (
	"errors"
	"strconv"
	"strings"
)

// CustomFlag is one of the family's own AI filters, e.g. "Heavy swearing".
type CustomFlag struct {
	ID          int64  `db:"id" json:"id"`
	Key         string `db:"key" json:"key"` // JSON key in the AI's answer
	Label       string `db:"label" json:"label"`
	Description string `db:"description" json:"description"` // what the AI looks for
	CreatedAt   string `db:"created_at" json:"created_at"`
}

// KeyFlagsVersion goes up whenever a filter is added or its instructions
// change, so books rated before then are offered for re-rating.
const KeyFlagsVersion = "custom_flags_version"

// MaxCustomFlags keeps the AI prompt (and its cost) small.
const MaxCustomFlags = 12

var ErrTooManyFlags = errors.New("that's the most custom filters NovelCheck allows (12)")

func (s *Store) CustomFlags() ([]CustomFlag, error) {
	out := []CustomFlag{}
	err := s.DB.Select(&out, `SELECT id, key, label, description, created_at FROM custom_flags ORDER BY id`)
	return out, err
}

// FlagsVersion is the current custom-filters version (0 = never had any).
func (s *Store) FlagsVersion() int { return s.SettingInt(KeyFlagsVersion) }

func (s *Store) bumpFlagsVersion() error {
	return s.SetSetting(KeyFlagsVersion, strconv.Itoa(s.FlagsVersion()+1))
}

// flagKey turns "Heavy swearing / language" into "heavy_swearing_language".
func flagKey(label string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(label) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "_"):
			b.WriteByte('_')
		}
	}
	k := strings.Trim(b.String(), "_")
	if len(k) > 24 {
		k = strings.TrimRight(k[:24], "_")
	}
	if k == "" {
		k = "filter"
	}
	return k
}

// AddCustomFlag creates a filter with a unique key derived from its label.
func (s *Store) AddCustomFlag(label, description string) (CustomFlag, error) {
	var n int
	if err := s.DB.Get(&n, `SELECT COUNT(*) FROM custom_flags`); err != nil {
		return CustomFlag{}, err
	}
	if n >= MaxCustomFlags {
		return CustomFlag{}, ErrTooManyFlags
	}
	base := flagKey(label)
	key := base
	for i := 2; ; i++ {
		var taken int
		_ = s.DB.Get(&taken, `SELECT COUNT(*) FROM custom_flags WHERE key = ?`, key)
		if taken == 0 {
			break
		}
		key = base + "_" + strconv.Itoa(i)
	}
	res, err := s.DB.Exec(`INSERT INTO custom_flags (key, label, description) VALUES (?, ?, ?)`, key, label, description)
	if err != nil {
		return CustomFlag{}, err
	}
	id, _ := res.LastInsertId()
	if err := s.bumpFlagsVersion(); err != nil {
		return CustomFlag{}, err
	}
	var f CustomFlag
	err = s.DB.Get(&f, `SELECT id, key, label, description, created_at FROM custom_flags WHERE id = ?`, id)
	return f, err
}

// UpdateCustomFlag renames a filter or changes its instructions. New
// instructions mean books should be checked again.
func (s *Store) UpdateCustomFlag(id int64, label, description string) error {
	var old string
	if err := s.DB.Get(&old, `SELECT description FROM custom_flags WHERE id = ?`, id); err != nil {
		return ErrNotFound
	}
	if _, err := s.DB.Exec(`UPDATE custom_flags SET label = ?, description = ? WHERE id = ?`, label, description, id); err != nil {
		return err
	}
	if old != description {
		return s.bumpFlagsVersion()
	}
	return nil
}

// DeleteCustomFlag removes a filter and its marks on books.
func (s *Store) DeleteCustomFlag(id int64) error {
	res, err := s.DB.Exec(`DELETE FROM custom_flags WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// setBookFlags replaces a book's custom-filter marks with keys (unknown
// keys are ignored).
func (s *Store) setBookFlags(bookID int64, keys []string) error {
	if _, err := s.DB.Exec(`DELETE FROM book_flags WHERE book_id = ?`, bookID); err != nil {
		return err
	}
	for _, k := range keys {
		if _, err := s.DB.Exec(`INSERT OR IGNORE INTO book_flags (book_id, flag_id)
			SELECT ?, id FROM custom_flags WHERE key = ?`, bookID, k); err != nil {
			return err
		}
	}
	return nil
}

// customFlagCond is the SQL test for "book has custom filter key" (one arg).
const customFlagCond = `EXISTS (SELECT 1 FROM book_flags bf JOIN custom_flags cf ON cf.id = bf.flag_id
	WHERE bf.book_id = b.id AND cf.key = ?)`

// flagPredicate resolves a filter name: a built-in flag, or "flag:<key>".
func flagPredicate(name string) (string, []any, bool) {
	if p, ok := flagColumns[name]; ok {
		return p, nil, true
	}
	if k, ok := strings.CutPrefix(name, "flag:"); ok && k != "" {
		return customFlagCond, []any{k}, true
	}
	return "", nil, false
}
