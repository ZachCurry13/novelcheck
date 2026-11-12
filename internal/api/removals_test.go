package api_test

import (
	"testing"
)

// Every book NovelCheck removes from Calibre is logged with who removed it and from which page.
func TestCalibreRemovalLog(t *testing.T) {
	_, st := setup(t)
	if err := st.LogCalibreRemovals("admin", "Duplicates in Calibre", map[int]string{7: "Rant", 9: "2001"}, []int{7, 9}); err != nil {
		t.Fatal(err)
	}
	got := st.CalibreRemovals(10)
	if len(got) != 2 || got[0].CalibreID != 9 || got[0].Title != "2001" || got[1].By != "admin" || got[1].Tool != "Duplicates in Calibre" {
		t.Fatalf("the log, newest first: %+v", got)
	}
}
