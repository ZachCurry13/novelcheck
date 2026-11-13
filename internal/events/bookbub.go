package events

// "BookBub free today": the free books in the newest BookBub email, read
// from the family's mailbox (read-only) and named by the AI, since the
// email's links go through BookBub rather than straight to Amazon.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/mailbox"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Asker sends one question to the AI and hands its answer to parse.
type Asker func(ctx context.Context, system, user string, parse func(string) error) error

const (
	bookBubFrom   = "bookbub.com"
	bookBubName   = "BookBub free today"
	bookBubSource = "https://www.bookbub.com/ebook-deals/free-ebooks"
	maxEmailText  = 16000
)

var (
	styleRe   = regexp.MustCompile(`(?is)<(style|script|head)\b.*?</(style|script|head)>`)
	urlTextRe = regexp.MustCompile(`(?i)https?://\S+|\S+@\S+\.\S+`)
)

const bookBubSystem = `You read a BookBub deals email. List every book the email offers for FREE ($0.00, "free"), not the discounted ones.
Give the title as printed (without "Book 1 of …" or series notes) and the author.
Answer only with JSON: {"books":[{"title":"…","author":"…"}]}. If no book is free, answer {"books":[]}.`

// bookBub reads the newest BookBub email once and makes its free books the
// standing event's list.
func bookBub(ctx context.Context, st *store.Store, ask Asker, enqueue func(bool, ...int64), now time.Time) error {
	box := mailbox.Box{Host: st.Setting(store.KeyEventsMailHost), User: st.Setting(store.KeyEventsMailUser), Password: st.Setting(store.KeyEventsMailPassword)}
	if box.User == "" || box.Password == "" {
		return nil // not set up: nothing to do
	}
	mctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	msg, found, err := mailbox.Latest(mctx, box, bookBubFrom, now.Add(-48*time.Hour))
	if err != nil {
		return err
	}
	if !found || (msg.ID != "" && msg.ID == st.Setting(store.KeyEventsBookBubSeen)) {
		return nil
	}
	books, err := emailBooks(ctx, ask, msg)
	if err != nil {
		return err
	}
	if msg.ID != "" {
		_ = st.SetSetting(store.KeyEventsBookBubSeen, msg.ID)
	}
	if len(books) == 0 {
		return nil
	}
	id, archived, found := st.EventBySource(bookBubSource)
	if archived {
		return nil
	}
	if !found {
		if id, _, err = st.CreateEvent(bookBubName, bookBubSource, byApp, "", nil); err != nil {
			return err
		}
		_ = st.PinEventOn(id)
	}
	unrated, err := st.ReplaceEventBooks(id, books)
	if len(unrated) > 0 {
		enqueue(false, unrated...)
	}
	st.NotifyRoutine("events", fmt.Sprintf("BookBub: %d free books today are in Events.", len(books)), fmt.Sprintf("#/library?event=%d", id))
	return err
}

// emailBooks are the free books in an email: its Amazon links when it has
// enough of them, otherwise the AI's reading of its words (links and email
// addresses left out).
func emailBooks(ctx context.Context, ask Asker, msg mailbox.Message) ([]store.EventEntry, error) {
	if bs := Parse(msg.HTML, ""); len(bs) >= 3 {
		return bs, nil
	}
	words := msg.Text
	if strings.TrimSpace(msg.HTML) != "" {
		words = text(strings.NewReplacer("</p>", "\n", "<br>", "\n", "</tr>", "\n", "</td>", " ").Replace(styleRe.ReplaceAllString(msg.HTML, " ")))
	}
	words = strings.TrimSpace(urlTextRe.ReplaceAllString(words, ""))
	if words == "" {
		return nil, nil
	}
	if r := []rune(words); len(r) > maxEmailText {
		words = string(r[:maxEmailText])
	}
	if ask == nil {
		return nil, errors.New("no AI to read the email")
	}
	var out []store.EventEntry
	err := ask(ctx, bookBubSystem, "Subject: "+msg.Subject+"\n\n"+words, func(answer string) error {
		var got struct {
			Books []struct{ Title, Author string } `json:"books"`
		}
		i, j := strings.Index(answer, "{"), strings.LastIndex(answer, "}")
		if i < 0 || j < i {
			return errors.New("no JSON in the answer")
		}
		if err := json.Unmarshal([]byte(answer[i:j+1]), &got); err != nil {
			return err
		}
		out = out[:0]
		for _, b := range got.Books {
			t, a := strings.TrimSpace(b.Title), strings.TrimSpace(b.Author)
			if t == "" || len(out) >= MaxBooks {
				continue
			}
			q := url.QueryEscape(strings.TrimSpace(t + " " + a))
			out = append(out, store.EventEntry{Title: t, Author: a, Link: "https://www.amazon.com/s?k=" + q + "&i=digital-text"})
		}
		return nil
	})
	return out, err
}
