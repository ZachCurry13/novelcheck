package api

import "os"

// adoptOldCover gives a book the cover that was found for it before its row was deleted and made again (a free
// book that comes back on the next event, a book added back to Calibre): found covers are stored under the
// book's id, and the rating memory (db/ratingmemory.go) knows the ids it had. No search on the web is needed.
func (s *Server) adoptOldCover(id int64) bool {
	for _, old := range s.Store.PreviousBookIDs(id) {
		path, _, ok := s.Covers.Stored(s.coverName(old))
		if !ok {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if _, _, ok := s.Covers.Keep(s.coverName(id), data); ok {
			return true
		}
	}
	return false
}
