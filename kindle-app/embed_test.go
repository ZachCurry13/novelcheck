package kindleapp

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// appHash is the app's files for this Version. A change to app/ without a new Version would never reach a Kindle
// (the app only updates to a newer version), so this fails until Version is raised and the hash written here.
const appHash = "addf97a60a7292019de6a3b308111019d4820fdb9f41d5ff7eb58464358885fb"

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
