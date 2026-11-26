package kindleapp

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// appHash is the app's files for this Version. A change to app/ without a new Version would never reach a Kindle
// (the app only updates to a newer version), so this fails until Version is raised and the hash written here.
const appHash = "31c168523e0773cdeaa4a4b842d234a2e337381894b440c06039e071e6f9f809"

func TestVersionRaisedWithTheFiles(t *testing.T) {
	h := sha256.New()
	for _, n := range Files() {
		b, ok := File(n)
		if !ok {
			t.Fatalf("%s can't be read", n)
		}
		h.Write([]byte(n))
		h.Write(b)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != appHash {
		t.Fatalf("the Kindle app's files changed: raise Version (now %s) and set appHash to %s", Version, got)
	}
	if _, ok := File("config.js"); ok {
		t.Fatal("config.js is per account and never served")
	}
}
