package enrich

import "testing"

func TestBetterName(t *testing.T) {
	cases := []struct {
		title, author string
		isbn          bool
		f             Found
		want          string // "" = keep what NovelCheck has
	}{
		{"Abby jiminez a married little Christmas", "Abby Jimenez", true, Found{Title: "A Married Little Christmas", Author: "Abby Jimenez"}, "A Married Little Christmas"},
		{"Abby jiminez a married little Christmas", "", false, Found{Title: "A Married Little Christmas", Author: "Abby Jimenez"}, ""}, // no author to be sure
		{"abby jiminez a married little christmas", "abby jiminez", false, Found{Title: "A Married Little Christmas", Author: "Abby Jimenez"}, "A Married Little Christmas"},
		{"fourth wing", "rebecca yarros", false, Found{Title: "Fourth Wing", Author: "Rebecca Yarros"}, "Fourth Wing"},
		{"Christmas", "Abby Jimenez", false, Found{Title: "A Christmas Carol", Author: "Charles Dickens"}, ""},           // another book
		{"Part of Your World", "Abby Jimenez", false, Found{Title: "Part of Your World", Author: "Abby Jimenez"}, ""},     // already right
		{"Yours Truly", "Abby Jimenez", true, Found{Title: "Yours Truly", Author: "Kate Smith"}, ""},                     // the ISBN's author disagrees
		{"Yours Truly", "Abby Jimenez", false, Found{Title: "Yours Truly: A Novel", Author: "Abby Jimenez"}, ""},          // more words than we have
	}
	for _, c := range cases {
		got, ok := BetterName(c.title, c.author, c.isbn, c.f)
		if (c.want == "" && ok) || (c.want != "" && (!ok || got.Title != c.want)) {
			t.Errorf("%q by %q: got %+v %v, want %q", c.title, c.author, got, ok, c.want)
		}
	}
}
