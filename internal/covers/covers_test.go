package covers

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestFindAndThumb(t *testing.T) {
	lib := t.TempDir()
	dir := filepath.Join(lib, "Terry Pratchett", "Mort (4)")
	_ = os.MkdirAll(dir, 0o755)
	big := image.NewRGBA(image.Rect(0, 0, 900, 1400))
	for y := 0; y < 1400; y++ {
		for x := 0; x < 900; x++ {
			big.Set(x, y, color.RGBA{200, 30, 30, 255})
		}
	}
	f, _ := os.Create(filepath.Join(dir, "cover.jpg"))
	_ = jpeg.Encode(f, big, nil)
	f.Close()

	inside := func(p string) bool { return strings.HasPrefix(p, lib) }
	copies := []store.BookCopy{{Source: "calibre", Path: "calibre-entry:9"}, {Source: "calibre", Path: filepath.Join(dir, "Mort.epub")}}
	src, info, ok := Find(copies, inside)
	if !ok || filepath.Base(src) != "cover.jpg" {
		t.Fatalf("find: %q %v", src, ok)
	}
	if _, _, ok := Find(copies, func(string) bool { return false }); ok {
		t.Fatal("covers outside the library are never read")
	}

	c := Cache{Dir: t.TempDir()}
	p, err := c.Thumb(src, info, Small)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := os.Open(p)
	img, err := jpeg.Decode(g)
	g.Close()
	if err != nil || img.Bounds().Dx() != Small || img.Bounds().Dy() != 1400*Small/900 {
		t.Fatalf("thumb: %v %v", img.Bounds(), err)
	}
	if r, _, _, _ := img.At(10, 10).RGBA(); r>>8 < 180 {
		t.Fatalf("colour kept: %d", r>>8)
	}
	if again, _ := c.Thumb(src, info, Small); again != p {
		t.Fatal("made once")
	}
}

func TestPlaceholder(t *testing.T) {
	svg := string(Placeholder("The Very Long Title of a Book <with> Brackets & Ampersands", "Someone"))
	if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, "&lt;with&gt;") || strings.Contains(svg, "<with>") {
		t.Fatalf("escaped svg: %s", svg)
	}
}
