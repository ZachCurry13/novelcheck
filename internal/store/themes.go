package store

import (
	"strconv"
	"strings"
)

// themeText is a book's searchable words.
const themeText = `(' ' || LOWER(b.title || ' ' || b.tags || ' ' || b.premise || ' ' || b.blurb || ' ' || b.description) || ' ')`

// ThemeCandidates finds the family's books that may fit a collection theme:
// those with any of words in their title, tags or description, or in one of
// genres (a word counts double: it says more than a broad genre), most
// matches first, rated books before unrated ones, up to limit. With no words
// or genres it returns a spread of the library. exclude skips books already
// chosen or taken off the shelf.
func (s *Store) ThemeCandidates(words, genres []string, kind string, exclude []int64, limit int) ([]Book, error) {
	var score []string
	var args []any
	for _, w := range words {
		if w = strings.ToLower(strings.TrimSpace(w)); len(w) >= 3 {
			score = append(score, `2 * (`+themeText+` LIKE ?)`)
			args = append(args, "% "+w+"%")
		}
	}
	for _, g := range genres {
		if g = strings.ToLower(strings.TrimSpace(g)); g != "" {
			score = append(score, `(b.genres LIKE ?)`)
			args = append(args, "%,"+g+",%")
		}
	}
	expr := "0"
	if len(score) > 0 {
		expr = strings.Join(score, " + ")
	}
	where := ownedCond + " AND NOT " + discoverOnlyCond
	if len(score) > 0 {
		where += " AND (" + expr + ") > 0"
	}
	if kind == "fiction" || kind == "nonfiction" {
		where += " AND b.kind = '" + kind + "'"
	}
	skip := map[int64]bool{}
	for _, id := range exclude {
		skip[id] = true
	}
	order := `score DESC, b.status = 'analyzed' DESC, b.id DESC`
	if len(score) == 0 {
		order = `b.status = 'analyzed' DESC, RANDOM()`
	}
	var rows []struct {
		Book
		Score int `db:"score"`
	}
	// The score's words appear twice (SELECT and WHERE), so their args do too.
	all := append(append([]any{}, args...), args...)
	err := s.DB.Select(&rows, `SELECT `+bookCols+derivedCols+`, '' AS catalogs, (`+expr+`) AS score FROM books b
		WHERE `+where+` ORDER BY `+order+` LIMIT `+strconv.Itoa(limit+len(skip)), all...)
	if err != nil {
		return nil, err
	}
	out := []Book{}
	for _, r := range rows {
		if !skip[r.ID] && len(out) < limit {
			out = append(out, r.Book)
		}
	}
	return out, nil
}

// LibrarySample describes the family's library for the AI's collection
// ideas: how many books each genre has and a spread of rated titles.
func (s *Store) LibrarySample(n int) (genreCounts map[string]int, books []Book, err error) {
	genreCounts = map[string]int{}
	var gs []string
	if err = s.DB.Select(&gs, `SELECT b.genres FROM books b WHERE `+ownedCond+` AND b.genres != ''`); err != nil {
		return nil, nil, err
	}
	for _, g := range gs {
		for _, k := range strings.Split(strings.Trim(g, ","), ",") {
			if k != "" {
				genreCounts[k]++
			}
		}
	}
	books = []Book{}
	err = s.DB.Select(&books, `SELECT `+bookCols+derivedCols+`, '' AS catalogs FROM books b
		WHERE `+ownedCond+` AND b.status = 'analyzed' ORDER BY RANDOM() LIMIT ?`, n)
	return genreCounts, books, err
}
