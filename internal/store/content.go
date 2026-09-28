package store

// Detailed content items on books (package content) and users' rules to hide
// them.

import (
	"sort"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/content"
)

// Content sources: who said a book contains an item.
const (
	SourceAI     = "ai"
	SourceDeep   = "deep"
	SourceParent = "parent"
)

// contentItemCond is the SQL test for "book has content item key" (one arg).
const contentItemCond = `EXISTS (SELECT 1 FROM book_content bc WHERE bc.book_id = b.id AND bc.key = ?)`

// setBookContent replaces a book's content items (unknown keys are dropped)
// and records that it was checked, plus Deep Scan amounts when given.
func (s *Store) setBookContent(bookID int64, keys []string, source string, amounts map[string]int) error {
	if _, err := s.DB.Exec(`DELETE FROM book_content WHERE book_id = ?`, bookID); err != nil {
		return err
	}
	for _, k := range keys {
		if content.GroupOf(k) == "" {
			continue
		}
		if _, err := s.DB.Exec(`INSERT OR IGNORE INTO book_content (book_id, key, source) VALUES (?, ?, ?)`,
			bookID, k, source); err != nil {
			return err
		}
	}
	amountsSQL, args := "", []any{content.Version}
	if source != SourceParent {
		// A parent's correction keeps the Deep Scan amounts; a new rating replaces them.
		amountsSQL, args = ", content_amounts = ?", append(args, EncodeAmounts(amounts))
	}
	_, err := s.DB.Exec(`UPDATE books SET content_version = ?`+amountsSQL+`, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		append(args, bookID)...)
	return err
}

// SaveContent stores only a book's content items and premise, leaving its
// peppers and flags alone (the content check for Deep Scanned and hand-rated
// books). An empty premise keeps the one the book has.
func (s *Store) SaveContent(bookID int64, keys []string, source, premise string) error {
	if premise = strings.TrimSpace(premise); premise != "" {
		if _, err := s.DB.Exec(`UPDATE books SET premise = ? WHERE id = ?`, premise, bookID); err != nil {
			return err
		}
	}
	return s.setBookContent(bookID, keys, source, nil)
}

// EncodeAmounts writes Deep Scan amounts as "gore:1,violence:3".
func EncodeAmounts(amounts map[string]int) string {
	var parts []string
	for g, n := range amounts {
		if n > content.None && n <= content.ALot && content.ItemsOf(g) != nil {
			parts = append(parts, g+":"+strconv.Itoa(n))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// KeepsRating reports whether a re-rate must leave b's rating alone: a
// parent rated it or a Deep Scan read it. Such books only get their content
// items checked (ContentChecked says whether that's still needed).
func KeepsRating(b *Book) bool {
	return strings.HasPrefix(b.AnalysisModel, "manual:") || strings.HasPrefix(b.AnalysisModel, DeepModelPrefix)
}

// ContentChecked reports whether b was checked for the current content items.
func ContentChecked(b *Book) bool { return b.ContentVersion >= content.Version }

// hiddenContent returns a user's content hide rules.
func (s *Store) hiddenContent(userID int64) ([]string, error) {
	keys := []string{}
	err := s.DB.Select(&keys, `SELECT key FROM user_hidden_content WHERE user_id = ? ORDER BY key`, userID)
	return keys, err
}

// withHiddenContent fills in each user's content hide rules and collections.
func (s *Store) withHiddenContent(us []User) ([]User, error) {
	for i := range us {
		if err := s.userExtras(&us[i]); err != nil {
			return nil, err
		}
	}
	return us, nil
}

// userExtras loads what lives outside the users row: content hide rules and
// the collections a kid may be limited to.
func (s *Store) userExtras(u *User) error {
	var err error
	if u.HiddenContent, err = s.hiddenContent(u.ID); err != nil {
		return err
	}
	u.Collections, err = s.userCollections(u.ID)
	return err
}

// setHiddenContent replaces a user's content hide rules (invalid keys are
// dropped).
func (s *Store) setHiddenContent(userID int64, keys []string) error {
	if _, err := s.DB.Exec(`DELETE FROM user_hidden_content WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, k := range keys {
		if !content.ValidRule(k) {
			continue
		}
		if _, err := s.DB.Exec(`INSERT OR IGNORE INTO user_hidden_content (user_id, key) VALUES (?, ?)`, userID, k); err != nil {
			return err
		}
	}
	return nil
}

// contentRuleConds turns a user's hide rules into SQL conditions a visible
// book must meet. With content rules and "Hide unrated", books not yet
// checked for content items stay hidden too (a parent's age group counts as
// checked), except for a rule on LGBTQ+ alone: every rating has checked that
// since before v1.19.
func contentRuleConds(u *User) (conds []string, args []any) {
	itemKeys, customKeys := content.Expand(u.HiddenContent)
	if len(itemKeys) > 0 {
		conds = append(conds, `NOT EXISTS (SELECT 1 FROM book_content hc WHERE hc.book_id = b.id AND hc.key IN (`+marks(len(itemKeys))+`))`)
		for _, k := range itemKeys {
			args = append(args, k)
		}
		if u.HideUnrated && (len(itemKeys) > 1 || itemKeys[0] != "lgbtq") {
			conds = append(conds, "(b.content_version > 0 OR b.age_level > 0)") // a parent's age group counts as checked
		}
	}
	if len(customKeys) > 0 {
		conds = append(conds, `NOT EXISTS (SELECT 1 FROM book_flags hf JOIN custom_flags hcf ON hcf.id = hf.flag_id
			WHERE hf.book_id = b.id AND hcf.key IN (`+marks(len(customKeys))+`))`)
		for _, k := range customKeys {
			args = append(args, k)
		}
	}
	return conds, args
}

// contentPredicate resolves a Library filter name for a content item
// ("murder") or group ("g:violence").
func contentPredicate(name string) (string, []any, bool) {
	if content.GroupOf(name) != "" {
		return contentItemCond, []any{name}, true
	}
	if g, ok := strings.CutPrefix(name, content.GroupPrefix); ok {
		if keys := content.ItemsOf(g); keys != nil {
			args := make([]any, len(keys))
			for i, k := range keys {
				args[i] = k
			}
			return `EXISTS (SELECT 1 FROM book_content bg WHERE bg.book_id = b.id AND bg.key IN (` + marks(len(keys)) + `))`, args, true
		}
	}
	return "", nil, false
}

// marks is n comma-separated SQL placeholders.
func marks(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }
