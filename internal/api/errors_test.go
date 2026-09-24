package api_test

import (
	"fmt"
	"testing"
)

func TestErrorsListAndRetry(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	timeout := "qwen2.5:7b didn't answer within 2m0s"
	for i := 0; i < 3; i++ {
		id, _ := st.UpsertBook(fmt.Sprintf("Slow %d", i), "A", "", "")
		_ = st.SetStatus(id, "error", timeout)
	}
	bad, _ := st.UpsertBook("Bad Address", "B", "", "")
	_ = st.SetStatus(bad, "error", `parse "10.13.6.41:30068/chat/completions": first path segment in URL cannot contain colon`)

	res, out := admin.do("GET", "/api/admin/errors", nil, false)
	groups, _ := out["groups"].([]any)
	if res.StatusCode != 200 || len(groups) != 2 {
		t.Fatalf("errors: %d %v", res.StatusCode, out)
	}
	top := groups[0].(map[string]any)
	if top["message"] != timeout || top["count"].(float64) != 3 || len(top["titles"].([]any)) != 3 {
		t.Fatalf("top group: %v", top)
	}

	// Retry just the timeouts, then everything left.
	if res, out := admin.do("POST", "/api/admin/retry-errors", map[string]string{"message": timeout}, true); res.StatusCode != 200 || out["queued"].(float64) != 3 {
		t.Fatalf("retry group: %d %v", res.StatusCode, out)
	}
	if b, _ := st.BookByID(bad, nil); b.Status != "error" {
		t.Fatal("other errors must stay until retried")
	}
	if _, out := admin.do("POST", "/api/admin/retry-errors", nil, true); out["queued"].(float64) != 1 {
		t.Fatalf("retry all: %v", out)
	}
	if b, _ := st.BookByID(bad, nil); b.Status != "queued" {
		t.Fatalf("retried book should be queued, is %s", b.Status)
	}

	// Kids can't see or retry errors.
	admin.do("POST", "/api/admin/users", map[string]string{"username": "kiddo", "password": "kidpass12"}, true)
	k := login(t, srv, "kiddo", "kidpass12")
	if res, _ := k.do("GET", "/api/admin/errors", nil, false); res.StatusCode != 403 {
		t.Fatalf("kid errors: %d", res.StatusCode)
	}
}
