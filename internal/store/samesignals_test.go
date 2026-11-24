package store

import "testing"

// One ISBN on book 1 and book 10 of a series, one file much bigger: the ISBN is wrong on one of them, so the pair is
// "check" (a person looks), never "sure" (merged by itself).
func TestSameIsbnOnDifferentSeriesPlacesIsNotSure(t *testing.T) {
	var s *Store // sizes are given, so judge never reads the database
	row := func(id int64, index float64, size int64) *sameRow {
		return &sameRow{SameBook: SameBook{ID: id, ISBN: "9780062109378", Series: "The Lost Files", SeriesIndex: index, Size: size}, haveAuthor: true}
	}
	j := s.judge(row(1, 1, 339<<10), row(2, 10, 1300<<10), "exact")
	if j == nil || j.conf != "check" {
		t.Fatalf("got %+v", j)
	}
	if j = s.judge(row(1, 1, 339<<10), row(2, 1, 339<<10), "exact"); j == nil || j.conf != "sure" {
		t.Fatalf("the same ISBN and place stays sure: %+v", j)
	}
}
