package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/auth"
)

// handleActivity is what the AI is doing now, for the header's activity pill
// and the live status on book cards (parents): the book being rated, how
// many wait, whether automatic rating is on and allowed right now, and the
// Deep Scan being read.
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	st := s.Worker.Status()
	out := map[string]any{
		"state":      st.State,
		"book_id":    st.CurrentBook,
		"title":      st.CurrentName,
		"queue":      st.QueueLength,
		"waiting":    s.Store.WaitingToRate(),
		"auto":       analyzer.AutoRateOn(s.Store),
		"hours_open": analyzer.RateHoursOpen(s.Store, time.Now()),
	}
	if d := s.Store.ReadingDeepScan(); d != nil {
		out["deep"] = map[string]any{"id": d.ID, "book_id": d.BookID, "title": d.Title, "part": d.PartsDone + 1, "parts": d.PartsTotal}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleBookStates returns the listed books as the viewer sees them
// (?ids=1,2,3, at most 100), so cards can show a new rating in place.
func (s *Server) handleBookStates(w http.ResponseWriter, r *http.Request) {
	var ids []int64
	for _, p := range strings.Split(r.URL.Query().Get("ids"), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	books, err := s.Store.BooksByIDs(ids, auth.UserFrom(r))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": books})
}
