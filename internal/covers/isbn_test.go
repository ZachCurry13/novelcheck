package covers

import (
	"slices"
	"testing"
)

func TestISBN10(t *testing.T) {
	for in, want := range map[string]string{"9781984841582": "1984841580", "978-0-06-294161-9": "0062941615", "9780521123457": "0521123453", "1442362979": "1442362979", "9798212662314": "", "": ""} {
		if got := ISBN10(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestAuthorLinesWithSeveralPeople(t *testing.T) {
	for ours, found := range map[string][]string{
		"Rob Knight with Brendan Buhler":                  {"Rob Knight"},
		"Written By J. K. Rowling, Narrated By Jim Dale":  {"J. K. Rowling"},
		"Stephen King, Gregorio Vlastelica, Carlos Milla": {"Stephen King"},
		"Murray, Brian H. (Writer on real estate)":        {"Brian H. Murray"},
		"Sam Walton with John Huey":                       {"Sam Walton"},
	} {
		if !authorOK(found, ours) {
			t.Errorf("%q should match %v", ours, found)
		}
	}
	if authorOK([]string{"Someone Else"}, "Rob Knight with Brendan Buhler") {
		t.Error("a different author must not match")
	}
	if got := mainAuthor("Rob Knight with Brendan Buhler"); got != "Rob Knight" {
		t.Errorf("main author %q", got)
	}
	if got := mainAuthor("Meyer, Marissa"); got != "Meyer, Marissa" {
		t.Errorf("main author %q", got)
	}
}

func TestSearchTitlesStripBracketsAndCodes(t *testing.T) {
	for in, want := range map[string]string{
		"A Longa Marcha [The Long Walk]":                                            "The Long Walk",
		"Harry Potter and the Philosopher’s Stone (Full-Cast Edition) (Unabridged)": "Harry Potter and the Philosopher’s Stone",
		"HP-1 Harry Potter And The Philosopher's Stone":                             "Harry Potter And The Philosopher's Stone",
	} {
		if got := searchTitles(in); !slices.Contains(got, want) {
			t.Errorf("%q: %v lacks %q", in, got, want)
		}
	}
}
