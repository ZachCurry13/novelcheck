package store_test

import (
	"fmt"
	"testing"
)

// The running scan comes first, then the queue in the order it will run;
// dragging changes which one runs next.
func TestDeepScanQueueOrder(t *testing.T) {
	s := newStore(t)
	var scans []int64
	for i := range 3 {
		b, _ := s.UpsertBook(fmt.Sprintf("Book %d", i), "Author", "", "")
		id, _ := s.RequestDeepRead(b, "admin", "admin", "", true, 1000, 1, 1)
		scans = append(scans, id)
	}
	first, _ := s.NextDeepRead()
	_ = s.SetDeepProgress(first.ID, 0, 1, "m", nil) // it starts reading
	order := func() []int64 {
		ds, _ := s.DeepReads(10)
		ids := []int64{}
		for _, d := range ds {
			ids = append(ids, d.ID)
		}
		return ids
	}
	if got := order(); fmt.Sprint(got) != fmt.Sprint(scans) || first.ID != scans[0] {
		t.Fatalf("run order %v, want %v", got, scans)
	}
	if err := s.OrderDeepReads([]int64{scans[2], scans[1]}); err != nil {
		t.Fatal(err)
	}
	if got := order(); fmt.Sprint(got) != fmt.Sprint([]int64{scans[0], scans[2], scans[1]}) {
		t.Fatalf("after dragging: %v", got)
	}
}
