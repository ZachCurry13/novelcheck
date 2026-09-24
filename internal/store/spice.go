package store

// The pepper scale (0-5), in the family's own words (see the Library's
// "What do the peppers mean?"):
//
//	0 No Romance · 1 Sweet Romance · 2 Romantic · 3 Steamy Closed-Door ·
//	4 Explicit · 5 Very Explicit / Erotica-Level
const MaxSpiceLevel = 5

// ValidSpice reports whether level is on the 0-5 pepper scale.
func ValidSpice(level int) bool { return level >= 0 && level <= MaxSpiceLevel }

// ClassificationForSpice maps peppers onto the older three-way label that the
// "Open Door" filter and kids' rules still use: nothing sexual (0-2), sex off
// the page (3), sex on the page (4-5).
func ClassificationForSpice(level int) string {
	switch {
	case level >= 4:
		return "Open Door"
	case level == 3:
		return "Closed Door"
	default:
		return "No Spice"
	}
}

// effectiveSpice is a book's pepper level for filtering. Books rated before
// the pepper scale use the highest level their old label allows ("No Spice"
// could be up to 2 peppers of kissing), so kids' limits stay on the safe side.
const effectiveSpice = `COALESCE(b.spice_level, CASE b.classification
	WHEN 'No Spice' THEN 2 WHEN 'Closed Door' THEN 3 WHEN 'Open Door' THEN 4 END)`

// RerateCandidates returns analyzed books rated by the AI before the pepper
// scale existed (hand-rated books are left alone).
func (s *Store) RerateCandidates() ([]int64, error) {
	var ids []int64
	err := s.DB.Select(&ids, `SELECT id FROM books WHERE status = 'analyzed' AND spice_level IS NULL
		AND analysis_model NOT LIKE 'manual:%' ORDER BY id`)
	return ids, err
}
