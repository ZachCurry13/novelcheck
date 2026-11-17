package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/zachcurry13/novelcheck/internal/audiobookshelf"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The Audiobookshelf cleanup: every audiobook is matched against Hardcover (the source of truth), by ISBN, then
// by Audible ASIN, then by exact title and author. A certain match fixes the audiobook in Audiobookshelf by
// itself: title, author and series as Hardcover has them, and what is missing (ISBN, ASIN, narrator, genres,
// description, cover). A match that needed a typo fixed in the title, or an ISBN that leads to a different
// title, waits for a parent (Library cleanup → Audiobookshelf). Each audiobook is looked at again after a month.

const keyABSCleanAuto = "abs_cleanup_auto" // "off" = every fix waits for a parent
const keyABSCleanResult = "abs_cleanup_result"

type absMatch struct {
	it      store.AudioItem
	info    hardcover.BookInfo
	certain bool
	why     string
}

// CleanAudiobooks looks at up to limit audiobooks not looked at this month and says what it did.
func (s *Server) CleanAudiobooks(ctx context.Context, limit int) (string, error) {
	c, ok := audiobookshelf.FromSettings(s.Store)
	token := s.Store.RatingsToken()
	if !ok || token == "" {
		return "", nil
	}
	hc := &hardcover.Client{Token: token, URL: hardcoverURL}
	items, err := s.absToCheck(ctx, c, limit)
	if err != nil || len(items) == 0 {
		return "", err
	}
	matches, unknown := s.matchAudiobooks(ctx, hc, items)
	auto := s.Store.Setting(keyABSCleanAuto) != "off"
	fixed, waiting := 0, 0
	for _, m := range matches {
		f := absFix(m.it, m.info)
		if (m.it.ASIN == "" || m.it.Narrator == "") && m.info.ID > 0 {
			if more, err := hc.MoreAbout(ctx, m.info.ID, 0); err == nil { // the audiobook edition: ASIN and narrator
				if m.it.ASIN == "" {
					f.ASIN = more.ASIN
				}
				if m.it.Narrator == "" {
					f.Narrators = more.Narrators
				}
			}
		}
		switch {
		case f.Empty():
		case m.certain && auto:
			if err := c.ApplyFix(ctx, m.it.ID, f); err != nil {
				if errors.Is(err, audiobookshelf.ErrNoEditRights) {
					return "", err
				}
				log.Printf("audiobookshelf cleanup %q: %v", m.it.Title, err)
				continue
			}
			fixed++
		default:
			why := m.why
			if why == "" {
				why = "Fixing by itself is turned off"
			}
			raw, _ := json.Marshal(f)
			s.Store.SaveABSFix(store.ABSFix{ItemID: m.it.ID, Title: m.it.Title, Author: m.it.Author, Fix: string(raw), Why: why})
			waiting++
		}
		s.Store.MarkABSChecked(m.it.ID)
	}
	for _, it := range unknown {
		s.Store.MarkABSChecked(it.ID)
	}
	if fixed > 0 { // NovelCheck's own copy of the details
		res, err := audiobookshelf.Sync(ctx, s.Store, c)
		audiobookshelf.Record(s.Store, res, err)
	}
	msg := fmt.Sprintf("%s: looked at %d audiobooks, fixed %d, %d waiting for you, %d Hardcover doesn't know",
		time.Now().Format("2 Jan 15:04"), len(items), fixed, waiting, len(unknown))
	_ = s.Store.SetSetting(keyABSCleanResult, msg)
	return msg, nil
}

// absToCheck reads the audiobooks not looked at in the last 30 days, up to limit.
func (s *Server) absToCheck(ctx context.Context, c *audiobookshelf.Client, limit int) ([]store.AudioItem, error) {
	libs, err := c.Libraries(ctx)
	if err != nil {
		return nil, err
	}
	done := s.Store.ABSCheckedSince(time.Now().Add(-30 * 24 * time.Hour))
	var out []store.AudioItem
	for _, l := range libs {
		if l.MediaType != "book" {
			continue
		}
		for page := 0; len(out) < limit; page++ {
			items, total, err := c.Items(ctx, l.ID, page, 100)
			if err != nil {
				return nil, err
			}
			for _, it := range items {
				if !done[it.ID] && len(out) < limit {
					out = append(out, it)
				}
			}
			if (page+1)*100 >= total {
				break
			}
		}
	}
	return out, nil
}

// matchAudiobooks finds each audiobook on Hardcover; unknown are the ones it doesn't know.
func (s *Server) matchAudiobooks(ctx context.Context, hc *hardcover.Client, items []store.AudioItem) (out []absMatch, unknown []store.AudioItem) {
	var rest []store.AudioItem
	for i := 0; i < len(items); i += 25 {
		chunk := items[i:min(i+25, len(items))]
		var isbns, asins []string
		for _, it := range chunk {
			isbns, asins = append(isbns, digitsOnly(it.ISBN)), append(asins, strings.ToUpper(it.ASIN))
		}
		byISBN, _ := hc.InfoByISBN(ctx, isbns)
		byASIN, _ := hc.InfoByASIN(ctx, asins)
		for _, it := range chunk {
			info, ok := byISBN[digitsOnly(it.ISBN)]
			how := "ISBN"
			if !ok || it.ISBN == "" {
				info, ok, how = byASIN[strings.ToUpper(it.ASIN)], it.ASIN != "", "ASIN"
				ok = ok && info.ID > 0
			}
			if !ok {
				rest = append(rest, it)
				continue
			}
			// An exact ISBN or ASIN by the same author is the same book, whatever the edition calls it (a film
			// tie-in title, a UK title, "Dark Tower II" for "The Drawing of the Three"). Only a different title
			// by a different author is doubtful: the number may be wrong on the audiobook.
			m := absMatch{it: it, info: info, certain: absSameTitle(it.Title, info.Title) || sameAuthor(it.Author, info.Authors)}
			if !m.certain {
				m.why = fmt.Sprintf("The %s leads to “%s” by %s on Hardcover, a different book", how, info.Title, strings.Join(info.Authors, " & "))
			}
			out = append(out, m)
		}
		time.Sleep(time.Second)
	}
	for _, it := range rest {
		if ctx.Err() != nil {
			break
		}
		if info, ok, err := hc.InfoByTitle(ctx, it.Title, it.Author); err == nil && ok {
			out = append(out, absMatch{it: it, info: info, certain: true})
		} else if t := s.closestTitle(ctx, it.Title, it.Author); t != "" {
			if info, ok, err := hc.InfoByTitle(ctx, t, it.Author); err == nil && ok {
				out = append(out, absMatch{it: it, info: info, why: fmt.Sprintf("Found as “%s” (the title looked misspelled)", t)})
			} else {
				unknown = append(unknown, it)
			}
		} else {
			unknown = append(unknown, it)
		}
		time.Sleep(time.Second)
	}
	return out, unknown
}

// sameAuthor: one of Hardcover's authors is among the audiobook's (by last name, "King" in "Stephen King").
func sameAuthor(item string, hcAuthors []string) bool {
	norm := func(s string) string {
		return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && r != '\'' }), " ")
	}
	have := " " + norm(item) + " "
	for _, a := range hcAuthors {
		f := strings.Fields(norm(strings.ReplaceAll(a, ",", " ")))
		if len(f) > 0 && strings.Contains(have, " "+f[len(f)-1]+" ") {
			return true
		}
	}
	return false
}

// absSameTitle: the same title, or one holds the other ("Dune" and "Dune: Deluxe Edition").
func absSameTitle(a, b string) bool {
	x, y := store.NormKey(a, ""), store.NormKey(b, "")
	return x != "|" && y != "|" && (x == y || strings.HasPrefix(x, strings.TrimSuffix(y, "|")) || strings.HasPrefix(y, strings.TrimSuffix(x, "|")))
}

// absFix is what to change so the audiobook matches Hardcover.
func absFix(it store.AudioItem, b hardcover.BookInfo) audiobookshelf.Fix {
	var f audiobookshelf.Fix
	if b.Title != "" && strings.TrimSpace(it.Title) != b.Title {
		f.Title = b.Title
	}
	if len(b.Authors) > 0 && !strings.Contains(strings.ToLower(it.Author), strings.ToLower(b.Authors[0])) {
		f.Author = b.Authors[0]
	}
	if b.Series != "" && (!strings.EqualFold(it.Series, b.Series) || b.Position > 0 && it.SeriesIndex != b.Position) {
		f.Series = b.Series
		if b.Position > 0 {
			f.Sequence = strconv.FormatFloat(b.Position, 'f', -1, 64)
		}
	}
	if it.ISBN == "" {
		f.ISBN = firstNonEmpty(b.ISBN13, b.ISBN10)
	}
	if len(it.Genres) == 0 && len(b.Tags) > 0 {
		f.Genres = b.Tags
	}
	if it.Description == "" {
		f.Description = b.Description
	}
	if !it.HasCover && strings.HasPrefix(b.Image, "https://") {
		f.Cover = b.Image
	}
	return f
}
