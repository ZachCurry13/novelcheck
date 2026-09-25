package api_test

import (
	"strconv"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Batch size 0 means "all waiting books"; blank goes back to the default;
// nonsense is refused instead of breaking batches.
func TestBatchSizeGuardrail(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	for i := 0; i < 25; i++ {
		_, _ = st.UpsertBook("Book "+strconv.Itoa(i), "A", "", "")
	}
	for _, c := range []struct {
		in   string
		want int
	}{{"0", 200}, {"", 200}, {"500", 200}, {"501", 400}, {"-1", 400}, {"2.5", 400}, {"ten", 400}} {
		res, _ := admin.do("PUT", "/api/admin/settings", map[string]string{"batch_size": c.in}, true)
		if res.StatusCode != c.want {
			t.Errorf("batch_size %q: got %d, want %d", c.in, res.StatusCode, c.want)
		}
	}
	if st.Setting(store.KeyBatchSize) != "500" {
		t.Fatalf("last accepted value should stick: %q", st.Setting(store.KeyBatchSize))
	}
	admin.do("PUT", "/api/admin/settings", map[string]string{"batch_size": ""}, true)
	if st.Setting(store.KeyBatchSize) != "20" {
		t.Fatalf("blank should reset to the default: %q", st.Setting(store.KeyBatchSize))
	}
	admin.do("PUT", "/api/admin/settings", map[string]string{"batch_size": "0"}, true)
	if _, out := admin.do("POST", "/api/admin/analyze-batch", nil, true); out["queued"].(float64) != 25 {
		t.Fatalf("0 should queue every waiting book: %v", out)
	}
}
