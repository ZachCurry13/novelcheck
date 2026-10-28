package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The Readers page: people in the family who chose to share their reading
// profile (a kid's is the parent's choice). Private until switched on. What
// anyone sees of someone else is limited to what they could see themselves:
// kids' content rules apply, personal files and notes are left out.

// canViewProfile says whether viewer may look at owner's reading stats.
func (s *Server) canViewProfile(viewer, owner *store.User) bool {
	if viewer.ID == owner.ID || s.Store.SharingOf(owner.ID).Profile {
		return true
	}
	return owner.Role == store.RoleRestricted && canManageUser(viewer, owner) // parents see their kids'
}

func queryUnix(r *http.Request, key string, def int64) int64 {
	if n, err := strconv.ParseInt(r.URL.Query().Get(key), 10, 64); err == nil && n >= 0 {
		return n
	}
	return def
}

// handleReaders: GET /api/readers?week=&month=&year= (the start of each, as
// the browser's Unix seconds), the people the viewer may look at with a few numbers.
func (s *Server) handleReaders(w http.ResponseWriter, r *http.Request) {
	viewer := auth.UserFrom(r)
	now := time.Now()
	week := queryUnix(r, "week", now.Add(-7*24*time.Hour).Unix())
	month := queryUnix(r, "month", time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Unix())
	year := queryUnix(r, "year", time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC).Unix())
	type nowReading struct {
		BookID  int64   `json:"book_id"`
		Title   string  `json:"title"`
		Percent float64 `json:"percent"`
		At      int64   `json:"at"`
	}
	type reader struct {
		store.ReaderRow
		WeekSec      int64       `json:"week_sec"`
		MonthSec     int64       `json:"month_sec"`
		FinishedYear int         `json:"finished_year"`
		Now          *nowReading `json:"now,omitempty"`
	}
	out := []reader{}
	for _, row := range s.Store.SharingReaders(viewer.ID, viewer.CanManage()) {
		owner, err := s.Store.UserByID(row.ID)
		if err != nil || !s.canViewProfile(viewer, owner) {
			continue
		}
		x := reader{ReaderRow: row, WeekSec: s.Store.ReadingSecondsSince(row.ID, week), MonthSec: s.Store.ReadingSecondsSince(row.ID, month),
			FinishedYear: s.Store.FinishedSince(row.ID, year)}
		if row.Shared && row.Current { // only when they chose to show it
			if id, title, at, pct := s.Store.NowReading(row.ID); id > 0 {
				if b, err := s.Store.BookByID(id, viewer); err == nil { // the viewer's content rules apply
					x.Now = &nowReading{BookID: id, Title: b.Title, Percent: pct, At: at}
					_ = title
				}
			}
		}
		out = append(out, x)
	}
	writeJSON(w, http.StatusOK, map[string]any{"readers": out})
}

// handleReaderStats: GET /api/readers/{id}/stats.
func (s *Server) handleReaderStats(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	viewer := auth.UserFrom(r)
	owner, err := s.Store.UserByID(id)
	if err != nil || owner == nil || !s.canViewProfile(viewer, owner) {
		writeErr(w, http.StatusNotFound, "no such reader") // not shared looks the same as not there
		return
	}
	out := s.readingStatsFor(owner, viewer)
	out["name"] = owner.Username
	writeJSON(w, http.StatusOK, out)
}

type sharingBody struct {
	Profile bool `json:"profile"`
	Current bool `json:"current"`
}

// handleMySharing: GET/PUT /api/me/sharing.
func (s *Server) handleMySharing(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	if r.Method == http.MethodPut {
		if u.Role == store.RoleRestricted {
			writeErr(w, http.StatusForbidden, "a parent decides whether your reading profile is shared")
			return
		}
		var b sharingBody
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&b); err != nil {
			writeErr(w, http.StatusBadRequest, "bad request")
			return
		}
		if err := s.Store.SetSharing(u.ID, store.Sharing{Profile: b.Profile, Current: b.Current}); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sharing": s.Store.SharingOf(u.ID), "kid": u.Role == store.RoleRestricted})
}

// handleKidsSharing: GET /api/admin/kids-sharing, the kids this parent can set sharing for.
func (s *Server) handleKidsSharing(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFrom(r)
	kids, err := s.Store.ListUsersByRole(store.RoleRestricted)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	type kid struct {
		ID       int64         `json:"id"`
		Username string        `json:"username"`
		Sharing  store.Sharing `json:"sharing"`
	}
	out := []kid{}
	for i := range kids {
		if canManageUser(actor, &kids[i]) {
			out = append(out, kid{kids[i].ID, kids[i].Username, s.Store.SharingOf(kids[i].ID)})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"kids": out})
}

// handleKidSharing: PUT /api/admin/users/{id}/sharing, a parent's choice for a kid.
func (s *Server) handleKidSharing(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	target, err := s.Store.UserByID(id)
	if err != nil || target == nil || target.Role != store.RoleRestricted || !canManageUser(auth.UserFrom(r), target) {
		writeErr(w, http.StatusNotFound, "no such kid account")
		return
	}
	var b sharingBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}
	if err := s.Store.SetSharing(target.ID, store.Sharing{Profile: b.Profile, Current: b.Current}); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sharing": s.Store.SharingOf(target.ID)})
}
