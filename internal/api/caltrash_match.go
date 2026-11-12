package api

// Calibre's recycle bin against the library as it is now: for each removed book, is there still a copy of the
// same book in Calibre? A removal that left another copy was most likely a duplicate cleanup; one that left none
// may have been a mistake, and is the one to put back.

// trashState is what the bin view shows beside a removed book.
type trashState struct {
	InLibrary    bool   `json:"in_library"`              // another Calibre copy of the same book is there now
	LibraryID    int64  `json:"library_id,omitempty"`    // NovelCheck's book, when it matched one
	LibraryTitle string `json:"library_title,omitempty"` // that book's title as the library has it
	LibraryAt    string `json:"library_at,omitempty"`    // the Calibre number of the copy that is there
}

type trashRow struct {
	trashBook
	trashState
}

// matchTrash looks each removed book up in the library (same title and author, as NovelCheck matches books).
// The copy that was removed does not count, even when the library has not been re-read since.
func (s *Server) matchTrash(books []trashBook) []trashRow {
	out := make([]trashRow, 0, len(books))
	for _, b := range books {
		r := trashRow{trashBook: b}
		if id := s.Store.MatchBook(b.Title, b.Author); id != 0 && !s.otherAuthor(id, b.Author) {
			r.LibraryID = id
			if book, err := s.Store.BookByID(id, nil); err == nil {
				r.LibraryTitle = book.Title
			}
			copies, _ := s.Store.BookCopies(id)
			for _, c := range copies {
				if c.Source == "calibre" && c.ExternalID != "" && c.ExternalID != b.ID {
					r.InLibrary, r.LibraryAt = true, c.ExternalID
					break
				}
			}
		}
		out = append(out, r)
	}
	return out
}
