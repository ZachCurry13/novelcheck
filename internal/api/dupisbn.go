package api

import (
	"context"
	"log"
)

// carryISBN gives the copies of a book that stay in Calibre the ISBN of a copy being removed, when none of them
// has one, so removing a duplicate (another edition, say) doesn't take the book's ISBN with it. Calibre's other
// identifiers are kept. A failure is only logged: the removed copy goes to Calibre's recycle bin, ISBN and all.
func (s *Server) carryISBN(ctx context.Context, stay, going []int) {
	cal := s.calibreClient()
	have, err := cal.Details(ctx, append(append([]int{}, stay...), going...))
	if err != nil {
		return
	}
	isbn := ""
	for _, id := range going {
		if v := have[id].Identifiers["isbn"]; v != "" && isbn == "" {
			isbn = v
		}
	}
	for _, id := range stay {
		if have[id].Identifiers["isbn"] != "" {
			return // the book keeps an ISBN already
		}
	}
	if isbn == "" {
		return
	}
	for _, id := range stay {
		d, ok := have[id]
		if !ok {
			continue
		}
		ids := map[string]string{"isbn": isbn}
		for k, v := range d.Identifiers {
			if k != "isbn" {
				ids[k] = v
			}
		}
		if _, err := cal.SetFields(ctx, id, [][]any{{"identifiers", ids}}); err != nil {
			log.Printf("duplicates: ISBN %s to calibre #%d: %v", isbn, id, err)
		}
	}
}
