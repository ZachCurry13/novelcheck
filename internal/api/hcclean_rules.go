package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/zachcurry13/novelcheck/internal/calibresrv"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A correction to a Calibre book, from Hardcover: what to write and how to show it.
type hcChange struct {
	Field, Current, Shown string
	Payload               any
}

func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// person is a name without the order or punctuation: "Meyer, Marissa" and "Marissa Meyer" are the same person.
func person(name string) string {
	w := words(name)
	sort.Strings(w)
	return strings.Join(w, " ")
}

// seriesKey is a series name without case, punctuation and a leading "The".
func seriesKey(s string) string {
	w := words(s)
	if len(w) > 1 && w[0] == "the" {
		w = w[1:]
	}
	return strings.Join(w, " ")
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// hcChanges compares what Calibre has with what Hardcover says about the same book and lists what to fix.
// It only fills gaps or tidies the form of what is already right (never swaps one book's facts for another's);
// match must already be an exact match on ISBN or on title and author.
func hcChanges(cal calibresrv.Details, hc hardcover.BookInfo) []hcChange {
	var out []hcChange
	add := func(field, cur, shown string, payload any) { out = append(out, hcChange{field, cur, shown, payload}) }

	if cal.Title != "" && hc.Title != "" && cal.Title != hc.Title && store.NormKey(cal.Title, "") == store.NormKey(hc.Title, "") {
		add("title", cal.Title, hc.Title, hc.Title)
	}
	if len(cal.Authors) == 1 && len(hc.Authors) > 0 && cal.Authors[0] != hc.Authors[0] && person(cal.Authors[0]) == person(hc.Authors[0]) {
		add("authors", cal.Authors[0], hc.Authors[0], []string{hc.Authors[0]})
	}
	if hc.Series != "" && hc.Position > 0 {
		switch {
		case cal.Series == "":
			add("series", "(none)", hc.Series, hc.Series)
			add("series_index", "(none)", num(hc.Position), hc.Position) // the number goes last
		case seriesKey(cal.Series) == seriesKey(hc.Series) && cal.SeriesIndex != hc.Position:
			add("series_index", num(cal.SeriesIndex), num(hc.Position), hc.Position)
		}
	}
	if cal.Identifiers["isbn"] == "" && hc.ISBN13 != "" {
		ids := map[string]string{"isbn": hc.ISBN13}
		for k, v := range cal.Identifiers {
			ids[k] = v
		}
		add("identifiers", "(no ISBN)", "isbn:"+hc.ISBN13, ids)
	}
	if cal.Comments == "" && hc.Description != "" {
		add("comments", "(empty)", firstChars(hc.Description, 160), hc.Description)
	}
	if cal.Cover == "" && strings.HasPrefix(hc.Image, "https://") {
		add("cover", "(no cover)", "the cover from Hardcover", hc.Image)
	}
	have := map[string]bool{}
	for _, t := range cal.Tags {
		have[strings.ToLower(t)] = true
	}
	merged, fresh := append([]string{}, cal.Tags...), []string{}
	for _, t := range hc.Tags {
		if !have[strings.ToLower(t)] {
			merged, fresh = append(merged, t), append(fresh, t)
		}
	}
	if len(fresh) > 0 {
		add("tags", strings.Join(cal.Tags, ", "), "adds "+strings.Join(fresh, ", "), merged)
	}
	return out
}

func firstChars(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// fieldsOf turns suggestions for one book into [field, value] pairs for calibre, series before its number.
func fieldsOf(list []store.HCSuggestion) ([][]any, error) {
	order := map[string]int{"title": 0, "authors": 1, "series": 2, "series_index": 3, "identifiers": 4, "comments": 5, "tags": 6}
	sort.SliceStable(list, func(i, j int) bool { return order[list[i].Field] < order[list[j].Field] })
	var out [][]any
	for _, x := range list {
		if x.Field == "cover" { // set on its own (a picture, not a field): see runHCApply
			continue
		}
		if _, ok := order[x.Field]; !ok {
			return nil, fmt.Errorf("unknown field %q", x.Field)
		}
		var v any
		if err := json.Unmarshal([]byte(x.Payload), &v); err != nil {
			return nil, err
		}
		out = append(out, []any{x.Field, v})
	}
	return out, nil
}
