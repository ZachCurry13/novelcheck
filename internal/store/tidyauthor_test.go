package store_test

import "testing"

func TestTidyTitlesTakesTheAuthorOff(t *testing.T) {
	st := newStore(t)
	if _, err := st.DB.Exec(`INSERT INTO books (norm_key, title, author) VALUES ('x', 'Cornwell, Patricia_From Potter''s Field', 'Patricia Cornwell')`); err != nil {
		t.Fatal(err)
	}
	if n, err := st.TidyTitles(); err != nil || n != 1 {
		t.Fatalf("tidied %d: %v", n, err)
	}
	var row struct {
		Title string `db:"title"`
		Fix   string `db:"title_fix"`
	}
	_ = st.DB.Get(&row, `SELECT title, title_fix FROM books`)
	if row.Title != "From Potter's Field" || row.Fix != "Cornwell, Patricia_From Potter's Field" {
		t.Fatalf("%+v", row)
	}
}
