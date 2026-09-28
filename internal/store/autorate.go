package store

// Automatic rating: books waiting to be rated are fed to the AI while it
// has nothing else to do (see analyzer.Worker.feed).

const (
	KeyAutoRate      = "auto_rate"       // "" = on with a local AI, off with a paid one; "on"; "off"
	KeyAutoRateHours = "auto_rate_hours" // "" = any time, or "23-7": only from 23:00 until 07:00
	KeyAutoRateTZ    = "auto_rate_tz"    // time zone for those hours (the admin's browser's)
)

func init() {
	Defaults[KeyAutoRate] = ""
	Defaults[KeyAutoRateHours] = ""
	Defaults[KeyAutoRateTZ] = ""
}

// rateOrder puts books people want first (in someone's Up Next or on a
// wishlist), then the newest additions.
const rateOrder = `EXISTS (SELECT 1 FROM queue_items qo WHERE qo.book_id = b.id AND qo.status IN ('queued', 'reading'))
		OR EXISTS (SELECT 1 FROM wishlist wo WHERE wo.book_id = b.id AND wo.status IN ('wanted', 'approved')) DESC,
	(SELECT MAX(co.added_at) FROM catalog_books co WHERE co.book_id = b.id) DESC, b.id DESC`

// WaitingToRate counts the family's books still waiting for a rating
// (Discover-only books have their own daily allowance).
func (s *Store) WaitingToRate() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM books b WHERE b.status IN ('pending', 'queued') AND NOT `+discoverOnlyCond)
	return n
}

// ReadingDeepScan is the Deep Scan being read right now, or nil.
func (s *Store) ReadingDeepScan() *DeepRead {
	var d DeepRead
	if err := s.DB.Get(&d, `SELECT `+deepCols+` FROM deep_reads d JOIN books b ON b.id = d.book_id
		WHERE d.status = 'reading' ORDER BY d.updated_at DESC LIMIT 1`); err != nil {
		return nil
	}
	return &d
}

// BooksByIDs returns the listed books the viewer may see, for refreshing
// cards in place (at most 100).
func (s *Store) BooksByIDs(ids []int64, viewer *User) ([]Book, error) {
	out := []Book{}
	if len(ids) == 0 {
		return out, nil
	}
	if len(ids) > 100 {
		ids = ids[:100]
	}
	where, args := visibilityClause(viewer)
	marks := "?"
	idArgs := []any{ids[0]}
	for _, id := range ids[1:] {
		marks += ",?"
		idArgs = append(idArgs, id)
	}
	err := s.DB.Select(&out, `SELECT `+bookCols+derivedCols+`, '' AS catalogs FROM books b WHERE b.id IN (`+marks+`)`+where,
		append(idArgs, args...)...)
	return out, err
}
