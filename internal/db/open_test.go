package db

import (
	"fmt"
	"sync"
	"testing"
)

// With several connections, reads go on while a write is happening, and many writers queue instead of failing.
func TestOpenManyConnections(t *testing.T) {
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if st := d.Stats(); st.MaxOpenConnections != 4 {
		t.Fatalf("a few connections by default: %d", st.MaxOpenConnections)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 200)
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				if _, err := d.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`, fmt.Sprintf("k%d-%d", w, i), "v"); err != nil {
					errs <- err
				}
			}
		}(w)
	}
	for r := 0; r < 6; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				var n int
				if err := d.Get(&n, `SELECT COUNT(*) FROM settings`); err != nil {
					errs <- err
				}
				tx, err := d.Beginx() // an immediate transaction: waits its turn, never fails halfway
				if err != nil {
					errs <- err
					continue
				}
				if _, err := tx.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('tx', 'v')`); err != nil {
					errs <- err
				}
				_ = tx.Commit()
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("a query failed under load: %v", err)
	}
}
