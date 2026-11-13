package titles

import "testing"

func TestWithoutAuthor(t *testing.T) {
	for _, c := range []struct{ title, author, want string }{
		{"Cornwell, Patricia_From Potter's Field", "Patricia Cornwell", "From Potter's Field"},
		{"Cornwell, Patricia_From Potter's Field", "Cornwell, Patricia", "From Potter's Field"},
		{"Patricia Cornwell - Postmortem", "Patricia Cornwell", "Postmortem"},
		{"Postmortem - Patricia Cornwell", "Patricia Cornwell", "Postmortem"},
		{"Postmortem by Patricia Cornwell", "Patricia Cornwell", "Postmortem"},
		{"Postmortem (by Patricia Cornwell)", "Patricia Cornwell", "Postmortem"},
		{"Cornwell_Body of Evidence", "Patricia Cornwell", "Body of Evidence"},
		{"P. Cornwell: Cruel and Unusual", "Patricia Cornwell", "Cruel and Unusual"},
		{"Stephen King_It", "Stephen King", "It"},
		{"George R. R. Martin - A Game of Thrones", "George R. R. Martin", "A Game of Thrones"},
		{"The_Hunger_Games", "Suzanne Collins", "The Hunger Games"},
		// left alone
		{"Eleanor Oliphant Is Completely Fine", "Gail Honeyman", "Eleanor Oliphant Is Completely Fine"},
		{"King Lear", "William Shakespeare", "King Lear"},
		{"Kingdom of Ash", "Sarah J. Maas", "Kingdom of Ash"},
		{"Cornwell", "Patricia Cornwell", "Cornwell"},
		{"Dune", "Frank Herbert", "Dune"},
		{"Martin Eden", "Jack London", "Martin Eden"},
		{"Dune", "Herbert", "Dune"},
		{"Old young_adult_fiction", "Sam Lister", "Old young_adult_fiction"}, // a title with spaces keeps its underscores
		{"Stephen_King_It", "Stephen King", "Stephen King It"},               // a file name without the joins we know
	} {
		if got := WithoutAuthor(c.title, c.author); got != c.want {
			t.Errorf("%q by %q: got %q, want %q", c.title, c.author, got, c.want)
		}
	}
}
