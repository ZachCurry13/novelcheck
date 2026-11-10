package store_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A book that would not convert can be left as it is: it is then off the convert list until offered again.
func TestConvertLeftAsIs(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Only Mobi", "Someone", "", "")
	_ = st.AddCopy(cal, id, "/c/20/only.mobi", "mobi", "20")
	if list, _ := st.ToConvert(); len(list) != 1 {
		t.Fatalf("to convert: %v", list)
	}
	if err := st.SetConvertAsIs("20", true); err == nil {
		t.Fatal("a book that never failed can't be left as is")
	}
	_ = st.NoteConvertFailure("20", "Only Mobi", "mobi", "calibre couldn't convert it")
	if list, _ := st.ToConvert(); len(list) != 1 {
		t.Fatal("a failure alone keeps the book on the list")
	}
	if err := st.SetConvertAsIs("20", true); err != nil {
		t.Fatal(err)
	}
	if list, _ := st.ToConvert(); len(list) != 0 {
		t.Fatalf("left as is: off the list, got %v", list)
	}
	_ = st.NoteConvertFailure("20", "Only Mobi", "mobi", "again") // a later failure keeps the choice
	if f := st.ConvertFailures(); len(f) != 1 || !f[0].AsIs || f[0].Reason != "again" {
		t.Fatalf("failures: %+v", f)
	}
	_ = st.SetConvertAsIs("20", false)
	if list, _ := st.ToConvert(); len(list) != 1 {
		t.Fatal("offered again")
	}
}
