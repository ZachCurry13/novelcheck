package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A book KOReader marked finished leaves "reading" on the Nightstand, even if its last page
// is short of 97%; a book that is not on the Nightstand is not added, and a book put back
// after that day (to read again) stays where it is.
func TestDeviceFinishedMovesNightstand(t *testing.T) {
	_, st := setup(t)
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	u, _ := st.UserByName("admin")
	mk := func(title, author string) int64 {
		id, _ := st.UpsertBook(title, author, "", "")
		_ = st.AddCopy(cal, id, "/c/"+title+".epub", "epub", title)
		return id
	}
	sam, other, again := mk("The Search for Sam", "Pittacus Lore"), mk("Hunted", "Pittacus Lore"), mk("Re-read Me", "Someone Else")
	_ = st.Enqueue(u.ID, sam)
	_, _ = st.SyncReading(u.ID, sam, false) // reading
	_ = st.Enqueue(u.ID, again)
	_, _ = st.SyncReading(u.ID, again, false)
	status := func(id int64) string {
		var s string
		_ = st.DB.Get(&s, `SELECT status FROM queue_items WHERE user_id = ? AND book_id = ?`, u.ID, id)
		return s
	}
	if status(sam) != "reading" {
		t.Fatalf("setup: %q", status(sam))
	}
	books := []store.DeviceBook{
		{Filename: "sam.epub", MD5: "a1", Title: "The Search for Sam", Author: "Pittacus Lore", Finished: true, FinishedOn: "2099-01-01"},
		{Filename: "hunted.epub", MD5: "a2", Title: "Hunted", Author: "Pittacus Lore", Finished: true, FinishedOn: "2099-01-01"},
		{Filename: "again.epub", MD5: "a3", Title: "Re-read Me", Author: "Someone Else", Finished: true, FinishedOn: "2000-01-01"},
	}
	if _, err := st.SaveDeviceBooks(u.ID, "Kindle", books); err != nil {
		t.Fatal(err)
	}
	if got := status(sam); got != "finished" {
		t.Fatalf("KOReader's finished mark moves it: %q", got)
	}
	if got := status(other); got != "" {
		t.Fatalf("a book that was never on the Nightstand is not added: %q", got)
	}
	if got := status(again); got != "reading" {
		t.Fatalf("a book put back after it was finished stays: %q", got)
	}
}
