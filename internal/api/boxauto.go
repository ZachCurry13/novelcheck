package api

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/epub"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Box sets by themselves: NovelCheck reads what a box set holds (its own table of contents when it has a file, else the AI) and, when
// it is sure, splits it into its books: sure means the title says how many books are inside, that many were found, and (when the AI
// named them) each one is a real book by that author. Anything less is only filled in for a parent to confirm. A split can be undone.
// It runs with the same switch as reading the first pages of unsure books ("Identify books").

const boxTried = 7 * 24 * time.Hour // a box set that could not be settled is looked at again after a week

// BoxLoop looks at waiting box sets every half hour.
func (s *Server) BoxLoop(ctx context.Context) {
	t := time.NewTicker(30 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s.identifyOn() {
				s.boxOnce(ctx, 4)
			}
		}
	}
}

var boxCountWords = map[string]int{"duology": 2, "trilogy": 3, "quartet": 4, "quintet": 5, "hexalogy": 6}
var boxNumBooks = regexp.MustCompile(`(?i)\b(\d+)\s*[-\s]?(?:books?|novels?|stories)\b|\b(\d+)\s*in\s*1\b`)

// boxCount is how many books a box set's title says it holds (0 when it does not say).
func boxCount(title string) int {
	if m := regexp.MustCompile(`(?i)books?\s+(\d+)\s*(?:[-–—]|to|through)\s*(\d+)`).FindStringSubmatch(title); m != nil {
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		if b > a && b-a < 30 {
			return b - a + 1
		}
	}
	low := strings.ToLower(title)
	for w, n := range boxCountWords {
		if strings.Contains(low, w) {
			return n
		}
	}
	if m := boxNumBooks.FindStringSubmatch(title); m != nil {
		n, _ := strconv.Atoi(m[1] + m[2])
		if n >= 2 && n < 30 {
			return n
		}
	}
	return 0
}

// boxOnce settles up to limit waiting box sets and says how many it split.
func (s *Server) boxOnce(ctx context.Context, limit int) int {
	if _, err := s.Store.FindBoxSets(); err != nil {
		return 0
	}
	list, err := s.Store.BoxSets()
	if err != nil {
		return 0
	}
	split, filled, tried := 0, 0, 0
	for _, box := range list {
		if box.State != "found" || tried >= limit || ctx.Err() != nil {
			continue
		}
		key := fmt.Sprintf("box_auto_%d", box.BookID)
		if at, err := time.Parse(time.RFC3339, s.Store.Setting(key)); err == nil && time.Since(at) < boxTried {
			continue
		}
		tried++
		_ = s.Store.SetSetting(key, time.Now().UTC().Format(time.RFC3339))
		entries, fromFile := s.boxContents(ctx, box)
		if len(entries) < 2 {
			continue
		}
		if n := boxCount(box.Title); n == len(entries) && (fromFile || s.boxTitlesReal(ctx, entries, box.Author)) {
			if unrated, err := s.Store.SplitBox(box.BookID, entries, "automatic"); err == nil {
				if len(unrated) > 0 && s.Worker != nil {
					s.Worker.Enqueue(true, unrated...)
				}
				split++
				s.Store.NotifyRoutine("box-split", fmt.Sprintf("Split “%s” into its %d books by itself. Undo it under Admin → Library → Box sets if it is wrong.", box.Title, len(entries)), "#/admin?tab=cleanup")
				continue
			}
		}
		if err := s.Store.SetBoxProposal(box.BookID, entries); err == nil {
			filled++ // not sure enough to split: what it found is filled in for a parent to confirm
		}
	}
	if filled > 0 {
		s.Store.NotifyRoutine("box-filled", fmt.Sprintf("%d box set%s read; the books inside are filled in for you to confirm.", filled, map[bool]string{true: "", false: "s"}[filled == 1]), "#/admin?tab=cleanup")
	}
	return split
}

// boxContents is what a box set holds: its own table of contents when there is a file, else the AI's answer (fromFile false).
func (s *Server) boxContents(ctx context.Context, box store.BoxSet) ([]store.BoxEntry, bool) {
	if s.Deep != nil {
		if path, ok := s.Deep.EPUBFor(box.BookID); ok {
			if titles, _ := epub.Books(path); len(titles) >= 2 {
				out := make([]store.BoxEntry, len(titles))
				for i, t := range titles {
					out[i] = store.BoxEntry{Title: t, Number: float64(i + 1)}
				}
				return out, true
			}
		}
	}
	book, err := s.Store.BookByID(box.BookID, nil)
	if err != nil {
		return nil, false
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var found []store.BoxEntry
	if _, err := llm.Ask(cctx, s.Store, s.Store.AIConfigs(), box.BookID, llm.BoxContentsSystem,
		llm.BoxContentsUser(box.Title, box.Author, box.Series, book.Description), func(out string) error {
			var perr error
			found, perr = llm.ParseBoxContents(out)
			return perr
		}); err != nil {
		return nil, false
	}
	return found, false
}

// boxTitlesReal says whether every title is a real book by this author (one NovelCheck has, or Open Library knows).
func (s *Server) boxTitlesReal(ctx context.Context, entries []store.BoxEntry, author string) bool {
	for _, e := range entries {
		if !s.realBook(ctx, enrich.Found{Title: e.Title, Author: author}) {
			return false
		}
	}
	return true
}
