package events

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/backend/memory"
	imapclient "github.com/emersion/go-imap/client"
	"github.com/emersion/go-imap/server"

	"github.com/zachcurry13/novelcheck/internal/mailbox"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// bookBubEmail is a daily deals email: HTML whose links go through BookBub.
func bookBubEmail(id string) string {
	return "From: BookBub <deals@e.bookbub.com>\r\nTo: reader@example.com\r\nSubject: Today's free books\r\n" +
		"Date: Fri, 13 Nov 2026 09:00:00 +0000\r\nMessage-ID: <" + id + "@bookbub.com>\r\n" +
		"MIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=\"b1\"\r\n\r\n" +
		"--b1\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nPlain words.\r\n" +
		"--b1\r\nContent-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n" +
		"<html><head><style>p{color:red}</style></head><body><p>Hi reader@example.com</p>" +
		"<table><tr><td><a href=3D\"https://www.bookbub.com/l/abc?u=3D1\">The Quiet Lake</a> by Ann Writer <b>FREE</b></td></tr>" +
		"<tr><td><a href=3D\"https://www.bookbub.com/l/def\">Big Sale Book</a> by Bo Seller $1.99</td></tr></table></body></html>\r\n" +
		"--b1--\r\n"
}

// fakeMailbox serves an IMAP inbox with the given emails (the test library's
// account is "username" / "password").
func fakeMailbox(t *testing.T, emails ...string) string {
	be := memory.New()
	u, _ := be.Login(nil, "username", "password")
	inbox, _ := u.GetMailbox("INBOX")
	for _, e := range emails {
		if err := inbox.(*memory.Mailbox).CreateMessage(nil, time.Now(), bytes.NewReader([]byte(e))); err != nil {
			t.Fatal(err)
		}
	}
	s := server.New(be)
	s.AllowInsecureAuth = true
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = s.Serve(l) }()
	old := mailbox.Dial
	mailbox.Dial = func(host string) (*imapclient.Client, error) { return imapclient.Dial(host) } // no TLS on this computer
	t.Cleanup(func() { mailbox.Dial = old; _ = s.Close() })
	return l.Addr().String()
}

func TestBookBubEmailBecomesAnEvent(t *testing.T) {
	st := testStore(t)
	host := fakeMailbox(t, bookBubEmail("one"))
	_ = st.SetSetting(store.KeyEventsMailHost, host)
	_ = st.SetSetting(store.KeyEventsMailUser, "username")
	_ = st.SetSetting(store.KeyEventsMailPassword, "password")
	var asked []string
	ask := func(_ context.Context, system, user string, parse func(string) error) error {
		asked = append(asked, user)
		return parse("Here you go:\n```json\n" + `{"books":[{"title":"The Quiet Lake","author":"Ann Writer"}]}` + "\n```")
	}
	var queued []int64
	enqueue := func(_ bool, ids ...int64) { queued = append(queued, ids...) }
	now := time.Now()
	for range 2 { // the same email is read once
		if err := bookBub(context.Background(), st, ask, enqueue, now); err != nil {
			t.Fatal(err)
		}
	}
	if len(asked) != 1 {
		t.Fatalf("asked the AI %d times", len(asked))
	}
	q := asked[0]
	if !strings.Contains(q, "The Quiet Lake") || !strings.Contains(q, "Subject: Today's free books") ||
		strings.Contains(q, "bookbub.com/l") || strings.Contains(q, "reader@example.com") || strings.Contains(q, "color:red") {
		t.Fatalf("what the AI was sent:\n%s", q)
	}
	evs, _ := st.ListEvents(&store.User{Role: "admin"})
	if len(evs) != 1 || evs[0].Name != "BookBub free today" || evs[0].Books != 1 || !evs[0].Pinned || len(queued) != 1 {
		t.Fatalf("events %+v, queued %v", evs, queued)
	}
}

func TestBookBubWithoutAMailboxDoesNothing(t *testing.T) {
	st := testStore(t)
	err := bookBub(context.Background(), st, func(context.Context, string, string, func(string) error) error {
		return errors.New("should not be asked")
	}, func(bool, ...int64) {}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
}

func TestAWrongPasswordIsSaidPlainly(t *testing.T) {
	st := testStore(t)
	host := fakeMailbox(t)
	_ = st.SetSetting(store.KeyEventsMailHost, host)
	_ = st.SetSetting(store.KeyEventsMailUser, "username")
	_ = st.SetSetting(store.KeyEventsMailPassword, "wrong")
	err := bookBub(context.Background(), st, nil, func(bool, ...int64) {}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "app password") {
		t.Fatalf("err: %v", err)
	}
}
