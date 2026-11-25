package store

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
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

// ShowAllLeftOut empties the list.
func (s *Store) ShowAllLeftOut() error { return s.SetSetting(keyLeftOut, "") }

const keyNothingSince = "cover_nothing_since"

// ConfirmNothingFound takes the books a cover search found nothing for and says which of them to leave out: only those
// that were also missing at least gap ago. One search can miss a cover because a site was down for a while, so a book is
// left out only after a second sighting on another day.
func (s *Store) ConfirmNothingFound(ids []int64, now time.Time, gap time.Duration) []int64 {
	since := map[string]int64{}
	_ = json.Unmarshal([]byte(s.Setting(keyNothingSince)), &since)
	next := map[string]int64{}
	var out []int64
	for _, id := range ids {
		k := strconv.FormatInt(id, 10)
		first, seen := since[k]
		switch {
		case !seen:
			next[k] = now.Unix()
		case now.Sub(time.Unix(first, 0)) >= gap:
			out = append(out, id)
		default:
			next[k] = first
		}
	}
	raw, _ := json.Marshal(next)
	_ = s.SetSetting(keyNothingSince, string(raw))
	return out
}
