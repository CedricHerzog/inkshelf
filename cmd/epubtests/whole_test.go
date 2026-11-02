package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/epubtest"
	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
)

// wholeBook returns the files of a book of one chapter of body, in language, styled by css.
func wholeBook(language, css, body string) map[string]string {
	return map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`,
		"OEBPS/content.opf": `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Book</dc:title><dc:language>` + language + `</dc:language></metadata>
  <manifest><item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/><item id="css" href="style.css" media-type="text/css"/></manifest>
  <spine><itemref idref="c1"/></spine>
</package>`,
		"OEBPS/style.css": css,
		"OEBPS/c1.xhtml": `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" lang="` + language + `"><head><title>Book</title><link rel="stylesheet" href="style.css"/></head>
<body>` + body + `</body></html>`,
	}
}

// A vertical Japanese book and an Arabic one, read whole: no missing glyph, every rune on a
// page, and pages turning from right to left (SC-003, on books made here rather than real ones).
func TestReadWholeBooks(t *testing.T) {
	japanese := strings.Repeat("<p>吾輩は猫である。名前はまだ無い。どこで生れたかとんと見当がつかぬ。「ニャー」と泣いた。<span style=\"text-combine-upright: all\">2026</span>年、ABCの本。</p>\n", 40)
	arabic := strings.Repeat("<p>كان يا ما كان في قديم الزمان، كانت هناك مدينة صغيرة على شاطئ البحر، فيها منارة عالية تضيء الليل.</p>\n", 40)
	paths := []string{
		epubtest.Write(t, wholeBook("ja", "body { writing-mode: vertical-rl }", japanese)),
		epubtest.Write(t, wholeBook("ar", "", arabic)),
	}
	fonts.AddDir("../../emulator/fonts") // the reader's fonts, as the runner adds them
	var report bytes.Buffer
	readWholeBooks(paths, &report)
	got := report.String()
	if strings.Count(got, "pages turn right to left") != 2 || strings.Count(got, "missing glyph: 0\n") != 2 ||
		strings.Count(got, "runes on no page: 0\n") != 2 || strings.Contains(got, "error") {
		t.Errorf("report:\n%s", got)
	}
}
