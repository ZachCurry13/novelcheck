package kosync

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// The fingerprint reads 1 KB at 0, 1 KB, 4 KB, 16 KB… and stops at the end.
func TestPartialMD5(t *testing.T) {
	data := make([]byte, 20000)
	for i := range data {
		data[i] = byte(i * 7)
	}
	p := filepath.Join(t.TempDir(), "book.epub")
	_ = os.WriteFile(p, data, 0o644)
	h := md5.New()
	for _, off := range []int{0, 1024, 4096, 16384} { // 65536 is past the end
		end := min(off+1024, len(data))
		h.Write(data[off:end])
	}
	want := hex.EncodeToString(h.Sum(nil))
	if got, err := PartialMD5(p); err != nil || got != want {
		t.Fatalf("got %s (%v), want %s", got, err, want)
	}
	// A tiny file: just its bytes.
	small := filepath.Join(t.TempDir(), "s.epub")
	_ = os.WriteFile(small, []byte("hello"), 0o644)
	sum := md5.Sum([]byte("hello"))
	if got, _ := PartialMD5(small); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("small file: %s", got)
	}
}
