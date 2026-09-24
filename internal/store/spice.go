package store

// The pepper scale (0-5), in the family's own words (see the Library's
// "What do the peppers mean?"):
//
//	0 No Romance · 1 Sweet Romance · 2 Mild / Closed Door ·
//	3 Steamy / Heavy Tension ("gray area") · 4 Explicit / Open Door ·
//	5 Very Explicit / Erotica
const MaxSpiceLevel = 5

// RulesVersion identifies the AI's rating rules (pepper descriptions and
// content flags). Bump it whenever they change: books the AI rated under an
// older version are then offered for re-rating. 2 = v1.16 pepper wording.
const RulesVersion = 2

// ValidSpice reports whether level is on the 0-5 pepper scale.
func ValidSpice(level int) bool { return level >= 0 && level <= MaxSpiceLevel }

// ClassificationForSpice maps peppers onto the older three-way label that the
// "Open Door" filter and kids' rules still use: no intimacy (0-1), intimacy
// off the page or short of explicit (2-3), sex on the page (4-5).
func ClassificationForSpice(level int) string {
	switch {
	case level >= 4:
		return "Open Door"
	case level >= 2:
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

// AISummary is an AI-written summary, for the language check.
type AISummary struct {
	ID      int64  `db:"id"`
	Summary string `db:"summary_verdict"`
}

// AISummaries lists analyzed books' AI-written summaries (not hand ratings).
func (s *Store) AISummaries() ([]AISummary, error) {
	var out []AISummary
	err := s.DB.Select(&out, `SELECT id, summary_verdict FROM books WHERE status = 'analyzed'
		AND summary_verdict != '' AND analysis_model NOT LIKE 'manual:%'`)
	return out, err
}

// RerateCandidates returns books the AI rated before the pepper scale or
// under older rating rules (hand-rated books are left alone).
func (s *Store) RerateCandidates() ([]int64, error) {
	var ids []int64
	err := s.DB.Select(&ids, `SELECT id FROM books WHERE status = 'analyzed'
		AND (spice_level IS NULL OR rules_version < ?) AND analysis_model NOT LIKE 'manual:%' ORDER BY id`, RulesVersion)
	return ids, err
}

// AIRatedIDs returns every book the AI has rated, for "Re-rate whole library".
func (s *Store) AIRatedIDs() ([]int64, error) {
	var ids []int64
	err := s.DB.Select(&ids, `SELECT id FROM books WHERE status = 'analyzed'
		AND analysis_model NOT LIKE 'manual:%' ORDER BY id`)
	return ids, err
}
