package hardcover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Writing to Hardcover (opt-in): the calls the Hardcover plugin for KOReader
// makes: insert_user_book, update_user_book, insert_user_book_read and
// update_user_book_read. Books are found by ISBN only, so a wrong match can't
// change the wrong book. Everything built into the query is a number, an
// ISBN made of digits, or a date, never free text.

// Edition is a book's edition on Hardcover.
type Edition struct {
	ID, BookID, Pages int
}

// UserBook is a person's entry for a book, with the newest read.
type UserBook struct {
	ID       int // 0 = not on their shelves
	Status   int
	ReadID   int // 0 = no read yet
	Progress int
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == 'X' || r == 'x' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FindEdition looks a book up by ISBN (10 or 13); ok is false if Hardcover doesn't have it.
func (c *Client) FindEdition(ctx context.Context, isbn string) (Edition, bool, error) {
	isbn = digits(isbn)
	if len(isbn) != 10 && len(isbn) != 13 {
		return Edition{}, false, nil
	}
	var d struct {
		Editions []struct {
			ID     int `json:"id"`
			BookID int `json:"book_id"`
			Pages  int `json:"pages"`
		} `json:"editions"`
	}
	q := fmt.Sprintf(`query { editions(where: {_or: [{isbn_13: {_eq: "%s"}}, {isbn_10: {_eq: "%s"}}]}, limit: 1) { id book_id pages } }`, isbn, isbn)
	if err := c.do(ctx, q, &d); err != nil {
		return Edition{}, false, err
	}
	if len(d.Editions) == 0 {
		return Edition{}, false, nil
	}
	e := d.Editions[0]
	return Edition{ID: e.ID, BookID: e.BookID, Pages: e.Pages}, true, nil
}

// FindUserBook reads the person's entry for a Hardcover book.
func (c *Client) FindUserBook(ctx context.Context, bookID int) (UserBook, error) {
	var d struct {
		Me json.RawMessage `json:"me"`
	}
	q := fmt.Sprintf(`query { me { user_books(where: {book_id: {_eq: %d}}, limit: 1) { id status_id user_book_reads(order_by: {id: desc}, limit: 1) { id progress_pages } } } }`, bookID)
	if err := c.do(ctx, q, &d); err != nil {
		return UserBook{}, err
	}
	u, err := me[struct {
		UserBooks []struct {
			ID     int `json:"id"`
			Status int `json:"status_id"`
			Reads  []struct {
				ID       int `json:"id"`
				Progress int `json:"progress_pages"`
			} `json:"user_book_reads"`
		} `json:"user_books"`
	}](d.Me)
	if err != nil || len(u.UserBooks) == 0 {
		return UserBook{}, err
	}
	ub := UserBook{ID: u.UserBooks[0].ID, Status: u.UserBooks[0].Status}
	if len(u.UserBooks[0].Reads) > 0 {
		ub.ReadID, ub.Progress = u.UserBooks[0].Reads[0].ID, u.UserBooks[0].Reads[0].Progress
	}
	return ub, nil
}

func (c *Client) mutate(ctx context.Context, q string) error {
	var out json.RawMessage
	return c.do(ctx, q, &out)
}

var errNoUserBook = errors.New("Hardcover didn't add the book to your shelves")

// Push brings the person's Hardcover entry in line: on their shelves with
// the status (2 reading, 3 read) and, when pages is known, their progress.
// Finishing marks the read finished today. It never lowers progress or a
// status already further along.
func (c *Client) Push(ctx context.Context, ed Edition, status, pages int, today time.Time) error {
	ub, err := c.FindUserBook(ctx, ed.BookID)
	if err != nil {
		return err
	}
	if ub.ID == 0 {
		q := fmt.Sprintf(`mutation { insert_user_book(object: {book_id: %d, edition_id: %d, status_id: %d}) { __typename } }`, ed.BookID, ed.ID, status)
		if err := c.mutate(ctx, q); err != nil {
			return err
		}
		if ub, err = c.FindUserBook(ctx, ed.BookID); err != nil {
			return err
		}
		if ub.ID == 0 {
			return errNoUserBook
		}
	} else if ub.Status < status && ub.Status != DNF && ub.Status != Paused {
		if err := c.mutate(ctx, fmt.Sprintf(`mutation { update_user_book(id: %d, object: {status_id: %d}) { __typename } }`, ub.ID, status)); err != nil {
			return err
		}
	}
	if status == Read && pages == 0 {
		pages = ed.Pages
	}
	if pages <= 0 || pages <= ub.Progress && status != Read {
		return nil
	}
	date := today.Format("2006-01-02")
	switch {
	case ub.ReadID == 0:
		return c.mutate(ctx, fmt.Sprintf(`mutation { insert_user_book_read(user_book_id: %d, user_book_read: {progress_pages: %d, edition_id: %d, started_at: "%s"}) { __typename } }`, ub.ID, pages, ed.ID, date))
	case status == Read:
		q := fmt.Sprintf(`mutation { update_user_book_read(id: %d, object: {progress_pages: %d, finished_at: "%s"}) { __typename } }`, ub.ReadID, pages, date)
		if err := c.mutate(ctx, q); err != nil { // an older server may not take finished_at
			return c.mutate(ctx, fmt.Sprintf(`mutation { update_user_book_read(id: %d, object: {progress_pages: %d}) { __typename } }`, ub.ReadID, pages))
		}
		return nil
	default:
		return c.mutate(ctx, fmt.Sprintf(`mutation { update_user_book_read(id: %d, object: {progress_pages: %d}) { __typename } }`, ub.ReadID, pages))
	}
}
