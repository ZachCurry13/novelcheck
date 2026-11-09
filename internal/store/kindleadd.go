package store

import (
	"errors"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/titles"
)

// AddDeviceBook makes a book from an entry on an e-reader that is not in any
// library ("Add to NovelCheck"): it joins the Looked up list under its tidy
// title and author, so it can be rated, counted in stats and found again. A
// book with that title and author that NovelCheck already knows is reused.
func (s *Store) AddDeviceBook(ref MatchRef) (int64, error) {
	title, author := strings.TrimSpace(ref.Title), strings.TrimSpace(ref.Authors)
	if title == "" {
		if gs := titles.FromFilename(ref.Filename); len(gs) > 0 {
			title, author = gs[0].Title, gs[0].Author
		}
	}
	title = titles.Clean(title)
	if title == "" {
		return 0, errors.New("that entry has no title to add")
	}
	if junkAuthors[strings.ToLower(author)] {
		author = ""
	}
	if id := s.MatchBook(title, author); id > 0 {
		return id, nil
	}
	id, err := s.UpsertBook(title, author, "", "")
	if err != nil {
		return 0, err
	}
	cat, err := s.EnsureCatalog(LookedUpCatalog, "custom")
	if err != nil {
		return 0, err
	}
	return id, s.AddCopy(cat, id, "lookup:"+title, "list", "")
}
