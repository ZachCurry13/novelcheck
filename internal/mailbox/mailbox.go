// Package mailbox reads the newest email from one sender (BookBub's daily
// free books) over IMAP, read-only: nothing is marked read, moved or deleted.
package mailbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	_ "github.com/emersion/go-message/charset" // emails in other character sets
	"github.com/emersion/go-message/mail"
)

// Message is one email's parts that matter here.
type Message struct {
	ID, Subject string
	Date        time.Time
	HTML, Text  string
}

// Box is where to sign in: "imap.gmail.com:993", the address and an app password.
type Box struct{ Host, User, Password string }

// Dial is how a connection is made (a test serves its own mailbox).
var Dial = func(host string) (*client.Client, error) {
	return client.DialWithDialerTLS(&net.Dialer{Timeout: 20 * time.Second}, host, nil)
}

// Latest is the newest email from a sender (part of its address, such as
// "bookbub.com") received since a time, looked for in the inbox and then in
// Gmail's All Mail (a filter may skip the inbox). found is false when none.
func Latest(ctx context.Context, b Box, from string, since time.Time) (msg Message, found bool, err error) {
	if b.Host == "" || b.User == "" || b.Password == "" {
		return msg, false, errors.New("the mailbox isn't set up")
	}
	c, err := Dial(b.Host)
	if err != nil {
		return msg, false, fmt.Errorf("couldn't reach %s: %w", b.Host, err)
	}
	defer c.Logout()
	if dl, ok := ctx.Deadline(); ok {
		c.Timeout = time.Until(dl)
	} else {
		c.Timeout = time.Minute
	}
	if err := c.Login(b.User, b.Password); err != nil {
		return msg, false, fmt.Errorf("the mailbox refused the sign-in (for Gmail use an app password): %w", err)
	}
	for _, folder := range []string{"INBOX", "[Gmail]/All Mail"} {
		if _, err := c.Select(folder, true); err != nil { // read-only
			continue
		}
		crit := imap.NewSearchCriteria()
		crit.Header.Add("From", from)
		crit.Since = since
		uids, err := c.UidSearch(crit)
		if err != nil || len(uids) == 0 {
			continue
		}
		newest := uids[0]
		for _, u := range uids {
			newest = max(newest, u)
		}
		return fetch(c, newest)
	}
	return msg, false, nil
}

func fetch(c *client.Client, uid uint32) (Message, bool, error) {
	set := new(imap.SeqSet)
	set.AddNum(uid)
	section := &imap.BodySectionName{Peek: true}
	ch := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() { done <- c.UidFetch(set, []imap.FetchItem{section.FetchItem()}, ch) }()
	var raw io.Reader
	for m := range ch {
		raw = m.GetBody(section)
	}
	if err := <-done; err != nil {
		return Message{}, false, err
	}
	if raw == nil {
		return Message{}, false, nil
	}
	return parse(raw)
}

// parse reads an email: its id, subject, date, and HTML and plain text.
func parse(raw io.Reader) (Message, bool, error) {
	r, err := mail.CreateReader(raw)
	if err != nil {
		return Message{}, false, err
	}
	var m Message
	m.ID = strings.Trim(r.Header.Get("Message-Id"), "<> ")
	m.Subject, _ = r.Header.Subject()
	m.Date, _ = r.Header.Date()
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return m, true, nil // keep what was read
		}
		h, ok := p.Header.(*mail.InlineHeader)
		if !ok {
			continue // attachments
		}
		t, _, _ := h.ContentType()
		body, _ := io.ReadAll(io.LimitReader(p.Body, 4<<20))
		switch {
		case t == "text/html" && m.HTML == "":
			m.HTML = string(body)
		case t == "text/plain" && m.Text == "":
			m.Text = string(body)
		}
	}
	return m, true, nil
}
