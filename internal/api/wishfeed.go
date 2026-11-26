package api

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/push"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The wish feed: an RSS list of the books and audiobooks the family has asked for and not got yet, behind a secret link
// (like the bug link), so an admin can share it without a login. It shows only the title, the author and the format; never
// who asked, a cover, a rating or anything else. A wish leaves it as soon as the book is in a library.

var wishFeedLast atomic.Int64 // when the feed last looked for arrivals, so the open link cannot make the server do it over and over

func formatWord(f string) string {
	switch f {
	case "audio":
		return "audiobook"
	case "ebook":
		return "ebook"
	}
	return "book"
}

type rssItem struct {
	Title string `xml:"title"`
	GUID  string `xml:"guid"`
	Date  string `xml:"pubDate,omitempty"`
}

// handleWishFeed: GET /feed/wishes/{token}.
func (s *Server) handleWishFeed(w http.ResponseWriter, r *http.Request) {
	if !s.Store.WishFeedOK(chi.URLParam(r, "token")) {
		http.NotFound(w, r)
		return
	}
	var wishes []store.Wish
	if now := time.Now().Unix(); now-wishFeedLast.Load() > 60 {
		wishFeedLast.Store(now)
		wishes, _ = s.Store.OpenWishes()
	} else {
		wishes, _ = s.Store.OpenWishesQuick()
	}
	var items []rssItem
	for _, wi := range wishes {
		title := wi.Title
		if wi.Author != "" {
			title += " by " + wi.Author
		}
		it := rssItem{Title: title + " (" + formatWord(wi.Format) + ")", GUID: fmt.Sprintf("wish-%d", wi.ID)}
		if t, err := time.Parse("2006-01-02 15:04:05", wi.CreatedAt); err == nil {
			it.Date = t.UTC().Format(time.RFC1123Z)
		}
		items = append(items, it)
	}
	feed := struct {
		XMLName xml.Name  `xml:"rss"`
		Version string    `xml:"version,attr"`
		Title   string    `xml:"channel>title"`
		Desc    string    `xml:"channel>description"`
		Items   []rssItem `xml:"channel>item"`
	}{Version: "2.0", Title: "Books wanted", Desc: "Books and audiobooks the family has asked for and not got yet.", Items: items}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Robots-Tag", "noindex")
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(feed)
}

// handleAdminWishFeed: GET shows the link, POST turns it on or makes a new one (the old one stops working), DELETE turns it off.
func (s *Server) handleAdminWishFeed(w http.ResponseWriter, r *http.Request) {
	var err error
	switch r.Method {
	case http.MethodPost:
		_, err = s.Store.NewWishFeedToken()
	case http.MethodDelete:
		err = s.Store.ClearWishFeed()
	}
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	out := map[string]any{"enabled": false}
	if tok := s.Store.WishFeedToken(); tok != "" {
		out["enabled"], out["url"] = true, s.baseURL(r)+"/feed/wishes/"+tok
	}
	writeJSON(w, http.StatusOK, out)
}

// WishArrived is told once when a wished-for book arrives: the person who asked gets a phone notice, and the parents a bell
// notice (which goes to their phones too, with notifications on).
func (s *Server) WishArrived(wi store.Wish) {
	what := wi.Title
	if wi.Author != "" {
		what += " by " + wi.Author
	}
	word := formatWord(wi.Format)
	if s.Push != nil {
		s.Push.ToUser(wi.UserID, push.Message{Title: "📚 Your " + word + " is here", Body: what, URL: "/#/library", Tag: fmt.Sprintf("wish-%d", wi.ID)})
	}
	s.Store.Notify("info", "wishlist", fmt.Sprintf("“%s” (%s), asked for by %s, is in the library now.", what, word, wi.Username), "#/wishlist")
}

// WishLoop looks for arrivals every ten minutes, so people are told soon after a book is added.
func (s *Server) WishLoop(ctx context.Context) {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Store.MarkAcquired()
		}
	}
}
