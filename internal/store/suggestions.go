package store

// Suggested Reads: the books a suggestion can come from, what each person
// likes (their Up Next, reading, finished, and 👍/👎 votes), and the AI's
// latest picks, which are made at most once a day per person.

import (
	"errors"
	"strings"
	"time"
)

// KeySuggestMode is the admin's choice of how suggestions are made. The
// values are what the settings list shows.
const (
	KeySuggestMode   = "suggest_mode"
	SuggestAIOutside = "AI picks + books you don't own"
	SuggestAI        = "AI picks from your library"
	SuggestFree      = "Free matching only (no AI)"
)

func init() { Defaults[KeySuggestMode] = SuggestAIOutside }

// ValidSuggestMode reports whether v is one of the three modes.
func ValidSuggestMode(v string) bool {
	return v == SuggestAIOutside || v == SuggestAI || v == SuggestFree
}

// SuggestBook is a book as the suggestion matcher sees it.
type SuggestBook struct {
	ID          int64   `db:"id"` // 0 for a 👍/👎 on a book outside the library
	Title       string  `db:"title"`
	Author      string  `db:"author"`
	Series      string  `db:"series"`
	SeriesIndex float64 `db:"series_index"`
	Description string  `db:"description"`
	Spice       int     `db:"spice"` // -1 = not rated
}

// SuggestSignal is a book that says what someone likes (Weight > 0) or
// doesn't (Weight < 0).
type SuggestSignal struct {
	SuggestBook
	Weight float64 `db:"weight"`
	Status string  `db:"status"` // queued, reading, finished, up, down, read (a 👎 "already read it")
	Reason string  `db:"reason"` // why a 👎 (see DownReasons)
}

// DownReasons are the optional answers to "Why not?" after a 👎.
var DownReasons = map[string]bool{"read": true, "story": true, "author": true, "series": true, "spicy": true}

const suggestCols = `b.id, b.title, b.author, b.series, b.series_index,
	CASE WHEN b.description != '' THEN b.description ELSE b.blurb END AS description,
	COALESCE(` + effectiveSpice + `, -1) AS spice`

// SuggestPool returns the owned books u may see that aren't in u's Up Next
// (in any state) and that u hasn't voted on.
func (s *Store) SuggestPool(u *User) ([]SuggestBook, error) {
	where, args := visibilityClause(u)
	out := []SuggestBook{}
	err := s.DB.Select(&out, `SELECT `+suggestCols+` FROM books b
		WHERE EXISTS (SELECT 1 FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
			WHERE cb.book_id = b.id AND c.name != ?)
		AND NOT EXISTS (SELECT 1 FROM queue_items q WHERE q.book_id = b.id AND q.user_id = ?)
		AND NOT EXISTS (SELECT 1 FROM suggestion_votes v WHERE v.user_id = ? AND v.norm_key = b.norm_key)`+where,
		append([]any{LookedUpCatalog, u.ID, u.ID}, args...)...)
	return out, err
}

// SuggestSignals returns what someone has in Up Next (queued 2, reading or
// finished 3) and their votes: 👍 4, and 👎 -1 with its reason, which package
// suggest turns into the right lesson (a 👎 "already read it" counts as
// reading history instead, weight 1.5).
func (s *Store) SuggestSignals(userID int64) ([]SuggestSignal, error) {
	out := []SuggestSignal{}
	err := s.DB.Select(&out, `SELECT `+suggestCols+`, CASE q.status WHEN 'queued' THEN 2 ELSE 3 END AS weight, q.status, '' AS reason
		FROM queue_items q JOIN books b ON b.id = q.book_id WHERE q.user_id = ?
		UNION ALL
		SELECT COALESCE(b.id, 0), v.title, v.author, COALESCE(b.series, ''), COALESCE(b.series_index, 0),
			COALESCE(CASE WHEN b.description != '' THEN b.description ELSE b.blurb END, ''),
			CASE WHEN b.id IS NULL THEN -1 ELSE COALESCE(`+effectiveSpice+`, -1) END,
			CASE WHEN v.vote = 1 THEN 4 WHEN v.reason = 'read' THEN 1.5 ELSE -1 END,
			CASE WHEN v.vote = 1 THEN 'up' WHEN v.reason = 'read' THEN 'read' ELSE 'down' END, v.reason
		FROM suggestion_votes v LEFT JOIN books b ON b.norm_key = v.norm_key WHERE v.user_id = ?`, userID, userID)
	return out, err
}

// FamilyReads counts, per book, how many other people are reading or finished it.
func (s *Store) FamilyReads(userID int64) map[int64]int {
	var rows []struct {
		Book int64 `db:"book_id"`
		N    int   `db:"n"`
	}
	_ = s.DB.Select(&rows, `SELECT book_id, COUNT(*) AS n FROM queue_items
		WHERE user_id != ? AND status IN ('reading', 'finished') GROUP BY book_id`, userID)
	out := make(map[int64]int, len(rows))
	for _, r := range rows {
		out[r.Book] = r.N
	}
	return out
}

// VoteSuggestion records a 👍 (1) or 👎 (-1) on a book, with the 👎's reason
// if given ("" or one of DownReasons); 0 takes the vote back.
func (s *Store) VoteSuggestion(userID int64, title, author string, vote int, reason string) error {
	title, author = strings.TrimSpace(title), strings.TrimSpace(author)
	if title == "" {
		return errors.New("title required")
	}
	key := NormKey(title, author)
	if vote == 0 {
		_, err := s.DB.Exec(`DELETE FROM suggestion_votes WHERE user_id = ? AND norm_key = ?`, userID, key)
		return err
	}
	if vote != 1 && vote != -1 || (reason != "" && (vote != -1 || !DownReasons[reason])) {
		return errors.New("vote must be 1, -1 or 0, and a reason only goes with a 👎")
	}
	_, err := s.DB.Exec(`INSERT INTO suggestion_votes (user_id, norm_key, title, author, vote, reason) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id, norm_key) DO UPDATE SET vote = excluded.vote, reason = excluded.reason, created_at = CURRENT_TIMESTAMP`,
		userID, key, title, author, vote, reason)
	return err
}

// Voted returns someone's votes by NormKey.
func (s *Store) Voted(userID int64) map[string]int {
	var rows []struct {
		Key  string `db:"norm_key"`
		Vote int    `db:"vote"`
	}
	_ = s.DB.Select(&rows, `SELECT norm_key, vote FROM suggestion_votes WHERE user_id = ?`, userID)
	out := make(map[string]int, len(rows))
	for _, r := range rows {
		out[r.Key] = r.Vote
	}
	return out
}

// ClearSuggestionVotes forgets all of someone's 👍/👎.
func (s *Store) ClearSuggestionVotes(userID int64) error {
	_, err := s.DB.Exec(`DELETE FROM suggestion_votes WHERE user_id = ?`, userID)
	return err
}

// SuggestionSet is the AI's latest suggestions for one person.
type SuggestionSet struct {
	MadeAt time.Time
	Mode   string
	Data   string // JSON, owned by package suggest
}

// SuggestionSetFor returns someone's latest AI suggestions (nil if none).
func (s *Store) SuggestionSetFor(userID int64) *SuggestionSet {
	var row struct {
		MadeAt string `db:"made_at"`
		Mode   string `db:"mode"`
		Data   string `db:"data"`
	}
	if s.DB.Get(&row, `SELECT made_at, mode, data FROM suggestion_sets WHERE user_id = ?`, userID) != nil {
		return nil
	}
	at, _ := time.Parse(time.DateTime, row.MadeAt)
	return &SuggestionSet{MadeAt: at, Mode: row.Mode, Data: row.Data}
}

// SaveSuggestionSet stores someone's new AI suggestions.
func (s *Store) SaveSuggestionSet(userID int64, mode, data string) error {
	_, err := s.DB.Exec(`INSERT INTO suggestion_sets (user_id, made_at, mode, data) VALUES (?, ?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET made_at = excluded.made_at, mode = excluded.mode, data = excluded.data`,
		userID, time.Now().UTC().Format(time.DateTime), mode, data)
	return err
}
