package api

import (
	"context"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Duplicates in Calibre: what Calibre itself says about each copy (its own title, authors, series, ISBN and
// the start of its description), so two copies can be told apart before one is removed.

type dupDetail struct {
	Title   string `json:"title"`
	Authors string `json:"authors"`
	Series  string `json:"series"`
	IDs     string `json:"ids"`   // ISBN and other identifiers
	Tags    string `json:"tags"`  // the first few
	About   string `json:"about"` // the start of the description
	Open    string `json:"open"`  // the book's page in Calibre-Web, when its address is saved
}

var tagRE = regexp.MustCompile(`<[^>]*>`)

// handleDuplicateDetails: GET /api/admin/calibre/duplicates/details?ids=12,34.
func (s *Server) handleDuplicateDetails(w http.ResponseWriter, r *http.Request) {
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		writeErr(w, http.StatusBadRequest, "connect the Calibre Content server first: the details come from it")
		return
	}
	var ids []int
	for _, p := range strings.Split(r.URL.Query().Get("ids"), ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && n > 0 && len(ids) < 20 {
			ids = append(ids, n)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	have, err := s.calibreClient().Details(ctx, ids)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	web := strings.TrimRight(strings.TrimSpace(s.Store.Setting(store.KeyCalibreWebURL)), "/")
	out := map[string]dupDetail{}
	for id, d := range have {
		det := dupDetail{Title: d.Title, Authors: strings.Join(d.Authors, " & "), Series: d.Series}
		if d.Series != "" && d.SeriesIndex > 0 {
			det.Series += " #" + strconv.FormatFloat(d.SeriesIndex, 'f', -1, 64)
		}
		keys := make([]string, 0, len(d.Identifiers))
		for k := range d.Identifiers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				det.IDs += " · "
			}
			det.IDs += k + " " + d.Identifiers[k]
		}
		det.Tags = strings.Join(d.Tags[:min(len(d.Tags), 6)], ", ")
		about := strings.Join(strings.Fields(tagRE.ReplaceAllString(d.Comments, " ")), " ")
		if r := []rune(about); len(r) > 280 {
			about = string(r[:280]) + "…"
		}
		det.About = about
		if web != "" {
			det.Open = web + "/book/" + strconv.Itoa(id)
		}
		out[strconv.Itoa(id)] = det
	}
	writeJSON(w, http.StatusOK, map[string]any{"details": out})
}
