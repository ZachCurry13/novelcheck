package analyzer

import (
	"bytes"
	"image"
	"image/jpeg"
	_ "image/png" // photos may arrive as PNG
)

// turned returns the photo turned a quarter (1), half (2) or three quarters (3) of a turn clockwise, as a
// JPEG: for a cover photographed sideways or upside down. ok is false when the photo can't be opened here
// (WebP, for one).
func turned(photo []byte, quarters int) ([]byte, bool) {
	src, _, err := image.Decode(bytes.NewReader(photo))
	if err != nil {
		return nil, false
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if quarters%2 == 1 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch quarters % 4 {
			case 1:
				dx, dy = h-1-y, x
			case 2:
				dx, dy = w-1-x, h-1-y
			case 3:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	var out bytes.Buffer
	if jpeg.Encode(&out, dst, &jpeg.Options{Quality: 85}) != nil {
		return nil, false
	}
	return out.Bytes(), true
}
