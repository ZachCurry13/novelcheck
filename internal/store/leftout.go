package store

import (
	"strconv"
	"strings"
)

// keyLeftOut lists the books the owner said are not books worth a cover (a Calibre junk entry, a free comic): the cover
// finder and its lists leave them alone.
const keyLeftOut = "cover_left_out"

// LeftOut is the ids of the books left out of the cover lists.
func (s *Store) LeftOut() map[int64]bool {
	out := map[int64]bool{}
	for _, f := range strings.Split(s.Setting(keyLeftOut), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(f), 10, 64); err == nil {
			out[id] = true
		}
	}
	return out
}

// LeaveOut adds a book to the left-out list.
func (s *Store) LeaveOut(id int64) error {
	m := s.LeftOut()
	if m[id] {
		return nil
	}
	m[id] = true
	ids := make([]string, 0, len(m))
	for k := range m {
		ids = append(ids, strconv.FormatInt(k, 10))
	}
	return s.SetSetting(keyLeftOut, strings.Join(ids, ","))
}
