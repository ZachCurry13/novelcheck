package api_test

import (
	"strings"
	"testing"
)

// A Library cleanup action that is refused goes to the cleanup log (and so the bug link), with what was asked.
func TestCleanupLogRecordsRefusals(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	if res, _ := admin.do("POST", "/api/admin/title-fixes", map[string]any{"ids": []int64{}}, true); res.StatusCode < 400 {
		t.Fatalf("an empty choice is refused: %d", res.StatusCode)
	}
	log := st.CleanupLog(5)
	if len(log) != 1 || !strings.Contains(log[0].Action, "/api/admin/title-fixes") || !strings.Contains(log[0].Text, `"ids":[]`) {
		t.Fatalf("refusal logged with the request: %+v", log)
	}
	if res, _ := admin.do("GET", "/api/admin/title-fixes", nil, false); res.StatusCode != 200 || len(st.CleanupLog(5)) != 1 {
		t.Fatal("reading a list is not logged")
	}
}
