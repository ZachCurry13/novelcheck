package epub

import (
	"archive/zip"
	"errors"
	"strings"
)

// Meta is what the EPUB itself says it is: the title and first author in its package file (dc:title, dc:creator).
// Either may be "" (many files have none, or "Unknown").
func Meta(file string) (title, author string, err error) {
	zr, err := zip.OpenReader(file)
	if err != nil {
		return "", "", err
	}
	defer zr.Close()
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	var c container
	if err := readXML(files["META-INF/container.xml"], &c); err != nil || len(c.Rootfiles) == 0 {
		return "", "", errors.New("the EPUB has no container.xml")
	}
	var m struct {
		Titles   []string `xml:"metadata>title"`
		Creators []string `xml:"metadata>creator"`
	}
	if err := readXML(files[c.Rootfiles[0].FullPath], &m); err != nil {
		return "", "", err
	}
	if len(m.Titles) > 0 {
		title = strings.TrimSpace(m.Titles[0])
	}
	if len(m.Creators) > 0 {
		author = strings.TrimSpace(m.Creators[0])
	}
	return title, author, nil
}
