package api

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/zachcurry13/novelcheck/internal/deepread"
	"github.com/zachcurry13/novelcheck/internal/epub"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Identifying books from their own first pages. A book NovelCheck is unsure of (a title that looks like a file name, a book
// Hardcover does not know) or unsure is a duplicate of another is read: the first pages of its EPUB go to the AI, which says
// what the title page prints. The answer only counts when that title really is in the text. Then:
//   - a wrongly named book is renamed in Calibre (and its author set when it had none), with the old name in the cleanup log;
//   - two entries that are probably one book are joined when both say the same title and author, and marked as different
//     books when their pages name different titles. Anything less certain is left for a person.

const keyIdentify = "identify_auto" // "off" turns it off

func (s *Server) identifyOn() bool { return s.Store.Setting(keyIdentify) != "off" }

func norm(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}

// printedIn says whether the title really is written in the text (the AI must not make one up).
func printedIn(text, title string) bool {
	t := norm(title)
	return len(t) >= 3 && strings.Contains(norm(text), t)
}

// looksOff says whether a title looks like a file name or junk.
func looksOff(title string) bool {
	l := strings.ToLower(strings.TrimSpace(title))
	return l == "" || l == "unknown" || l == "untitled" || strings.Contains(l, "_") || strings.Contains(l, "oceanofpdf") || strings.Contains(l, ".epub")
}

func authorUnknown(a string) bool {
	l := strings.ToLower(strings.TrimSpace(a))
	return l == "" || l == "unknown" || l == "unknown author"
}

// identifyBook says what the book's first pages say it is (kept, so each book is read once). ok is false when it could not be told.
func (s *Server) identifyBook(ctx context.Context, bookID int64) (store.Identity, bool) {
	if id, tried := s.Store.IdentityOf(bookID); tried {
		return id, id.Title != ""
	}
	if s.Deep == nil {
		return store.Identity{}, false
	}
	path, found := s.Deep.EPUBFor(bookID)
	if !found {
		s.Store.SaveIdentity(bookID, "", "", "") // no EPUB to read (an audiobook, a PDF): not asked again for a month
		return store.Identity{}, false
	}
	fm, err := epub.Front(path) // the package's metadata, the title page, the copyright page: where a book says what it is
	isbn := ""
	if len(fm.ISBNs) > 0 {
		isbn = fm.ISBNs[0]
	}
	if err != nil {
		s.Store.SaveIdentity(bookID, "", "", "")
		return store.Identity{}, false
	}
	front := fm.TitlePage + " " + fm.CopyrightPage
	// 1. The file's own title and author, when its title page or copyright page bears them out: no AI needed.
	if !looksOff(fm.Title) && !authorUnknown(fm.Author) && printedIn(front, fm.Title) {
		s.Store.SaveIdentity(bookID, fm.Title, fm.Author, isbn)
		return store.Identity{Title: fm.Title, Author: fm.Author, ISBN: isbn}, true
	}
	// 2. Otherwise the AI reads just the pages that say it.
	if strings.TrimSpace(front+fm.Opening) == "" {
		s.Store.SaveIdentity(bookID, "", "", isbn)
		return store.Identity{}, false
	}
	var got llm.Identity
	if _, err := llm.Ask(ctx, s.Store, s.Store.AIConfigs(), bookID, llm.IdentifySystem,
		llm.IdentifyUser(fm.Title, fm.Author, fm.Publisher, fm.TitlePage, fm.CopyrightPage, fm.Opening), func(out string) error {
			var perr error
			got, perr = llm.ParseIdentity(out)
			return perr
		}); err != nil {
		return store.Identity{}, false // the AI did not answer: not kept, so it is asked again next time
	}
	if !got.Certain || !printedIn(front+" "+fm.Opening+" "+fm.Title, got.Title) {
		s.Store.SaveIdentity(bookID, "", "", isbn)
		return store.Identity{}, false
	}
	s.Store.SaveIdentity(bookID, got.Title, got.Author, isbn)
	return store.Identity{Title: got.Title, Author: got.Author, ISBN: isbn}, true
}

// nameFromPages renames a wrongly named book in Calibre from what its pages say; true when it did.
func (s *Server) nameFromPages(ctx context.Context, bookID int64) bool {
	ident, ok := s.identifyBook(ctx, bookID)
	if !ok {
		return false
	}
	b, err := s.Store.BookByID(bookID, nil)
	if err != nil {
		return false
	}
	if b.ISBN == "" && ident.ISBN != "" && s.Store.SetMissingISBN(b.ID, ident.ISBN) { // the copyright page's ISBN, for a book with none
		s.saveISBNInCalibre(ctx, b.ID, ident.ISBN)
	}
	if !looksOff(b.Title) && deepread.TitleFits(b.Title, ident.Title) {
		return false // already named right
	}
	calibreID := 0
	copies, _ := s.Store.BookCopies(bookID)
	for _, c := range copies {
		if c.Source == "calibre" && c.ExternalID != "" {
			fmt.Sscan(c.ExternalID, &calibreID)
			break
		}
	}
	if calibreID == 0 || s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		return false
	}
	fields := [][]any{{"title", ident.Title}}
	if authorUnknown(b.Author) && ident.Author != "" {
		fields = append(fields, []any{"authors", []string{ident.Author}})
	}
	if found, err := s.calibreClient().SetFields(ctx, calibreID, fields); err != nil || !found {
		return false
	}
	s.cleanupNote("Identified from the first pages", fmt.Sprintf("Renamed “%s” by %s to “%s” by %s, as its own title page prints it (Calibre #%d).", b.Title, b.Author, ident.Title, ident.Author, calibreID))
	return true
}

// decidePair says what two entries' own pages settle: "merge" (the same title and author), "differ" (clearly different titles)
// or "" (not settled: the AI is not asked to guess).
func decidePair(a, b store.Identity) string {
	if a.Title == "" || b.Title == "" {
		return ""
	}
	if deepread.SameTitleWords(a.Title, b.Title) && (authorUnknown(a.Author) || authorUnknown(b.Author) || deepread.AuthorFits(a.Author, b.Author)) {
		return "merge"
	}
	if !deepread.TitleFits(a.Title, b.Title) {
		return "differ"
	}
	return ""
}

// IdentifyLoop reads unsure books every six hours (a little after start), a few at a time.
func (s *Server) IdentifyLoop(ctx context.Context) {
	wait := 20 * time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = 6 * time.Hour
		if s.identifyOn() {
			s.identifyOnce(ctx, 15, 10)
		}
	}
}

func (s *Server) identifyOnce(ctx context.Context, books, pairs int) {
	named := 0
	for _, id := range s.Store.IdentifyCandidates(books) {
		if ctx.Err() != nil {
			return
		}
		if s.nameFromPages(ctx, id) {
			named++
		}
	}
	merged, differ, looked := 0, 0, 0
	list, _ := s.Store.SamePairs()
	for _, p := range list {
		if p.Confidence == "sure" || looked >= pairs || ctx.Err() != nil {
			continue
		}
		looked++
		ia, oka := s.identifyBook(ctx, p.A.ID)
		ib, okb := s.identifyBook(ctx, p.B.ID)
		if !oka || !okb {
			continue
		}
		switch decidePair(ia, ib) {
		case "merge":
			keep := int64(0)
			if p.Suggest == "a" {
				keep = p.A.ID
			} else if p.Suggest == "b" {
				keep = p.B.ID
			}
			if _, err := s.Store.MergeKeeping(p.A.ID, p.B.ID, keep); err == nil {
				merged++
				s.cleanupNote("Identified from the first pages", fmt.Sprintf("Joined “%s” and “%s”: both pages say “%s” by %s.", p.A.Title, p.B.Title, ia.Title, ia.Author))
			}
		case "differ":
			if s.Store.IgnoreSame(p.A.ID, p.B.ID) == nil {
				differ++
				s.cleanupNote("Identified from the first pages", fmt.Sprintf("“%s” and “%s” are different books: their pages say “%s” and “%s”.", p.A.Title, p.B.Title, ia.Title, ib.Title))
			}
		}
	}
	if named+merged+differ > 0 {
		s.Store.NotifyRoutine("identify", fmt.Sprintf("Read the first pages of unsure books: %d renamed, %d pairs joined, %d pairs marked as different.", named, merged, differ), "#/admin?tab=cleanup")
	}
}

// FixWrongFile is the Deep Scan's way out of "this file isn't the book it is filed under": the file's first pages are read (the same
// reading as for unsure titles, only when that is switched on). If they say it is the library's book the metadata was only odd; if
// they name another book with certainty it is renamed in Calibre to that; otherwise nothing is settled and the scan stops as before.
func (s *Server) FixWrongFile(ctx context.Context, b *store.Book) (string, string, deepread.FileVerdict) {
	if !s.identifyOn() {
		return "", "", deepread.FileUnsure
	}
	ident, ok := s.identifyBook(ctx, b.ID)
	if !ok {
		return "", "", deepread.FileUnsure
	}
	if !looksOff(b.Title) && deepread.TitleFits(b.Title, ident.Title) {
		return "", "", deepread.FileAgrees
	}
	if !s.nameFromPages(ctx, b.ID) {
		return "", "", deepread.FileUnsure
	}
	author := b.Author
	if authorUnknown(author) && ident.Author != "" {
		author = ident.Author
	}
	return ident.Title, author, deepread.FileIs
}
