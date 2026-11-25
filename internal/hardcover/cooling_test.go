package hardcover

import (
	"testing"
	"time"
)

// CoolingDown reports the shared hold after "too many requests", for Health to say so.
func TestCoolingDown(t *testing.T) {
	c := &Client{Token: "cool-token"}
	if d := CoolingDown("cool-token"); d != 0 {
		t.Fatalf("no hold yet: %v", d)
	}
	c.coolDown(time.Minute)
	if d := CoolingDown("Bearer cool-token"); d < 50*time.Second || d > time.Minute {
		t.Fatalf("held: %v", d)
	}
}
