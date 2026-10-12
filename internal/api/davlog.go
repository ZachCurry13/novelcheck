package api

import (
	"sort"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// What the last few WebDAV requests did, so Profile can say why KOReader's
// Cloud storage fails (KOReader itself only says "Cannot fetch list of folder
// contents"). Kept in memory; names that match nobody go under user 0 and are
// shown to the main admin only.

const davLogKeep = 5

// DavAttempt is one WebDAV request and how it ended.
type DavAttempt struct {
	At     int64  `json:"at"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
	Name   string `json:"name,omitempty"`
	Note   string `json:"note"`
}

func (d *davState) note(userID int64, a DavAttempt) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.attempts == nil {
		d.attempts = map[int64][]DavAttempt{}
	}
	a.At = time.Now().Unix()
	list := append([]DavAttempt{a}, d.attempts[userID]...)
	if len(list) > davLogKeep {
		list = list[:davLogKeep]
	}
	d.attempts[userID] = list
}

func (d *davState) recent(userID int64) []DavAttempt {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]DavAttempt{}, d.attempts[userID]...)
}

// davAttemptsFor is what Profile shows: this person's, plus (for the main
// admin) the ones that matched no name.
func (s *Server) davAttemptsFor(u *store.User) []DavAttempt {
	list := s.dav.recent(u.ID)
	if u.Owner {
		list = append(list, s.dav.recent(0)...)
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].At > list[j].At })
	if len(list) > davLogKeep {
		list = list[:davLogKeep]
	}
	return list
}
