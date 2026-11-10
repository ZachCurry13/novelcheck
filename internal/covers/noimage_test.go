package covers

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func picture(fill func(x, y int) color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 120, 180))
	for y := 0; y < 180; y++ {
		for x := 0; x < 120; x++ {
			img.Set(x, y, fill(x, y))
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, nil)
	return b.Bytes()
}

func TestNoImage(t *testing.T) {
	card := picture(func(x, y int) color.Color { // white with a band of grey words
		if y > 80 && y < 100 && x%3 == 0 {
			return color.Gray{Y: 150}
		}
		return color.White
	})
	if !NoImage(card) {
		t.Fatal("a white card with grey words is not a cover")
	}
	cover := picture(func(x, y int) color.Color { return color.RGBA{R: uint8(x * 2), G: 90, B: uint8(y), A: 255} })
	if NoImage(cover) {
		t.Fatal("a coloured cover is a cover")
	}
	if NoImage([]byte("not a picture")) {
		t.Fatal("something unreadable is left alone")
	}
}
