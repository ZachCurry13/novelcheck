package calibresrv

import (
	"bytes"
	"testing"
)

func TestMsgpack(t *testing.T) {
	got, err := msgpack([]any{7, []any{"a.epub", []byte{1, 2}}, "EPUB", false})
	want := []byte{0x94, 0x07, 0x92, 0xa6, 'a', '.', 'e', 'p', 'u', 'b', 0xc4, 0x02, 1, 2, 0xa4, 'E', 'P', 'U', 'B', 0xc2}
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("got % x (%v)\nwant % x", got, err, want)
	}
	big := bytes.Repeat([]byte{9}, 300)
	if got, _ := msgpack(big); got[0] != 0xc5 || got[1] != 0x01 || got[2] != 0x2c || len(got) != 303 {
		t.Fatalf("bin16: % x", got[:3])
	}
	if got, _ := msgpack("0123456789012345678901"); got[0] != 0xb6 { // 22-character fixstr
		t.Fatalf("fixstr: % x", got[0])
	}
	if got, _ := msgpack(1000); !bytes.Equal(got, []byte{0xcf, 0, 0, 0, 0, 0, 0, 0x03, 0xe8}) {
		t.Fatalf("uint64: % x", got)
	}
}
