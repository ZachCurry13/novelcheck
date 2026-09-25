// Package covers finds each book's cover in the Calibre library (the
// cover.jpg Calibre keeps next to a book's files), shrinks it once and keeps
// the small copy under /data/covers. Books without one get a drawn
// placeholder. The library itself is only ever read.
package covers

import (
	"crypto/sha1"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // some libraries hold PNG covers named .jpg
	"os"
	"path/filepath"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Widths, in pixels.
const (
	Small = 240 // cards and lists
	Large = 480 // the book window
)

// Cache keeps shrunk covers (and ones from Open Library) in Dir.
type Cache struct {
	Dir            string
	OpenLibraryURL string // "" = covers.openlibrary.org (tests use a fake)
}

// Find returns the cover.jpg next to any of the book's Calibre files that
// inside allows (it must stay within the library mount).
func Find(copies []store.BookCopy, inside func(string) bool) (string, os.FileInfo, bool) {
	for _, c := range copies {
		if c.Source != "calibre" || c.Path == "" || strings.HasPrefix(c.Path, "calibre-entry:") {
			continue
		}
		p := filepath.Join(filepath.Dir(c.Path), "cover.jpg")
		if !inside(p) {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return p, info, true
		}
	}
	return "", nil, false
}

// Thumb returns the path of a JPEG copy of src at most width pixels wide,
// made once. The name includes the cover's time and size, so a cover
// changed in Calibre is picked up by itself.
func (c Cache) Thumb(src string, info os.FileInfo, width int) (string, error) {
	if c.Dir == "" {
		return "", fmt.Errorf("no cover cache folder")
	}
	key := sha1.Sum([]byte(fmt.Sprintf("%s|%d|%d", src, info.ModTime().UnixNano(), info.Size())))
	out := filepath.Join(c.Dir, fmt.Sprintf("%x-%d.jpg", key[:10], width))
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}
	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return "", fmt.Errorf("read cover: %w", err)
	}
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(c.Dir, "cover-*.tmp")
	if err != nil {
		return "", err
	}
	err = jpeg.Encode(tmp, shrink(img, width), &jpeg.Options{Quality: 82})
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), out)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return out, nil
}

// shrink scales img down to at most w pixels wide, averaging the source
// pixels under each new one (smooth, unlike just picking every nth pixel).
func shrink(img image.Image, w int) image.Image {
	b := img.Bounds()
	if b.Dx() <= w {
		return img
	}
	h := max(1, b.Dy()*w/b.Dx())
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0, y1 := b.Min.Y+y*b.Dy()/h, b.Min.Y+(y+1)*b.Dy()/h
		for x := 0; x < w; x++ {
			x0, x1 := b.Min.X+x*b.Dx()/w, b.Min.X+(x+1)*b.Dx()/w
			var r, g, bl, n uint64
			for sy := y0; sy < max(y1, y0+1); sy++ {
				for sx := x0; sx < max(x1, x0+1); sx++ {
					cr, cg, cb, _ := img.At(sx, sy).RGBA()
					r, g, bl, n = r+uint64(cr), g+uint64(cg), bl+uint64(cb), n+1
				}
			}
			i := dst.PixOffset(x, y)
			dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = uint8(r/n>>8), uint8(g/n>>8), uint8(bl/n>>8), 255
		}
	}
	return dst
}
