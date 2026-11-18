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

// MatchDeviceBook finds the book NovelCheck already knows for a file on an e-reader, trying every reading of the
// file name ("Author - Title" and "Title - Author"), without making anything up. 0 means it does not know it.
func (s *Store) MatchDeviceBook(ref MatchRef) int64 {
	title, author := strings.TrimSpace(ref.Title), strings.TrimSpace(ref.Authors)
	var tries [][2]string
	if title != "" {
		tries = append(tries, [2]string{title, author})
	}
	for _, g := range titles.FromFilename(ref.Filename) {
		tries = append(tries, [2]string{g.Title, g.Author})
	}
	for _, t := range tries {
		ti, au := titles.Clean(TidyBookTitle(t[0])), calibreNumber.ReplaceAllString(t[1], "") // "Fall of Five, The", "(4006)"
		if junkAuthors[strings.ToLower(au)] {
			au = ""
		}
		if ti == "" {
			continue
		}
		if id := s.MatchBook(ti, au); id > 0 {
			return id
		}
	}
	return 0
}
