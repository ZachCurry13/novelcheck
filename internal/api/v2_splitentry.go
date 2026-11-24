package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleSplitDuplicates: POST /api/admin/calibre/duplicates/split {"calibre_ids": ["1724"]}: the ticked entries are not
// copies of the others ("Not a copy"): they become a book of their own, and it is remembered. When a ticked entry holds
// the book's own name, the others are what is split away instead (the two sides are different books either way).
func (s *Server) handleSplitDuplicates(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CalibreIDs []string `json:"calibre_ids"`
	}
	if !readJSON(w, r, &body, 4<<10) || len(body.CalibreIDs) == 0 {
		writeErr(w, http.StatusBadRequest, "tick the copies that are a different book")
		return
	}
	groups, err := s.Store.CalibreDuplicates()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	var all []string
	for _, g := range groups {
		for _, e := range g.Entries {
			if e.CalibreID == body.CalibreIDs[0] {
				for _, x := range g.Entries {
					all = append(all, x.CalibreID)
				}
			}
		}
	}
	if len(all) == 0 {
		writeErr(w, http.StatusConflict, "that book isn't listed twice any more; refresh the list")
		return
	}
	ticked := map[string]bool{}
	for _, id := range body.CalibreIDs {
		ticked[id] = true
	}
	var nums []int
	for _, id := range all {
		if n, err := strconv.Atoi(id); err == nil {
			nums = append(nums, n)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	details, err := s.calibreClient().Details(ctx, nums)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "couldn't ask Calibre what these entries are called: "+err.Error())
		return
	}
	split := func(want bool) (n int, theName bool, err error) {
		for _, id := range all {
			if ticked[id] != want {
				continue
			}
			num, _ := strconv.Atoi(id)
			d, ok := details[num]
			if !ok {
				continue
			}
			switch e := s.Store.SplitCalibreEntry(id, d.Title, strings.Join(d.Authors, " & ")); {
			case e == nil:
				n++
			case errors.Is(e, store.ErrIsTheName):
				theName = true
			default:
				err = e
			}
		}
		return
	}
	n, theName, err := split(true)
	if err == nil && theName { // a ticked entry is the book's own name: the others leave instead
		var m int
		m, _, err = split(false)
		n += m
	}
	if err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if n == 0 {
		writeErr(w, http.StatusConflict, "nothing was split: those entries are already separate books")
		return
	}
	go func() { // read Calibre again, so each entry is filed under its own book
		s.refreshMu.Lock()
		defer s.refreshMu.Unlock()
		s.refreshCalibre()
	}()
	writeJSON(w, http.StatusOK, map[string]int{"split": n})
}
