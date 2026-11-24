package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/calibresrv"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Adding e-books by drag and drop for admins and the people an admin switched on (Readers → Allow adding e-books).
// The book goes straight into Calibre, then Hardcover is asked what it really is (by ISBN, then by title and
// author). A sure match fixes the title, series, missing author and ISBN in Calibre; an unsure one comes back with
// Hardcover's best guess for the person to confirm or correct (POST /api/upload/fix).

// maxUploadsPerDay is how many books a person who isn't an admin may add in a day.
const maxUploadsPerDay = 50

// POST /api/upload/ebook: one e-book file (multipart field "file").
func (s *Server) handleFamilyUpload(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	if !u.MayUpload() {
		writeErr(w, http.StatusForbidden, "an admin has to switch on adding e-books for you (Readers)")
		return
	}
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		writeErr(w, http.StatusBadRequest, "adding books isn't set up yet: ask an admin")
		return
	}
	if !u.IsAdmin() && s.Store.UploadsToday(u.ID) >= maxUploadsPerDay {
		writeErr(w, http.StatusTooManyRequests, "that's "+strconv.Itoa(maxUploadsPerDay)+" books today: try again tomorrow, or ask an admin")
		return
	}
	name, data, ok := readEbookUpload(w, r)
	if !ok {
		return
	}
	cal := s.calibreClient()
	id, err := cal.AddBook(r.Context(), name, data)
	if errors.Is(err, calibresrv.ErrDuplicate) {
		writeErr(w, http.StatusConflict, "“"+name+"” is already in the library")
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadGateway, "The library couldn't take “"+name+"”: "+err.Error())
		return
	}
	s.Store.NoteUpload(strconv.Itoa(id), u.ID)
	writeJSON(w, http.StatusOK, s.checkUpload(r.Context(), cal, id))
}

// POST /api/upload/fix {calibre_id, title, author}: the title and author for a book this person added.
func (s *Server) handleUploadFix(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	var body struct {
		CalibreID int    `json:"calibre_id"`
		Title     string `json:"title"`
		Author    string `json:"author"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	body.Title, body.Author = strings.TrimSpace(body.Title), strings.TrimSpace(body.Author)
	if !u.MayUpload() || (!u.IsAdmin() && !s.Store.UploadedRecently(strconv.Itoa(body.CalibreID), u.ID)) {
		writeErr(w, http.StatusForbidden, "you can fix the books you added in the last week")
		return
	}
	if body.Title == "" || len(body.Title) > 200 || len(body.Author) > 200 {
		writeErr(w, http.StatusBadRequest, "give a title (and the author) up to 200 characters")
		return
	}
	var authors []string
	for _, a := range strings.Split(body.Author, " & ") {
		if a = strings.TrimSpace(a); a != "" {
			authors = append(authors, a)
		}
	}
	fields := [][]any{{"title", body.Title}}
	if len(authors) > 0 {
		fields = append(fields, []any{"authors", authors})
	}
	cal := s.calibreClient()
	if found, err := cal.SetFields(r.Context(), body.CalibreID, fields); err != nil || !found {
		writeErr(w, http.StatusBadGateway, "The library couldn't save that: check the book is still there")
		return
	}
	writeJSON(w, http.StatusOK, s.checkUpload(r.Context(), cal, body.CalibreID))
}

// checkUpload asks Hardcover about a Calibre book that was just added and saves what is sure. The answer's status is
// "matched" (checked, and fixed where needed), "unsure" (with Hardcover's best guess, if any) or "unchecked"
// (Hardcover isn't connected or didn't answer: the book is in with the file's own details).
func (s *Server) checkUpload(ctx context.Context, cal *calibresrv.Client, id int) map[string]any {
	out := map[string]any{"calibre_id": id, "status": "unchecked"}
	det, err := cal.Details(ctx, []int{id})
	d, ok := det[id]
	if err != nil || !ok {
		return out
	}
	out["title"], out["authors"] = d.Title, d.Authors
	s.syncSoon(nil) // NovelCheck reads Calibre again so the book shows up
	token := s.Store.RatingsToken()
	if token == "" {
		return out
	}
	hc := &hardcover.Client{Token: token, URL: hardcoverURL}
	author := ""
	if len(d.Authors) > 0 && !strings.EqualFold(d.Authors[0], "unknown") {
		author = d.Authors[0]
	}
	var info hardcover.BookInfo
	found := false
	if isbn := d.Identifiers["isbn"]; isbn != "" {
		if m, err := hc.InfoByISBN(ctx, []string{isbn}); err == nil {
			for _, b := range m {
				info, found = b, true
			}
		}
	}
	if !found {
		var err error
		if info, found, err = hc.InfoByTitle(ctx, d.Title, author); err != nil {
			return out
		}
	}
	who := author // a file with no author is judged against the author Hardcover names
	if who == "" && len(info.Authors) > 0 {
		who = info.Authors[0]
	}
	if found && (info.Title == d.Title || hcTrueTitle(d.Title, who, info) != "") {
		out["fixed"] = s.applyUploadMatch(ctx, cal, id, d, info)
		out["status"] = "matched"
		return out
	}
	out["status"] = "unsure"
	if !found {
		var err error
		if info, found, err = hc.BestGuess(ctx, d.Title, author); err != nil {
			out["status"] = "unchecked"
			return out
		}
	}
	if found && len(info.Authors) > 0 {
		out["guess"] = map[string]string{"title": info.Title, "author": strings.Join(info.Authors, " & ")}
	}
	return out
}

// applyUploadMatch saves what Hardcover surely knows about the book in Calibre; it returns what changed.
func (s *Server) applyUploadMatch(ctx context.Context, cal *calibresrv.Client, id int, d calibresrv.Details, info hardcover.BookInfo) []string {
	var fields [][]any
	var said []string
	if info.Title != "" && info.Title != d.Title {
		fields, said = append(fields, []any{"title", info.Title}), append(said, "title")
	}
	if len(info.Authors) > 0 && (len(d.Authors) == 0 || strings.EqualFold(d.Authors[0], "unknown")) {
		fields, said = append(fields, []any{"authors", info.Authors}), append(said, "author")
	}
	if d.Series == "" && info.Series != "" {
		fields, said = append(fields, []any{"series", info.Series}), append(said, "series")
		if info.Position > 0 {
			fields = append(fields, []any{"series_index", info.Position})
		}
	}
	if isbn := firstNonEmpty(info.ISBN13, info.ISBN10); isbn != "" && d.Identifiers["isbn"] == "" {
		ids := map[string]string{"isbn": isbn}
		for k, v := range d.Identifiers {
			ids[k] = v
		}
		fields, said = append(fields, []any{"identifiers", ids}), append(said, "ISBN")
	}
	if len(fields) > 0 {
		if found, err := cal.SetFields(ctx, id, fields); err != nil || !found {
			return nil
		}
	}
	if d.Cover == "" && info.Image != "" { // a book with no cover gets Hardcover's
		if img, err := fetchCover(ctx, info.Image); err == nil && cal.SetCover(ctx, id, img) == nil {
			said = append(said, "cover")
		}
	}
	if len(said) > 0 {
		s.syncSoon(nil)
	}
	return said
}
