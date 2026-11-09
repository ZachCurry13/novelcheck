package covers

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
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

// The cover finder searches by title and author, and only keeps a result whose title is the book's.
func TestSearchFindsMatchingCover(t *testing.T) {
	img := bytes.Repeat([]byte{0xff}, 400)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/search.json":
			fmt.Fprint(w, `{"docs":[{"title":"Dune Messiah","author_name":["Frank Herbert"],"cover_i":111},{"title":"Dune","author_name":["Frank Herbert"],"cover_i":222}]}`)
		case strings.HasPrefix(r.URL.Path, "/b/id/222-M.jpg"):
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(img)
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()
	c := Cache{Dir: t.TempDir(), SearchURL: fake.URL, OpenLibraryURL: fake.URL, GoogleURL: fake.URL}
	p, _, ok := c.Search(context.Background(), "book-5", "Dune", "Frank Herbert")
	if !ok || !strings.HasSuffix(p, "book-5.jpg") {
		t.Fatalf("the Dune cover, not Dune Messiah's: %q %v", p, ok)
	}
	if _, _, ok := c.Search(context.Background(), "book-6", "Dune", "Someone Else"); ok {
		t.Fatal("a different author's book is not used")
	}
	if _, _, ok := c.Search(context.Background(), "book-7", "A Book Nobody Wrote", "Frank Herbert"); ok {
		t.Fatal("no matching title, no cover")
	}
	if _, err := os.Stat(filepath.Join(c.Dir, "book-7.jpg.none")); err != nil {
		t.Fatal("a miss is remembered")
	}
}
