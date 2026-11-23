package covers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/enrich"
)

func testJPEG(t *testing.T) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 300, 450)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Google's daily limit is not "no cover": no miss is written, Google rests for the day, and an ISBN the matching
// Open Library book carries comes back.
func TestRefusedSearchIsNoMiss(t *testing.T) {
	googleAsked := 0
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/search.json":
			fmt.Fprint(w, `{"docs":[{"title":"Frogs at Noontide","author_name":["Janeen Ippolito"],"isbn":["1234567890","9781234567897"]}]}`)
		case strings.HasPrefix(r.URL.Path, "/books/v1/"):
			googleAsked++
			http.Error(w, "quota", http.StatusTooManyRequests)
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()
	defer enrich.ResumeGoogle()
	c := Cache{Dir: t.TempDir(), SearchURL: fake.URL, OpenLibraryURL: fake.URL, GoogleURL: fake.URL}
	r := c.SearchBook(context.Background(), "book-1", "Frogs at Noontide: A Quirky YA Frog Prince Retelling", "Janeen Ippolito")
	if r.OK || !r.Refused || r.ISBN != "9781234567897" {
		t.Fatalf("refused, with the ISBN: %+v", r)
	}
	if c.Missed("book-1") || !enrich.GoogleResting() {
		t.Fatal("no miss written, and Google rests")
	}
	asked := googleAsked
	r = c.SearchBook(context.Background(), "book-2", "Another Book", "Someone")
	if googleAsked != asked || !r.Refused {
		t.Fatalf("Google is left alone for the day (asked %d more times)", googleAsked-asked)
	}
	nog := Cache{Dir: t.TempDir(), SearchURL: fake.URL, OpenLibraryURL: fake.URL, GoogleURL: fake.URL, NoGoogle: true}
	if r := nog.SearchBook(context.Background(), "book-3", "Nothing Here", "Nobody"); r.Refused || !nog.Missed("book-3") {
		t.Fatalf("without Google a plain miss is remembered: %+v", r)
	}
}

// Amazon's cover by ASIN is used; its 1-pixel answer for an unknown number is not.
func TestAmazonCover(t *testing.T) {
	img := testJPEG(t)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "B00JO8PEN2") {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(img)
			return
		}
		w.Header().Set("Content-Type", "image/gif")
		w.Write([]byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\xff\xff\xff\x00\x00\x00!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"))
	}))
	defer fake.Close()
	c := Cache{Dir: t.TempDir(), AmazonURL: fake.URL}
	if c.Amazon(context.Background(), "book-1", "B002MQYOFU") {
		t.Fatal("the 1-pixel picture is not a cover")
	}
	if !c.Amazon(context.Background(), "book-2", "b00jo8pen2") {
		t.Fatal("the real cover is kept")
	}
	if c.Amazon(context.Background(), "book-3", "../etc") {
		t.Fatal("only an ASIN is asked for")
	}
	got := ASINsIn("Loving the Lonely Viscount_B00JO8PEN2.kfx", "B07ABCDEFG_EBOK.azw", "xB00JO8PEN2", "documents/B00JO8PEN2.azw3")
	if strings.Join(got, ",") != "B00JO8PEN2,B07ABCDEFG" {
		t.Fatalf("ASINs in names: %v", got)
	}
}

// The cover inside an EPUB (named in its package file) and inside a MOBI/AZW3 (EXTH 201) is kept.
func TestCoverFromBookFile(t *testing.T) {
	img := testJPEG(t)
	dir := t.TempDir()
	epub := filepath.Join(dir, "Frogs.epub")
	f, _ := os.Create(epub)
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{
		"META-INF/container.xml":      `<container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`,
		"OEBPS/content.opf":           `<package><metadata><meta name="cover" content="c1"/></metadata><manifest><item id="c1" href="images/front%20page.jpg" media-type="image/jpeg"/></manifest></package>`,
		"OEBPS/images/front page.jpg": string(img),
	} {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	f.Close()
	c := Cache{Dir: filepath.Join(dir, "covers")}
	if !c.FromBookFile("book-1", epub) {
		t.Fatal("the EPUB's cover is kept")
	}

	// A MOBI: record 0 (PalmDOC header, MOBI header with EXTH 201 = 1), record 1 text, records 2 and 3 pictures.
	mobiHdr := make([]byte, 0xE8)
	copy(mobiHdr[0:], "MOBI")
	binary.BigEndian.PutUint32(mobiHdr[4:], uint32(len(mobiHdr)))
	binary.BigEndian.PutUint32(mobiHdr[0x6C-16:], 2)    // first picture is record 2
	binary.BigEndian.PutUint32(mobiHdr[0x80-16:], 0x40) // has EXTH
	exth := []byte("EXTH")
	exth = binary.BigEndian.AppendUint32(exth, 24)
	exth = binary.BigEndian.AppendUint32(exth, 1)
	exth = binary.BigEndian.AppendUint32(exth, 201)
	exth = binary.BigEndian.AppendUint32(exth, 12)
	exth = binary.BigEndian.AppendUint32(exth, 1) // the cover is picture 1, record 3
	rec0 := append(append(make([]byte, 16), mobiHdr...), exth...)
	recs := [][]byte{rec0, []byte("text"), []byte("not the cover, a small picture......"), img}
	var out bytes.Buffer
	head := make([]byte, 78)
	binary.BigEndian.PutUint16(head[76:], uint16(len(recs)))
	out.Write(head)
	off := 78 + 8*len(recs)
	for _, r := range recs {
		var e [8]byte
		binary.BigEndian.PutUint32(e[:], uint32(off))
		out.Write(e[:])
		off += len(r)
	}
	for _, r := range recs {
		out.Write(r)
	}
	mobi := filepath.Join(dir, "Viscount_B00JO8PEN2.azw3")
	os.WriteFile(mobi, out.Bytes(), 0o644)
	if !c.FromBookFile("book-2", mobi) {
		t.Fatal("the AZW3's cover is kept")
	}
	if c.FromBookFile("book-3", filepath.Join(dir, "missing.epub")) {
		t.Fatal("no file, no cover")
	}
}
