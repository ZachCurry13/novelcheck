package store

import "time"

// Hardcover ratings: each book's average rating from Hardcover's readers, looked up in the background
// (api/ratingloop.go) and again after a month, for the Library's "Highest rated" order and the ★ on its card.

// RatingToCheck is a book whose Hardcover rating is missing or a month old.
type RatingToCheck struct {
	ID     int64  `db:"id"`
	Title  string `db:"title"`
	Author string `db:"author"`
	ISBN   string `db:"isbn"`
}

// RatingsToCheck lists up to limit books in any library, list or event whose rating was never looked up or is
// over 30 days old, those never looked up first.
func (s *Store) RatingsToCheck(limit int) ([]RatingToCheck, error) {
	var out []RatingToCheck
	err := s.DB.Select(&out, `SELECT b.id, b.title, b.author, b.isbn FROM books b
		WHERE b.hc_rated_at < ? AND EXISTS (SELECT 1 FROM catalog_books cb WHERE cb.book_id = b.id)
		ORDER BY b.hc_rated_at, b.id DESC LIMIT ?`, time.Now().Add(-30*24*time.Hour).Unix(), limit)
	return out, err
}

// SaveHCRating keeps a book's Hardcover rating (0 and 0 when Hardcover had no rating, or no such book).
func (s *Store) SaveHCRating(id int64, rating float64, count int) {
	_, _ = s.DB.Exec(`UPDATE books SET hc_rating = ?, hc_ratings = ?, hc_rated_at = ? WHERE id = ?`, rating, count, time.Now().Unix(), id)
}

// RatingsToken is a Hardcover connection to look ratings up with: the main admin's, else any admin's, else
// anyone's in the family ("" when nobody connected Hardcover).
func (s *Store) RatingsToken() string {
	var t string
	_ = s.DB.Get(&t, `SELECT hardcover_token FROM users WHERE hardcover_token != '' ORDER BY owner DESC, role = 'admin' DESC, id LIMIT 1`)
	return t
}

// CountHCRatings is how many books have a Hardcover rating.
func (s *Store) CountHCRatings() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM books WHERE hc_rating > 0`)
	return n
}

// SetMissingISBN gives a book the ISBN found for it, when it has none; it says whether it did.
func (s *Store) SetMissingISBN(id int64, isbn string) bool {
	if isbn == "" {
		return false
	}
	res, err := s.DB.Exec(`UPDATE books SET isbn = ? WHERE id = ? AND isbn = ''`, isbn, id)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

// CountNoISBN is how many books in a library have no ISBN.
func (s *Store) CountNoISBN() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM books b WHERE b.isbn = '' AND `+ownedCond)
	return n
}

// SaveHCTags keeps a book's Hardcover content warnings and moods (comma-separated).
func (s *Store) SaveHCTags(id int64, warnings, moods string) {
	_, _ = s.DB.Exec(`UPDATE books SET hc_warnings = ?, hc_moods = ? WHERE id = ?`, warnings, moods, id)
}

// SaveHCID keeps the book's id on Hardcover (0 is ignored).
func (s *Store) SaveHCID(id int64, hcID int) {
	if hcID > 0 {
		_, _ = s.DB.Exec(`UPDATE books SET hc_id = ? WHERE id = ?`, hcID, id)
	}
}

// LibraryBookByKey is the book in a library, list or event with this title and author (NormKey), or 0.
func (s *Store) LibraryBookByKey(key string) int64 {
	var id int64
	_ = s.DB.Get(&id, `SELECT b.id FROM books b WHERE (b.norm_key = ? OR b.id IN (SELECT book_id FROM book_aliases WHERE norm_key = ?))
		AND EXISTS (SELECT 1 FROM catalog_books cb WHERE cb.book_id = b.id) ORDER BY b.id LIMIT 1`, key, key)
	return id
}

// LibraryBookByTitleKey is a library book whose title matches (ignoring case, spaces and punctuation) whoever the
// author is: titleKey is NormKey(title, "") up to its "|". 0 when there is none.
func (s *Store) LibraryBookByTitleKey(titleKey string) int64 {
	var id int64
	_ = s.DB.Get(&id, `SELECT b.id FROM books b WHERE b.norm_key LIKE ? AND EXISTS (SELECT 1 FROM catalog_books cb WHERE cb.book_id = b.id) ORDER BY b.id LIMIT 1`, titleKey+"|%")
	return id
}

// HCIDOf is the book's Hardcover id, 0 when not known yet.
func (s *Store) HCIDOf(id int64) int {
	var n int
	_ = s.DB.Get(&n, `SELECT hc_id FROM books WHERE id = ?`, id)
	return n
}

// RecheckHardcover has the background pass look at every book on Hardcover again (once, when the pass learned
// something new, like the true titles).
func (s *Store) RecheckHardcover() {
	_, _ = s.DB.Exec(`UPDATE books SET hc_rated_at = 0 WHERE hc_rated_at > 0`)
}

// PreviousBookIDs are the ids this book had before its row was deleted and made again (the rating memory knows
// them): the cover finder looks for a cover stored under one of them before searching the web.
func (s *Store) PreviousBookIDs(id int64) []int64 {
	var ids []int64
	_ = s.DB.Select(&ids, `SELECT m.last_id FROM rating_memory m JOIN books b ON b.norm_key = m.norm_key WHERE b.id = ? AND m.last_id != ?`, id, id)
	return ids
}

// NotSureToRecheck lists up to limit ratings the AI wasn't sure of (the Not sure chip: analyzed, low confidence, no
// parent OK) that are in a library, list or event, were not made by hand or by a Deep Scan, and have not had a
// second opinion from the bigger model under the current rules, newest first.
func (s *Store) NotSureToRecheck(limit int) ([]int64, error) {
	var ids []int64
	err := s.DB.Select(&ids, `SELECT b.id FROM books b WHERE b.status = 'analyzed' AND b.confidence = 'low' AND b.approved = 0
		AND b.big_rules < ? AND b.analysis_model NOT LIKE 'manual:%' AND b.analysis_model NOT LIKE 'deep:%'
		AND EXISTS (SELECT 1 FROM catalog_books cb WHERE cb.book_id = b.id) ORDER BY b.id DESC LIMIT ?`, RulesVersion, limit)
	return ids, err
}

// MarkBigChecked notes that the bigger model looked at this rating under the current rules.
func (s *Store) MarkBigChecked(id int64) {
	_, _ = s.DB.Exec(`UPDATE books SET big_rules = ? WHERE id = ?`, RulesVersion, id)
}

// TrialBook is a book picked to try the rating rules on, with the rating it has now.
type TrialBook struct {
	ID         int64  `db:"id" json:"id"`
	Title      string `db:"title" json:"title"`
	Author     string `db:"author" json:"author"`
	Level      *int   `db:"spice_level" json:"level"`
	Confidence string `db:"confidence" json:"confidence"`
	Reason     string `db:"spice_reason" json:"reason"`
	Warnings   string `db:"hc_warnings" json:"warnings"`
}

// TrialSample picks up to n AI-rated books of the family's libraries to try the rating rules on: a few with content
// warnings from Hardcover, and the rest spread over the levels 0 to 5 (the rarer levels are not left out).
func (s *Store) TrialSample(n int) ([]TrialBook, error) {
	const cols = `b.id, b.title, b.author, b.spice_level, b.confidence, b.spice_reason, b.hc_warnings`
	const base = ` FROM books b WHERE b.status = 'analyzed' AND b.spice_level IS NOT NULL AND b.analysis_model NOT LIKE 'manual:%' AND b.analysis_model NOT LIKE 'deep:%'
		AND ` + ownedCond + ` AND NOT ` + discoverOnlyCond
	var out []TrialBook
	taken := map[int64]bool{}
	add := func(rows []TrialBook) {
		for _, r := range rows {
			if !taken[r.ID] && len(out) < n {
				taken[r.ID], out = true, append(out, r)
			}
		}
	}
	var warned []TrialBook
	if err := s.DB.Select(&warned, `SELECT `+cols+base+` AND b.hc_warnings != '' ORDER BY RANDOM() LIMIT ?`, max(n/4, 1)); err != nil {
		return nil, err
	}
	add(warned)
	per := (n + 5) / 6
	for level := 0; level <= 5; level++ {
		var rows []TrialBook
		if err := s.DB.Select(&rows, `SELECT `+cols+base+` AND b.spice_level = ? ORDER BY RANDOM() LIMIT ?`, level, per); err != nil {
			return nil, err
		}
		add(rows)
	}
	if len(out) < n {
		var rest []TrialBook
		_ = s.DB.Select(&rest, `SELECT `+cols+base+` ORDER BY RANDOM() LIMIT ?`, n*2)
		add(rest)
	}
	return out, nil
}
