package epub

import (
	"strings"
	"testing"
)

const container1 = `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/c.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`

func TestFrontFindsTheTitleAndCopyrightPages(t *testing.T) {
	long := strings.Repeat("The road ran on through the dust. ", 20)
	p := writeEPUB(t, map[string]string{
		"META-INF/container.xml": container1,
		"OEBPS/c.opf": `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/"><metadata>
			<dc:title>_OceanofPDF.com_junk</dc:title><dc:creator>Unknown</dc:creator><dc:publisher>Penguin</dc:publisher><dc:identifier>urn:isbn:9780141439518</dc:identifier></metadata>
			<manifest><item id="x1" href="Text/a.xhtml" media-type="application/xhtml+xml"/><item id="x2" href="Text/b.xhtml" media-type="application/xhtml+xml"/>
			<item id="x3" href="Text/c.xhtml" media-type="application/xhtml+xml"/></manifest>
			<spine><itemref idref="x1"/><itemref idref="x2"/><itemref idref="x3"/></spine>
			<guide><reference type="title-page" href="Text/b.xhtml"/><reference type="copyright-page" href="Text/a.xhtml"/></guide></package>`,
		"OEBPS/Text/a.xhtml": `<html><body><p>First published 1847. Copyright (c) the estate. All rights reserved. ISBN 978-0-14-143951-8 Penguin Classics</p></body></html>`,
		"OEBPS/Text/b.xhtml": `<html><body><h1>PRIDE AND PREJUDICE</h1><p>Jane Austen</p></body></html>`,
		"OEBPS/Text/c.xhtml": `<html><body><h1>Chapter 1</h1><p>` + long + `</p></body></html>`,
	})
	fm, err := Front(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fm.TitlePage, "PRIDE AND PREJUDICE") || !strings.Contains(fm.CopyrightPage, "All rights reserved") || !strings.Contains(fm.Opening, "The road ran on") {
		t.Fatalf("pages: %+v", fm)
	}
	if fm.Publisher != "Penguin" || len(fm.ISBNs) != 1 || fm.ISBNs[0] != "9780141439518" {
		t.Fatalf("metadata: %+v", fm)
	}
}

// With no guide, the pages are found by what the files are called, and the copyright page by its words.
func TestFrontFindsPagesByNameAndWords(t *testing.T) {
	p := writeEPUB(t, map[string]string{
		"META-INF/container.xml": container1,
		"OEBPS/c.opf": `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf"><metadata/>
			<manifest><item id="cover" href="cover.xhtml" media-type="application/xhtml+xml"/><item id="tp" href="titlepage.xhtml" media-type="application/xhtml+xml"/>
			<item id="s3" href="section0003.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="cover"/><itemref idref="tp"/><itemref idref="s3"/></spine></package>`,
		"OEBPS/cover.xhtml":       `<html><body><p>Cover image</p></body></html>`,
		"OEBPS/titlepage.xhtml":   `<html><body><h1>Emma</h1><p>by Jane Austen</p></body></html>`,
		"OEBPS/section0003.xhtml": `<html><body><p>Copyright © 2011. All rights reserved. ISBN-10: 0-14-143951-3</p></body></html>`,
	})
	fm, err := Front(p)
	if err != nil || !strings.Contains(fm.TitlePage, "Emma") || !strings.Contains(fm.CopyrightPage, "All rights reserved") {
		t.Fatalf("%+v %v", fm, err)
	}
	if len(fm.ISBNs) != 1 || fm.ISBNs[0] != "0141439513" {
		t.Fatalf("isbn: %v", fm.ISBNs)
	}
}

func TestOnlyValidISBNsCount(t *testing.T) {
	for in, want := range map[string]string{"978-0-14-143951-8": "9780141439518", "0-14-143951-3": "0141439513", "978-0-14-143951-9": "", "12345": "", "urn:isbn:9780141439518": "9780141439518"} {
		if got := cleanISBN(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}
