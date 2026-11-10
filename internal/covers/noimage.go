package covers

import (
	"bytes"
	"image"
	"os"
)

// NoImage says whether a downloaded "cover" is really a shop's "No image available" picture: a white or
// pale card with grey words and no colour. Catalogues pass these along as if they were covers.
func NoImage(data []byte) bool {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return false
	}
	b := img.Bounds()
	step := max(1, max(b.Dx(), b.Dy())/64)
	var n, pale, colour int
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			r, g, bl, _ := img.At(x, y).RGBA()
			hi, lo := max(r, g, bl)>>8, min(r, g, bl)>>8
			n++
			if hi-lo > 40 {
				colour++
			}
			if lo > 225 {
				pale++
			}
		}
	}
	return n > 0 && colour*100 < n*2 && pale*100 >= n*70
}

// have returns a cover already kept at path. A small one that turns out to be a "No image available"
// picture (kept by an older version) is thrown away, so the book gets a real or drawn cover instead.
func (c Cache) have(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if info.Size() < 20000 {
		if data, err := os.ReadFile(path); err == nil && NoImage(data) {
			_ = os.Remove(path)
			_ = os.WriteFile(path+".none", nil, 0o644)
			return nil, false
		}
	}
	return info, true
}
