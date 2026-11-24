package hardcover

import (
	"context"
	"testing"
	"time"
)

// Two clients with one token share one pace and one cool-down (the cover finder and the genre scan are separate clients).
func TestClientsShareOneGate(t *testing.T) {
	old := MinGap
	MinGap = 50 * time.Millisecond
	defer func() { MinGap = old }()
	a, b := &Client{Token: "same"}, &Client{Token: "same"}
	start := time.Now()
	_ = a.pause(context.Background())
	_ = b.pause(context.Background())
	if time.Since(start) < 45*time.Millisecond {
		t.Fatal("the second client did not wait for the first")
	}
	a.coolDown(80 * time.Millisecond)
	start = time.Now()
	_ = b.pause(context.Background())
	if time.Since(start) < 70*time.Millisecond {
		t.Fatal("a cool-down is shared by every client with the token")
	}
}
