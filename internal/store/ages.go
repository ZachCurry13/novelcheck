package store

// Age groups used for books (set by a parent) and kid accounts. A kid sees
// books rated for their group or younger; books without an age group fall
// back to the account's content rules.
type AgeGroup struct {
	Level int    `json:"level"`
	Key   string `json:"key"`
	Label string `json:"label"`
}

var AgeGroups = []AgeGroup{
	{1, "young_kids", "Young kids (up to 8)"},
	{2, "middle_grade", "Middle grade (9–12)"},
	{3, "teens", "Teens (13–15)"},
	{4, "young_adult", "Young adult (16–17)"},
	{5, "adults", "Adults (18+)"},
}

// ValidAge reports whether level is 0 (not set) or a known age group.
func ValidAge(level int) bool { return level >= 0 && level <= len(AgeGroups) }

// agePresets are the starting content rules for a new kid account of each age
// group. Parents can change any of them afterwards.
var agePresets = map[int]User{
	1: {MaxSpice: 0, HideOpenDoor: true, HideNudity: true, HideSoloActs: true, HideInnuendo: true, HideDarkOccult: true, HideUnrated: true},
	2: {MaxSpice: 1, HideOpenDoor: true, HideNudity: true, HideSoloActs: true, HideInnuendo: true, HideDarkOccult: true, HideUnrated: true},
	3: {MaxSpice: 2, HideOpenDoor: true, HideNudity: true, HideSoloActs: true, HideInnuendo: true, HideDarkOccult: true, HideUnrated: true},
	4: {MaxSpice: 3, HideOpenDoor: true, HideSoloActs: true, HideDarkOccult: true, HideUnrated: true},
	5: {MaxSpice: -1},
}

// SetBookAge records a parent's age group for a book (0 clears it).
func (s *Store) SetBookAge(id int64, level int, by string) error {
	if level == 0 {
		by = ""
	}
	_, err := s.DB.Exec(`UPDATE books SET age_level = ?, age_set_by = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, level, by, id)
	return err
}
