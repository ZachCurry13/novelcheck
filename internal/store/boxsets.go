package store

// Box sets: a book whose title says it holds several ("Box Set", "Books
// 1-3", "Trilogy", "Omnibus", "Complete Series"). NovelCheck lists them for
// a parent, who checks what's inside (from the title's numbers and the
// series, or the AI's answer) and splits the box set into its books, or
// says it isn't one. The books of a split box set use its files and count
// as owned wherever it is; the box set itself leaves the Library lists.

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

// BoxEntry is one book inside a box set.
type BoxEntry struct {
	Title  string  `db:"title" json:"title"`
	Number float64 `db:"number" json:"number"`
}

// BoxSet is a box set waiting for a parent, or split.
type BoxSet struct {
	BookID   int64      `db:"book_id" json:"book_id"`
	Title    string     `db:"title" json:"title"`
	Author   string     `db:"author" json:"author"`
	Series   string     `db:"series" json:"series"`
	State    string     `db:"state" json:"state"`
	Raw      string     `db:"proposal" json:"-"`
	Proposal []BoxEntry `json:"proposal"`
	Members  []BoxEntry `json:"members"` // once split
}

var boxTitle = regexp.MustCompile(`(?i)\b(box(?:ed)?[\s-]*set|boxset|omnibus|trilogy|quartet|quintet|complete\s+(?:series|saga|collection|trilogy)|books?\s+\d+\s*(?:[-–—&]|to|through)\s*\d+|\d+[\s-]*books?\s+(?:set|bundle|collection|in\s+1)|\d+\s*in\s*1|bundle)\b`)
var bookRange = regexp.MustCompile(`(?i)books?\s+(\d+)\s*(?:[-–—]|to|through)\s*(\d+)`)

// splitBoxCond is a box set that was split into its books.
const splitBoxCond = `EXISTS (SELECT 1 FROM box_sets hx WHERE hx.book_id = b.id AND hx.state = 'split')`

// LooksLikeBoxSet reports whether a title says it holds several books.
func LooksLikeBoxSet(title string) bool { return boxTitle.MatchString(title) }

// FindBoxSets notes the family's books whose titles look like box sets and
// that no parent has decided on yet. It says how many are waiting.
func (s *Store) FindBoxSets() (int, error) {
	var books []struct {
		ID    int64  `db:"id"`
		Title string `db:"title"`
		Fix   string `db:"title_fix"`
	}
	if err := s.DB.Select(&books, `SELECT b.id, b.title, b.title_fix FROM books b
		WHERE `+ownedCond+` AND NOT EXISTS (SELECT 1 FROM box_sets x WHERE x.book_id = b.id)
		AND NOT EXISTS (SELECT 1 FROM box_members m WHERE m.book_id = b.id)`); err != nil {
		return 0, err
	}
	for _, b := range books {
		if LooksLikeBoxSet(b.Title) || LooksLikeBoxSet(b.Fix) {
			if _, err := s.DB.Exec(`INSERT OR IGNORE INTO box_sets (book_id, proposal) VALUES (?, ?)`,
				b.ID, s.rangeProposal(b.ID, b.Title+" "+b.Fix)); err != nil {
				return 0, err
			}
		}
	}
	var n int
	err := s.DB.Get(&n, `SELECT COUNT(*) FROM box_sets WHERE state = 'found'`)
	return n, err
}

// rangeProposal is what a title like "Series Books 1-3" says is inside:
// the series' books the family has under those numbers (JSON).
func (s *Store) rangeProposal(bookID int64, title string) string {
	m := bookRange.FindStringSubmatch(title)
	var series string
	_ = s.DB.Get(&series, `SELECT series FROM books WHERE id = ?`, bookID)
	if m == nil || series == "" {
		return "[]"
	}
	out := []BoxEntry{}
	_ = s.DB.Select(&out, `SELECT b.title, b.series_index AS number FROM books b WHERE LOWER(b.series) = LOWER(?)
		AND b.id != ? AND b.series_index BETWEEN ? AND ? ORDER BY b.series_index`, series, bookID, m[1], m[2])
	raw, _ := json.Marshal(out)
	return string(raw)
}

// BoxSets lists the box sets waiting for a parent, then the split ones.
func (s *Store) BoxSets() ([]BoxSet, error) {
	out := []BoxSet{}
	if err := s.DB.Select(&out, `SELECT x.book_id, b.title, b.author, b.series, x.state, x.proposal
		FROM box_sets x JOIN books b ON b.id = x.book_id WHERE x.state IN ('found', 'split')
		ORDER BY x.state = 'split', sort_title(b.title)`); err != nil {
		return nil, err
	}
	for i := range out {
		_ = json.Unmarshal([]byte(out[i].Raw), &out[i].Proposal)
		if out[i].Proposal == nil {
			out[i].Proposal = []BoxEntry{}
		}
		out[i].Members = s.boxMembers(out[i].BookID)
	}
	return out, nil
}

func (s *Store) boxMembers(boxID int64) []BoxEntry {
	out := []BoxEntry{}
	_ = s.DB.Select(&out, `SELECT b.title, m.position AS number FROM box_members m JOIN books b ON b.id = m.book_id
		WHERE m.box_id = ? ORDER BY m.position`, boxID)
	return out
}

// BoxSetFor is one box set, for asking the AI what's inside.
func (s *Store) BoxSetFor(boxID int64) (*BoxSet, error) {
	var b BoxSet
	if err := s.DB.Get(&b, `SELECT x.book_id, bk.title, bk.author, bk.series, x.state, x.proposal
		FROM box_sets x JOIN books bk ON bk.id = x.book_id WHERE x.book_id = ?`, boxID); err != nil {
		return nil, ErrNotFound
	}
	return &b, nil
}

// SetBoxProposal saves what the AI says is inside a box set.
func (s *Store) SetBoxProposal(boxID int64, entries []BoxEntry) error {
	raw, _ := json.Marshal(entries)
	return s.mustChange(s.DB.Exec(`UPDATE box_sets SET proposal = ?, updated_at = CURRENT_TIMESTAMP WHERE book_id = ? AND state = 'found'`,
		string(raw), boxID))
}

// SplitBox makes a box set's books: each title joins the book NovelCheck
// already has under it, or becomes a new one (by the box set's author). It
// returns the books that still need a rating.
func (s *Store) SplitBox(boxID int64, entries []BoxEntry, by string) ([]int64, error) {
	var box struct {
		Author string `db:"author"`
		Series string `db:"series"`
	}
	if err := s.DB.Get(&box, `SELECT b.author, b.series FROM books b JOIN box_sets x ON x.book_id = b.id
		WHERE b.id = ? AND x.state = 'found'`, boxID); err != nil {
		return nil, ErrNotFound
	}
	var unrated []int64
	pos := 0
	for _, e := range entries {
		title := strings.TrimSpace(e.Title)
		if title == "" || len([]rune(title)) > 300 {
			continue
		}
		var existed int
		_ = s.DB.Get(&existed, `SELECT COUNT(*) FROM books WHERE norm_key = ?`, s.aliasKey(NormKey(title, box.Author)))
		id, err := s.UpsertBook(title, box.Author, "", "")
		if err != nil || id == boxID {
			continue
		}
		pos++
		if box.Series != "" && e.Number > 0 {
			_, _ = s.DB.Exec(`UPDATE books SET series = CASE WHEN series = '' THEN ? ELSE series END,
				series_index = CASE WHEN series_index = 0 THEN ? ELSE series_index END WHERE id = ?`, box.Series, e.Number, id)
		}
		if _, err := s.DB.Exec(`INSERT OR IGNORE INTO box_members (box_id, book_id, position, created) VALUES (?, ?, ?, ?)`,
			boxID, id, pos, existed == 0); err != nil {
			return unrated, err
		}
		var status string
		_ = s.DB.Get(&status, `SELECT status FROM books WHERE id = ?`, id)
		if status == "pending" || status == "error" {
			unrated = append(unrated, id)
		}
	}
	if pos == 0 {
		return nil, errors.New("tick at least one book")
	}
	_, err := s.DB.Exec(`UPDATE box_sets SET state = 'split', decided_by = ?, updated_at = CURRENT_TIMESTAMP WHERE book_id = ?`, by, boxID)
	return unrated, err
}

// NotBoxSet records that a book isn't a box set after all.
func (s *Store) NotBoxSet(boxID int64, by string) error {
	return s.mustChange(s.DB.Exec(`UPDATE box_sets SET state = 'not_box', decided_by = ?, updated_at = CURRENT_TIMESTAMP
		WHERE book_id = ?`, by, boxID))
}

// UndoSplit puts a box set back: books the split made (and nothing else
// uses) go, the rest stay as they were.
func (s *Store) UndoSplit(boxID int64) error {
	if _, err := s.DB.Exec(`DELETE FROM books WHERE id IN (SELECT book_id FROM box_members WHERE box_id = ? AND created = 1)
		AND NOT EXISTS (SELECT 1 FROM catalog_books c WHERE c.book_id = books.id)
		AND NOT EXISTS (SELECT 1 FROM box_members o WHERE o.book_id = books.id AND o.box_id != ?)`, boxID, boxID); err != nil {
		return err
	}
	if _, err := s.DB.Exec(`DELETE FROM box_members WHERE box_id = ?`, boxID); err != nil {
		return err
	}
	return s.mustChange(s.DB.Exec(`UPDATE box_sets SET state = 'found', updated_at = CURRENT_TIMESTAMP WHERE book_id = ?`, boxID))
}

// BoxSetsWaiting is how many box sets wait for a parent.
func (s *Store) BoxSetsWaiting() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM box_sets WHERE state = 'found'`)
	return n
}

// BoxState is what a parent decided about a box set ("" if it isn't one).
func (s *Store) BoxState(bookID int64) string {
	var st string
	_ = s.DB.Get(&st, `SELECT state FROM box_sets WHERE book_id = ?`, bookID)
	return st
}

// BoxOf is the split box set a book is in (0 if none), with its title.
func (s *Store) BoxOf(bookID int64) (int64, string) {
	var box struct {
		ID    int64  `db:"id"`
		Title string `db:"title"`
	}
	_ = s.DB.Get(&box, `SELECT b.id, b.title FROM box_members m JOIN books b ON b.id = m.box_id WHERE m.book_id = ? LIMIT 1`, bookID)
	return box.ID, box.Title
}
