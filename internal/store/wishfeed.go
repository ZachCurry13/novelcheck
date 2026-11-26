package store

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
)

// KeyWishFeed is the secret in the wish feed's link (/feed/wishes/<secret>). Empty means the feed is off.
const KeyWishFeed = "wish_feed_token"

// WishFeedToken is the current secret ("" when the feed is off).
func (s *Store) WishFeedToken() string { return s.Setting(KeyWishFeed) }

// NewWishFeedToken turns the feed on with a fresh secret (an old link stops working).
func (s *Store) NewWishFeedToken() (string, error) {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	return tok, s.SetSetting(KeyWishFeed, tok)
}

// ClearWishFeed turns the feed off.
func (s *Store) ClearWishFeed() error { return s.SetSetting(KeyWishFeed, "") }

// WishFeedOK reports whether tok is the feed's secret.
func (s *Store) WishFeedOK(tok string) bool {
	cur := s.WishFeedToken()
	return cur != "" && subtle.ConstantTimeCompare([]byte(cur), []byte(tok)) == 1
}

// OpenWishes lists what is still wanted, oldest first, for the feed: no names of who asked.
func (s *Store) OpenWishes() ([]Wish, error) {
	s.MarkAcquired()
	return s.OpenWishesQuick()
}

// OpenWishesQuick is OpenWishes without looking for arrivals first.
func (s *Store) OpenWishesQuick() ([]Wish, error) {
	out := []Wish{}
	err := s.DB.Select(&out, `SELECT `+wishCols+` FROM wishlist w JOIN books b ON b.id = w.book_id
		WHERE w.status IN ('wanted', 'approved') ORDER BY w.id LIMIT 200`)
	return out, err
}
