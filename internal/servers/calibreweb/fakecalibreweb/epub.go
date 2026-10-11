package fakecalibreweb

import (
	"fmt"
	"html"
	"strings"
)

// makeEPUB returns the EPUB of a book, which shows off as much of the reader's layout as a few
// chapters can: EPUB 3 with an EPUB 2 NCX too, a stylesheet with a class, chapters with a
// heading, italic and bold, French spaces (&nbsp; under an XHTML doctype), soft hyphens, an
// illustration in the first chapter and an SVG one in the second, a drop cap and an image
// floated at the right of the text in the second, a scene break of ◆, a navigation document
// with nested entries, printed pages and a landmark, links in the first chapter (to a
// footnote, to the last chapter, into the notes, a non-linear item of the spine, and to the
// web), and a last chapter of MathML.
func makeEPUB(item book) ([]byte, error) {
	plate, err := drawPlate()
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{
		"META-INF/container.xml":  []byte(containerXML),
		"OEBPS/style.css":         []byte(styleCSS),
		"OEBPS/images/plate.png":  plate,
		"OEBPS/images/figure.svg": []byte(figureSVG),
		"OEBPS/content.opf":       []byte(contentOPF(item)),
		"OEBPS/toc.ncx":           []byte(tocNCX(item)),
		"OEBPS/nav.xhtml":         []byte(navXHTML(item)),
		"OEBPS/notes.xhtml":       []byte(notesXHTML),
		formulasFile:              []byte(formulasXHTML),
	}
	for number := range item.Chapters {
		files[chapterFile(number+1)] = []byte(chapterXHTML(item, number+1))
	}
	// The mimetype comes first and is stored, as EPUB requires.
	return zipFiles([]storedFile{{"mimetype", []byte("application/epub+zip")}}, files)
}

// containerXML is the file every EPUB starts from, which points to its package document.
const containerXML = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`

// styleCSS is the book's stylesheet: indented paragraphs, and the classes the chapters use
// for their box, drop cap, floated image and SVG emblem.
const styleCSS = `body { margin: 0; }
p { margin: 0; text-indent: 1.5em; }
p.first { text-indent: 0; }
h1 { text-align: center; margin: 2em 0 1em; }
.centre { text-align: center; }
a { text-decoration: underline; }
svg.emblem .disc { fill: #888; }
.encadre { border: 3px double black; background: #e0e0e0; padding: 0.5em 1em; margin: 1em 0; }
.gris { color: #555; }
.lettrine { float: left; font-size: 3.2em; line-height: 1; margin-right: 0.05em; }
img.vignette { float: right; width: 35%; margin: 0 0 0.5em 1em; }
.emblem text { font-size: 22px; font-weight: bold; text-anchor: middle; }
`

// chapterFile returns the path of chapter number in the EPUB.
func chapterFile(number int) string { return fmt.Sprintf("OEBPS/text/chapter%02d.xhtml", number) }

// contentOPF returns the package document of a book.
func contentOPF(item book) string {
	var manifest, spine strings.Builder
	for number := range item.Chapters {
		fmt.Fprintf(&manifest, "    <item id=\"chapter%02d\" href=\"text/chapter%02d.xhtml\" media-type=\"application/xhtml+xml\"/>\n", number+1, number+1)
		fmt.Fprintf(&spine, "    <itemref idref=\"chapter%02d\"/>\n", number+1)
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:calibre:%d</dc:identifier>
    <dc:title>%s</dc:title>
    <dc:creator>%s</dc:creator>
    <dc:language>fr</dc:language>
    <meta property="dcterms:modified">2026-10-01T12:00:00Z</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="style" href="style.css" media-type="text/css"/>
    <item id="plate" href="images/plate.png" media-type="image/png"/>
    <item id="figure" href="images/figure.svg" media-type="image/svg+xml"/>
    <item id="notes" href="notes.xhtml" media-type="application/xhtml+xml"/>
    <item id="formulas" href="text/formulas.xhtml" media-type="application/xhtml+xml"/>
%s  </manifest>
  <spine toc="ncx">
%s    <itemref idref="formulas"/>
    <itemref idref="notes" linear="no"/>
  </spine>
</package>
`, item.ID, html.EscapeString(item.Title), html.EscapeString(item.Author), manifest.String(), spine.String())
}

// chapterTitle returns the title of chapter number.
func chapterTitle(number int) string { return fmt.Sprintf("Chapitre %d", number) }

// printedPagesPerChapter is the number of printed pages of a chapter.
const printedPagesPerChapter = (chapterParagraphs + printedPageParagraphs - 1) / printedPageParagraphs

// tocNCX returns the EPUB 2 table of contents of a book: the chapters, each with its second
// scene under it.
func tocNCX(item book) string {
	var points strings.Builder
	for number := range item.Chapters {
		fmt.Fprintf(&points, `    <navPoint id="c%d"><navLabel><text>%s</text></navLabel><content src="text/chapter%02d.xhtml"/>
      <navPoint id="s%d"><navLabel><text>Scène 2</text></navLabel><content src="text/chapter%02d.xhtml#scene2"/></navPoint>
    </navPoint>
`, number+1, chapterTitle(number+1), number+1, number+1, number+1)
	}
	fmt.Fprintf(&points, "    <navPoint id=\"formulas\"><navLabel><text>%s</text></navLabel><content src=\"text/formulas.xhtml\"/></navPoint>\n", formulasTitle)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <head><meta name="dtb:uid" content="urn:calibre:%d"/></head>
  <docTitle><text>%s</text></docTitle>
  <navMap>
%s  </navMap>
</ncx>
`, item.ID, html.EscapeString(item.Title), points.String())
}

// navXHTML returns the EPUB 3 navigation document of a book: its table of contents (the
// chapters, each with its second scene under it), its printed pages and a landmark.
func navXHTML(item book) string {
	var entries, pages strings.Builder
	for number := range item.Chapters {
		fmt.Fprintf(&entries, "      <li><a href=\"text/chapter%02d.xhtml\">%s</a>\n", number+1, chapterTitle(number+1))
		fmt.Fprintf(&entries, "        <ol><li><a href=\"text/chapter%02d.xhtml#scene2\">Scène 2</a></li></ol></li>\n", number+1)
		for page := range printedPagesPerChapter {
			printed := number*printedPagesPerChapter + page + 1
			fmt.Fprintf(&pages, "      <li><a href=\"text/chapter%02d.xhtml#page%d\">%d</a></li>\n", number+1, printed, printed)
		}
	}
	fmt.Fprintf(&entries, "      <li><a href=\"text/formulas.xhtml\">%s</a></li>\n", formulasTitle)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>%s</title></head>
<body>
  <nav epub:type="toc">
    <ol>
%s    </ol>
  </nav>
  <nav epub:type="page-list" hidden="">
    <ol>
%s    </ol>
  </nav>
  <nav epub:type="landmarks" hidden="">
    <ol>
      <li><a epub:type="bodymatter" href="text/chapter01.xhtml">Début du texte</a></li>
    </ol>
  </nav>
</body>
</html>
`, html.EscapeString(item.Title), entries.String(), pages.String())
}
