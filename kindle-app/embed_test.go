package kindleapp

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// appHash is the app's files for this Version. A change to app/ without a new Version would never reach a Kindle
// (the app only updates to a newer version), so this fails until Version is raised and the hash written here.
const appHash = "11b85d27a720e3b9c734e2eb8f006a8b9a4e53757c94e4b7b2c4725b1e6520f3"

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
