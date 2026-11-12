package api

import "net/http"

// handleKeptBelow: GET /api/admin/deep-scans/kept-below lists the books whose Deep Scan found Level 1 or 2 but
// were kept lower; POST puts them back in the review list.
func (s *Server) handleKeptBelow(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		n, err := s.Store.ReopenKeptBelow()
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"reopened": n})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": s.Store.KeptBelowScan()})
}
