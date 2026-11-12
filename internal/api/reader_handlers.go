package api

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/delivery"
	"github.com/zachcurry13/novelcheck/internal/kosync"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleReaderPosition: GET /api/books/{id}/position is where you are in the book in NovelCheck's reader, and
// where KOReader's progress sync last put you (so the reader can offer to jump there); PUT {"cfi", "fraction"}
// saves your place.
func (s *Server) handleReaderPosition(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	b, err := s.Store.BookByID(id, u) // kids' rules apply
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if r.Method == http.MethodPut {
		var body struct {
			CFI      string  `json:"cfi"`
			Fraction float64 `json:"fraction"`
			XPointer string  `json:"xpointer"` // KOReader's name for the same place (see readerXPointer in js/reader.js)
		}
		if !readJSON(w, r, &body, 4<<10) {
			return
		}
		if len(body.CFI) > 2000 || body.Fraction < 0 || body.Fraction > 1 || !strings.HasPrefix(body.CFI, "epubcfi(") && body.CFI != "" {
			writeErr(w, http.StatusBadRequest, "not a place in a book")
			return
		}
		if err := s.Store.SaveReaderPosition(u.ID, id, body.CFI, body.Fraction); err != nil {
			writeStoreErr(w, err)
			return
		}
		synced := s.syncReaderToKOReader(u.ID, id, body.XPointer, body.Fraction)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "koreader": synced})
		return
	}
	out := map[string]any{"title": b.Title, "author": b.Author}
	if p, ok := s.Store.ReaderPositionOf(u.ID, id); ok {
		out["here"] = p
	}
	if k, ok := s.Store.KOReaderPositionOf(u.ID, id); ok {
		out["koreader"] = map[string]any{"fraction": k.Percentage, "device": k.Device, "at": k.Timestamp}
	}
	writeJSON(w, http.StatusOK, out)
}

// koXPointer is the shape of a KOReader (crengine) place in an EPUB: the book part, then the path of elements
// inside it, for example /body/DocFragment[12]/body/div/p[7].
var koXPointer = regexp.MustCompile(`^/body/DocFragment\[\d{1,5}\](/[a-z][a-z0-9]{0,15}(\[\d{1,5}\])?){1,40}$`)

// syncReaderToKOReader writes your place in NovelCheck's reader as KOReader progress for the very file the
// reader showed (KOReader names a book by a fingerprint of its file), so when you open that book on your
// e-reader, KOReader's progress sync takes you there. It says whether it wrote anything: nothing is written
// without a valid place or when the file can't be fingerprinted.
func (s *Server) syncReaderToKOReader(userID, bookID int64, xp string, fraction float64) bool {
	if xp == "" || len(xp) > 600 || !koXPointer.MatchString(xp) {
		return false
	}
	copies, err := s.Store.BookCopies(bookID)
	if err != nil {
		return false
	}
	best, ok := delivery.BestFile(copies, false)
	if !ok || !s.insideCalibre(best.Path) || strings.ToLower(best.Format) != "epub" {
		return false // the place is only meaningful in the same EPUB file
	}
	// Worked out from the file each time (it reads about 12 KB): Calibre rewrites a file after a conversion or a
	// metadata edit, and a fingerprint kept from before would send the place to a name KOReader no longer uses.
	doc, err := kosync.PartialMD5(best.Path)
	if err != nil || doc == "" {
		return false
	}
	if s.Store.FingerprintOf(best.Path) != doc {
		s.Store.MarkFingerprint(best.Path, doc, bookID)
	}
	err = s.Store.SaveKosync(userID, store.KosyncProgress{Document: doc, Progress: xp, Percentage: fraction,
		Device: "NovelCheck reader", DeviceID: store.ReaderDeviceID, Timestamp: time.Now().Unix()})
	return err == nil
}
