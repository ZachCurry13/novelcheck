package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/audiobookshelf"
)

// applyOldDoubtful applies the fixes that wait only because an older NovelCheck judged them doubtful: an ISBN that led to
// a different title ("Dark Tower II" for "The Drawing of the Three"). Now the same author makes that certain
// (absclean.go), and the stored fix says so: it has no new author, so the audiobook's author already holds Hardcover's.
// It applies them only while fixing by itself is on, and says how many.
func (s *Server) applyOldDoubtful(ctx context.Context, c *audiobookshelf.Client) int {
	if s.Store.Setting(keyABSCleanAuto) == "off" {
		return 0
	}
	list, err := s.Store.ABSFixes()
	if err != nil {
		return 0
	}
	n := 0
	for _, f := range list {
		var fix audiobookshelf.Fix
		if !strings.HasPrefix(f.Why, "The ISBN leads to") || !strings.HasSuffix(f.Why, "a different title") ||
			json.Unmarshal([]byte(f.Fix), &fix) != nil || fix.Author != "" || fix.Empty() {
			continue
		}
		if err := c.ApplyFix(ctx, f.ItemID, fix); err != nil {
			if errors.Is(err, audiobookshelf.ErrNoEditRights) {
				return n
			}
			log.Printf("audiobookshelf old waiting fix %q: %v", f.Title, err)
			continue
		}
		s.Store.DropABSFix(f.ItemID, false)
		n++
	}
	if n > 0 {
		res, err := audiobookshelf.Sync(ctx, s.Store, c)
		audiobookshelf.Record(s.Store, res, err)
	}
	return n
}
