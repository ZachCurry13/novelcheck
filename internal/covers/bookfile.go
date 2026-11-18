package covers

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"image"
	_ "image/gif" // a few books keep a GIF cover
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// FromBookFile saves the cover kept inside a book file as name.jpg: an EPUB's cover picture, or the cover record
// of a MOBI, AZW or AZW3 (Kindle) file, whose pictures stay readable even when its text is locked. A self-published
// Kindle book no catalog knows still carries its own cover. It says whether it saved one.
func (c Cache) FromBookFile(name, file string) bool {
	if c.Dir == "" || !storedName.MatchString(name) {
		return false
	}
	var img []byte
	switch strings.ToLower(filepath.Ext(file)) {
	case ".epub":
		img = epubCover(file)
	case ".mobi", ".azw", ".azw3", ".prc":
		img = mobiCover(file)
	}
	return c.saveImage(name, img)
}

// saveImage keeps data as name.jpg when it is a picture big enough to be a cover.
func (c Cache) saveImage(name string, data []byte) bool {
	if len(data) < 200 || len(data) > 8<<20 || NoImage(data) {
		return false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 100 || cfg.Height < 100 {
		return false
	}
	out := filepath.Join(c.Dir, name+".jpg")
	if os.MkdirAll(c.Dir, 0o755) != nil || os.WriteFile(out, data, 0o644) != nil {
		return false
	}
	c.Forget(name)
	return true
}

// epubCover reads the cover picture an EPUB names in its package file (EPUB 3's cover-image, EPUB 2's
// <meta name="cover">), or else the first picture whose name says cover.
func epubCover(file string) []byte {
	zr, err := zip.OpenReader(file)
	if err != nil {
		return nil
	}
	defer zr.Close()
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	read := func(n string) []byte {
		f := files[n]
		if f == nil || f.UncompressedSize64 > 8<<20 {
			return nil
		}
		rc, err := f.Open()
		if err != nil {
			return nil
		}
		defer rc.Close()
		b, _ := io.ReadAll(io.LimitReader(rc, 8<<20))
		return b
	}
	var container struct {
		Roots []struct {
			Path string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if xml.Unmarshal(read("META-INF/container.xml"), &container) == nil && len(container.Roots) > 0 {
		opf := container.Roots[0].Path
		var pkg struct {
			Metas []struct {
				Name    string `xml:"name,attr"`
				Content string `xml:"content,attr"`
			} `xml:"metadata>meta"`
			Items []struct {
				ID    string `xml:"id,attr"`
				Href  string `xml:"href,attr"`
				Type  string `xml:"media-type,attr"`
				Props string `xml:"properties,attr"`
			} `xml:"manifest>item"`
		}
		if xml.Unmarshal(read(opf), &pkg) == nil {
			coverID := ""
			for _, m := range pkg.Metas {
				if strings.EqualFold(m.Name, "cover") {
					coverID = m.Content
				}
			}
			for _, it := range pkg.Items {
				if strings.Contains(it.Props, "cover-image") || (coverID != "" && it.ID == coverID && strings.HasPrefix(it.Type, "image/")) {
					href, err := url.PathUnescape(it.Href)
					if err != nil {
						href = it.Href
					}
					if b := read(path.Join(path.Dir(opf), href)); b != nil {
						return b
					}
				}
			}
		}
	}
	for _, f := range zr.File {
		n := strings.ToLower(path.Base(f.Name))
		if strings.Contains(n, "cover") && (strings.HasSuffix(n, ".jpg") || strings.HasSuffix(n, ".jpeg") || strings.HasSuffix(n, ".png")) {
			return read(f.Name)
		}
	}
	return nil
}
