package analyzer

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"
)

func TestTurned(t *testing.T) {
	var b bytes.Buffer
	_ = jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 40, 60)), nil)
	for q, want := range map[int][2]int{1: {60, 40}, 2: {40, 60}, 3: {60, 40}} {
		out, ok := turned(b.Bytes(), q)
		if !ok {
			t.Fatalf("turn %d failed", q)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
		if err != nil || cfg.Width != want[0] || cfg.Height != want[1] {
			t.Fatalf("turn %d: %dx%d %v", q, cfg.Width, cfg.Height, err)
		}
	}
	if _, ok := turned([]byte("x"), 1); ok {
		t.Fatal("an unreadable photo can't be turned")
	}
}
