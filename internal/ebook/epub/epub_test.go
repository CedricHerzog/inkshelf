package epub

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/png"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/gen2brain/gav1d/avif"
	"golang.org/x/image/font/gofont/gobold"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/epubtest"
	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
)

const containerXML = `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`

// pngOf returns a PNG of size, as a string.
func pngOf(t *testing.T, size image.Point) string {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewGray(image.Rectangle{Max: size})); err != nil {
		t.Fatal(err)
	}
	return buffer.String()
}

// epub3 returns the files of an EPUB 3 book: a cover page (SVG), two chapters with a
// stylesheet, a nav and an NCX.
func epub3(t *testing.T) map[string]string {
	return map[string]string{
		"META-INF/container.xml": containerXML,
		"OEBPS/content.opf": `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title> Le Livre </dc:title></metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="css" href="css/style.css" media-type="text/css"/>
    <item id="cover" href="images/cover%20page.png" media-type="image/png" properties="cover-image"/>
    <item id="coverpage" href="text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="c1" href="text/c1.xhtml" media-type="application/xhtml+xml"/>
    <item id="notes" href="text/notes.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="text/c2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="coverpage"/><itemref idref="c1"/><itemref idref="notes" linear="no"/><itemref idref="c2"/>
  </spine>
</package>`,
		"OEBPS/css/style.css": `/* the book's style */
p { text-indent: 1.5em; margin: 0 }
p.first, .noindent { text-indent: 0 }
.center { text-align: center }
span.it { font-style: italic }
.hidden { display: none }
div.chapter h2 { page-break-before: always; font-weight: normal }
@media amzn-kf8 { p { margin: 3em } }`,
		"OEBPS/images/cover page.png": pngOf(t, image.Pt(600, 900)),
		"OEBPS/text/cover.xhtml": `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:xlink="http://www.w3.org/1999/xlink"><body>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 900"><image width="600" height="900" xlink:href="../images/cover%20page.png"/></svg>
</body></html>`,
		"OEBPS/text/c1.xhtml": `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Un</title>
<link rel="stylesheet" type="text/css" href="../css/style.css"/>
<style>.small { font-size: 80% }</style></head>
<body><div class="chapter">
<h1 id="one">Chapitre&nbsp;1</h1>
<p class="first">Il   était
 une <em>fois</em>, <span class="it">dit-il</span>&#160;: <b>fin</b>.</p>
<p>Deuxième<br/>ligne <span class="hidden">cachée</span>suite.</p>
<p id="here" class="center small">Centré</p>
<ul><li>un</li><li>deux</li></ul>
<ol><li>premier</li></ol>
<hr/>
<blockquote><p>Citation</p></blockquote>
<p><img src="../images/cover%20page.png" alt=""/></p>
<script>var ignored = 1;</script>
</div></body></html>`,
		"OEBPS/text/notes.xhtml": `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Note</p></body></html>`,
		"OEBPS/text/c2.xhtml": `<?xml version="1.0" encoding="iso-8859-1"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body><h2>Deux</h2><p>Caf` + "\xe9" + ` cr` + "\xe8" + `me.</p></body></html>`,
		"OEBPS/nav.xhtml": `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body>
<nav epub:type="toc"><ol>
  <li><a href="text/c1.xhtml">Chapitre <i>un</i></a><ol><li><a href="text/c1.xhtml#here">Ici</a></li></ol></li>
  <li><a href="text/c2.xhtml">Chapitre deux</a></li>
</ol></nav>
<nav epub:type="landmarks"><ol><li><a href="text/cover.xhtml">Cover</a></li></ol></nav>
</body></html>`,
		"OEBPS/toc.ncx": `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/"><navMap>
<navPoint><navLabel><text>NCX one</text></navLabel><content src="text/c1.xhtml"/></navPoint>
</navMap></ncx>`,
	}
}

// texts returns the text of each block of doc: its runs joined (an inline image as
// "[image path]"), "[image path]" or "[rule]".
func texts(doc *document.Document) []string {
	var all []string
	for _, block := range doc.Blocks {
		switch block.Kind {
		case document.Image:
			all = append(all, "[image "+block.Image+"]")
		case document.Rule:
			all = append(all, "[rule]")
		default:
			var text strings.Builder
			for _, run := range block.Runs {
				text.WriteString(run.Text)
				if run.Image != "" {
					text.WriteString("[image " + run.Image + "]")
				}
			}
			all = append(all, text.String())
		}
	}
	return all
}

func TestOpen(t *testing.T) {
	book, err := Open(epubtest.Write(t, epub3(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if book.Title != "Le Livre" || book.Chapters() != 3 {
		t.Errorf("title %q, %d chapters; want Le Livre, 3 (the notes are not linear)", book.Title, book.Chapters())
	}
	cover, err := book.Cover()
	if err != nil || !strings.HasPrefix(string(cover), "\x89PNG") {
		t.Errorf("cover: %v", err)
	}
}

func TestChapter(t *testing.T) {
	book, err := Open(epubtest.Write(t, epub3(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()

	cover, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := texts(cover); len(got) != 1 || got[0] != "[image OEBPS/images/cover page.png]" {
		t.Errorf("cover page %q", got)
	}

	doc, err := book.Chapter(1)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Chapitre 1",
		"Il était une fois, dit-il : fin.",
		"Deuxième", "ligne suite.",
		"Centré",
		"• un", "• deux", "1. premier",
		"[rule]",
		"Citation",
		"[image OEBPS/images/cover page.png]",
	}
	if got := texts(doc); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("blocks\n%q\nwant\n%q", got, want)
	}
	blocks := doc.Blocks
	heading, first, second, continued, centred := blocks[0], blocks[1], blocks[2], blocks[3], blocks[4]
	if heading.Kind != document.Heading || heading.Level != 1 || !heading.KeepWithNext || heading.Runs[0].Style != document.Bold {
		t.Errorf("heading %+v", heading)
	}
	if first.Indent != 0 || second.Indent != 150 || continued.Indent != 0 || continued.MarginTop != 0 || second.MarginBottom != 0 {
		t.Errorf("indents %d, %d, %d; continued margin %d, second's bottom %d", first.Indent, second.Indent, continued.Indent, continued.MarginTop, second.MarginBottom)
	}
	styles := map[string]document.Style{}
	for _, run := range first.Runs {
		styles[strings.TrimSpace(run.Text)] = run.Style
	}
	if styles["fois"] != document.Italic || styles["dit-il"] != document.Italic || styles["fin"] != document.Bold {
		t.Errorf("runs %+v", first.Runs)
	}
	if centred.Align != document.AlignCenter || centred.FontScale != 80 {
		t.Errorf("centred block %+v", centred)
	}
	if blocks[5].Left != 150 || blocks[9].Left != 150 {
		t.Errorf("list and quote margins %d, %d", blocks[5].Left, blocks[9].Left)
	}
	if offset, ok := doc.IDs["here"]; !ok || offset != centred.Start {
		t.Errorf("anchor at %d, %v; want %d", offset, ok, centred.Start)
	}
	// The document's length counts every rune of the text, and one for the rule and the image.
	length := 2
	for _, text := range want {
		if !strings.HasPrefix(text, "[") {
			length += len([]rune(text))
		}
	}
	if doc.Chars != length {
		t.Errorf("Chars = %d, want %d", doc.Chars, length)
	}

	latin1, err := book.Chapter(2)
	if err != nil {
		t.Fatal(err)
	}
	if got := texts(latin1); len(got) != 2 || got[1] != "Café crème." {
		t.Errorf("Latin-1 chapter %q", got)
	}
	if latin1.Blocks[0].Kind != document.Heading || latin1.Blocks[0].PageBreak {
		t.Errorf("h2 without a stylesheet: %+v", latin1.Blocks[0])
	}
}

func TestBuildIndex(t *testing.T) {
	book, err := Open(epubtest.Write(t, epub3(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	index, err := BuildIndex(book)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for chapter := range book.Chapters() {
		doc, err := book.Chapter(chapter)
		if err != nil {
			t.Fatal(err)
		}
		if index.Chapters[chapter].Chars != doc.Chars {
			t.Errorf("chapter %d: index says %d runes, the chapter has %d", chapter, index.Chapters[chapter].Chars, doc.Chars)
		}
		total += doc.Chars
	}
	if index.Total != total || index.Version != IndexVersion || index.Title != "Le Livre" {
		t.Errorf("index %+v", index)
	}
	doc, _ := book.Chapter(1)
	want := []TOCEntry{
		{Title: "Chapitre un", Level: 0, Chapter: 1},
		{Title: "Ici", Level: 1, Chapter: 1, Offset: doc.IDs["here"]},
		{Title: "Chapitre deux", Level: 0, Chapter: 2},
	}
	if len(index.TOC) != len(want) {
		t.Fatalf("TOC %+v, want %+v", index.TOC, want)
	}
	for position := range want {
		if index.TOC[position] != want[position] {
			t.Errorf("TOC entry %d: %+v, want %+v", position, index.TOC[position], want[position])
		}
	}
	if index.Chapters[1].Title != "Chapitre un" || index.Chapters[0].Title != "" {
		t.Errorf("chapter titles %+v", index.Chapters)
	}
}

// A chapter the table of contents leaves out is titled by its first heading.
func TestChapterTitleFromHeading(t *testing.T) {
	files := epub3(t)
	files["OEBPS/nav.xhtml"] = strings.Replace(files["OEBPS/nav.xhtml"], `<li><a href="text/c2.xhtml">Chapitre deux</a></li>`, "", 1)
	book, err := Open(epubtest.Write(t, files))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	index, err := BuildIndex(book)
	if err != nil {
		t.Fatal(err)
	}
	if index.Chapters[1].Title != "Chapitre un" || index.Chapters[2].Title != "Deux" || index.Chapters[0].Title != "" {
		t.Errorf("chapter titles %+v", index.Chapters)
	}
}

// Without a nav, the NCX gives the table of contents.
func TestNCX(t *testing.T) {
	files := epub3(t)
	files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"], ` properties="nav"`, "", 1)
	book, err := Open(epubtest.Write(t, files))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	index, err := BuildIndex(book)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.TOC) != 1 || index.TOC[0].Title != "NCX one" || index.TOC[0].Chapter != 1 {
		t.Errorf("TOC %+v", index.TOC)
	}
}

func TestDRM(t *testing.T) {
	files := epub3(t)
	files["META-INF/encryption.xml"] = `<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container" xmlns:enc="http://www.w3.org/2001/04/xmlenc#">
<enc:EncryptedData><enc:EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"/></enc:EncryptedData></encryption>`
	if _, err := Open(epubtest.Write(t, files)); !errors.Is(err, ErrDRM) {
		t.Errorf("Open = %v, want ErrDRM", err)
	}
	// Obfuscated fonts only are no DRM.
	files["META-INF/encryption.xml"] = strings.Replace(files["META-INF/encryption.xml"], "http://www.w3.org/2001/04/xmlenc#aes128-cbc", "http://www.idpf.org/2008/embedding", 1)
	book, err := Open(epubtest.Write(t, files))
	if err != nil {
		t.Fatal(err)
	}
	book.Close()
}

func TestMissingChapter(t *testing.T) {
	files := epub3(t)
	delete(files, "OEBPS/text/c2.xhtml")
	if _, err := Open(epubtest.Write(t, files)); err == nil || !strings.Contains(err.Error(), "c2.xhtml") {
		t.Errorf("Open = %v, want an error naming the missing chapter", err)
	}
}

// openPackage opens a book of files and of a package document with metadata, manifest items
// and a spine (written as XML, the spine from its attributes on) at OEBPS/content.opf. The
// book is closed with the test.
func openPackage(t testing.TB, packageAttributes, metadata, manifest, spine string, files map[string]string) *Book {
	t.Helper()
	files["META-INF/container.xml"] = containerXML
	files["OEBPS/content.opf"] = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" ` + packageAttributes + `>
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` + metadata + `</metadata>
  <manifest>` + manifest + `</manifest>
  <spine ` + spine + `</spine>
</package>`
	book, err := Open(epubtest.Write(t, files))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { book.Close() })
	return book
}

// xhtml returns an XHTML document of body.
func xhtml(body string) string {
	return `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body>` + body + `</body></html>`
}

// The first package document of the container is the book.
func TestFirstRootfile(t *testing.T) {
	files := epub3(t)
	files["META-INF/container.xml"] = `<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles>
<rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
<rootfile full-path="other.opf" media-type="application/oebps-package+xml"/>
</rootfiles></container>`
	files["other.opf"] = `<package xmlns="http://www.idpf.org/2007/opf"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Other</dc:title></metadata>
<manifest><item id="c" href="OEBPS/text/c2.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c"/></spine></package>`
	book, err := Open(epubtest.Write(t, files))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if book.Title != "Le Livre" {
		t.Errorf("title %q, want the first rootfile's", book.Title)
	}
}

func TestFallbacks(t *testing.T) {
	book := openPackage(t, "", "", `
<item id="pdf" href="part.pdf" media-type="application/pdf" fallback="dtbook"/>
<item id="dtbook" href="part.dtb" media-type="application/x-dtbook+xml" fallback="part"/>
<item id="part" href="part.xhtml" media-type="application/xhtml+xml"/>
<item id="loop1" href="loop1.bin" media-type="application/x-one" fallback="loop2"/>
<item id="loop2" href="loop2.bin" media-type="application/x-two" fallback="loop1"/>
<item id="figure" href="figure.tiff" media-type="image/tiff" fallback="figurepng"/>
<item id="figurepng" href="figure.png" media-type="image/png"/>
<item id="movie" href="movie.swf" media-type="application/x-shockwave-flash"/>`,
		`><itemref idref="pdf"/><itemref idref="loop1"/>`,
		map[string]string{
			"OEBPS/part.pdf":    "%PDF",
			"OEBPS/part.dtb":    "<dtbook/>",
			"OEBPS/loop1.bin":   "1",
			"OEBPS/loop2.bin":   "2",
			"OEBPS/figure.tiff": "II*",
			"OEBPS/figure.png":  pngOf(t, image.Pt(100, 100)),
			"OEBPS/movie.swf":   "FWS",
			"OEBPS/part.xhtml": xhtml(`<p>Text</p><img src="figure.tiff"/>` +
				`<object data="figure.tiff"><p>Not shown</p></object>` +
				`<object data="movie.swf"><p>Movie fallback</p></object>`),
		})
	if book.Chapters() != 2 || book.ChapterOfPath("OEBPS/part.pdf") != 0 {
		t.Fatalf("%d chapters, part.pdf is chapter %d", book.Chapters(), book.ChapterOfPath("OEBPS/part.pdf"))
	}
	part, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Text", "[image OEBPS/figure.png]", "[image OEBPS/figure.png]", "Movie fallback"}
	if got := texts(part); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("the PDF's fallback: %q, want %q", got, want)
	}
	// A cycle of unsupported items ends the chain.
	loop, err := book.Chapter(1)
	if err != nil {
		t.Fatal(err)
	}
	if got := texts(loop); len(got) != 1 || got[0] != unsupportedContent {
		t.Errorf("a cycle of fallbacks: %q", got)
	}
}

// A JPEG XL image is shown, not its fallback; an AVIF one gives way to its fallback while its
// decoder cannot run on the calling goroutine. Both are sized from their headers.
func TestJXLAndAVIFImages(t *testing.T) {
	jxl, err := os.ReadFile("../../graphics/pageimage/testdata/page.jxl") // 64×48
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := avif.Encode(&encoded, image.NewGray(image.Rect(0, 0, 40, 30))); err != nil {
		t.Fatal(err)
	}
	book := openPackage(t, "", "", `
<item id="part" href="part.xhtml" media-type="application/xhtml+xml"/>
<item id="photo" href="photo.jxl" media-type="image/jxl" fallback="photopng"/>
<item id="photopng" href="photo.png" media-type="image/png"/>
<item id="figure" href="figure.avif" media-type="image/avif" fallback="figurepng"/>
<item id="figurepng" href="figure.png" media-type="image/png"/>`,
		`><itemref idref="part"/>`,
		map[string]string{
			"OEBPS/photo.jxl":   string(jxl),
			"OEBPS/photo.png":   pngOf(t, image.Pt(100, 100)),
			"OEBPS/figure.avif": encoded.String(),
			"OEBPS/figure.png":  pngOf(t, image.Pt(100, 100)),
			"OEBPS/part.xhtml":  xhtml(`<p>Text</p><img src="photo.jxl"/><img src="figure.avif"/>`),
		})
	part, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Text", "[image OEBPS/photo.jxl]", "[image OEBPS/figure.png]"}
	if got := texts(part); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("images %q, want %q", got, want)
	}
	if size, err := book.ImageSize("OEBPS/photo.jxl"); err != nil || size != image.Pt(64, 48) {
		t.Errorf("ImageSize = %v, %v, want 64x48", size, err)
	}
	if size, err := book.ImageSize("OEBPS/figure.avif"); err != nil || size != image.Pt(40, 30) {
		t.Errorf("ImageSize = %v, %v, want 40x30", size, err)
	}
}

// A scripted document with a fallback is left for it: the app runs no scripts.
func TestScriptedFallback(t *testing.T) {
	book := openPackage(t, "", "", `
<item id="scripted" href="scripted.xhtml" media-type="application/xhtml+xml" properties="scripted" fallback="plain"/>
<item id="plain" href="plain.xhtml" media-type="application/xhtml+xml"/>
<item id="alone" href="alone.xhtml" media-type="application/xhtml+xml" properties="scripted"/>`,
		`><itemref idref="scripted"/><itemref idref="alone"/>`,
		map[string]string{
			"OEBPS/scripted.xhtml": xhtml("<p>Needs scripts</p>"),
			"OEBPS/plain.xhtml":    xhtml("<p>Fallback</p>"),
			"OEBPS/alone.xhtml":    xhtml("<p>No fallback</p>"),
		})
	for chapter, want := range []string{"Fallback", "No fallback"} {
		doc, err := book.Chapter(chapter)
		if err != nil {
			t.Fatal(err)
		}
		if got := texts(doc); len(got) != 1 || got[0] != want {
			t.Errorf("chapter %d: %q, want %q", chapter, got, want)
		}
	}
	if book.ChapterOfPath("OEBPS/scripted.xhtml") != 0 {
		t.Error("links to the scripted document no longer lead to its chapter")
	}
}

// Paths in the book resolve as URLs do: ".." at the root stays there.
func TestResolve(t *testing.T) {
	for _, test := range []struct{ base, href, want string }{
		{"EPUB/text/c1.xhtml", "../images/a%20b.png#frag", "EPUB/images/a b.png"},
		{"EPUB/c1.xhtml", "../../../media/photo.jpg", "media/photo.jpg"},
		{"EPUB/c1.xhtml", "/media/photo.jpg", "media/photo.jpg"},
		{"c1.xhtml", "./photo.jpg", "photo.jpg"},
	} {
		if got := resolve(test.base, test.href); got != test.want {
			t.Errorf("resolve(%q, %q) = %q, want %q", test.base, test.href, got, test.want)
		}
	}
}

// Pages turn as page-progression-direction says, else as the language is written.
func TestRightToLeft(t *testing.T) {
	for _, test := range []struct {
		progression, language string
		want                  bool
	}{
		{"", "en", false}, {"rtl", "en", true}, {"", "ar", true}, {"", "he-IL", true}, {"ltr", "fa", false}, {"", "", false},
	} {
		index := Index{PageProgression: test.progression, Language: test.language}
		if got := index.RightToLeft(); got != test.want {
			t.Errorf("progression %q, language %q: right to left %v, want %v", test.progression, test.language, got, test.want)
		}
	}
}

// A non-linear item is no chapter, but a link reaches it.
func TestNonLinear(t *testing.T) {
	book, err := Open(epubtest.Write(t, epub3(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if notes := book.ChapterOfPath("OEBPS/text/notes.xhtml"); notes != -1 || book.ChapterOfPath("OEBPS/text/c2.xhtml") != 2 {
		t.Errorf("notes in chapter %d, c2.xhtml in %d", notes, book.ChapterOfPath("OEBPS/text/c2.xhtml"))
	}
	notes, err := book.Document("OEBPS/text/notes.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	if got := texts(notes); len(got) != 1 || got[0] != "Note" {
		t.Errorf("notes %q", got)
	}
	if _, err := book.Document("OEBPS/css/style.css"); err == nil {
		t.Error("Document of a file out of the spine: no error")
	}
}

// The package's metadata, whitespace collapsed. The first rendition:layout counts, of the
// package and of an itemref.
func TestMetadata(t *testing.T) {
	book := openPackage(t, `unique-identifier="uid"`, `
<dc:identifier id="isbn">urn:isbn:123</dc:identifier>
<dc:identifier id="uid">  urn:uuid:abc
  </dc:identifier>
<dc:title>  A   long
  title </dc:title>
<dc:language> fr </dc:language><dc:language>en</dc:language>
<dc:creator>Jean   Dupont</dc:creator><dc:creator>Marie Curie</dc:creator>
<meta property="rendition:layout">pre-paginated</meta>
<meta property="rendition:layout">reflowable</meta>
<meta property="rendition:spread">none</meta>`,
		`<item id="one" href="one.xhtml" media-type="application/xhtml+xml"/><item id="two" href="two.xhtml" media-type="application/xhtml+xml"/>`,
		`page-progression-direction="rtl"><itemref idref="one" properties="page-spread-left rendition:layout-reflowable rendition:layout-pre-paginated"/><itemref idref="two"/>`,
		map[string]string{"OEBPS/one.xhtml": xhtml("<p>One</p>"), "OEBPS/two.xhtml": xhtml("<p>Two</p>")})
	if book.Title != "A long title" || book.Language != "fr" || book.Identifier != "urn:uuid:abc" || strings.Join(book.Creators, "|") != "Jean Dupont|Marie Curie" {
		t.Errorf("title %q, language %q, identifier %q, creators %q", book.Title, book.Language, book.Identifier, book.Creators)
	}
	if book.PageProgression != "rtl" || book.Rendition.Layout != PrePaginated {
		t.Errorf("page progression %q, layout %q", book.PageProgression, book.Rendition.Layout)
	}
	if book.ChapterLayout(0) != Reflowable || book.ChapterLayout(1) != PrePaginated {
		t.Errorf("chapter layouts %q, %q", book.ChapterLayout(0), book.ChapterLayout(1))
	}
	index, err := BuildIndex(book)
	if err != nil {
		t.Fatal(err)
	}
	if index.Language != "fr" || index.PageProgression != "rtl" {
		t.Errorf("index language %q, page progression %q", index.Language, index.PageProgression)
	}
}

// The navigation document's page list and landmarks; the toc is the first nav of its type.
func TestPageListAndLandmarks(t *testing.T) {
	files := epub3(t)
	files["OEBPS/nav.xhtml"] = xhtml(`
<nav epub:type="landmarks"><ol><li><a epub:type="cover" href="text/cover.xhtml">Cover</a></li>
  <li><a epub:type="bodymatter" href="text/c1.xhtml#here">Start</a></li></ol></nav>
<nav><ol><li><a href="text/c2.xhtml">Untyped</a></li></ol></nav>
<nav epub:type="toc"><ol><li><a href="text/c1.xhtml">One</a></li><li><a href="text/c2.xhtml"><img src="two.png" alt="Two"/></a></li></ol></nav>
<nav epub:type="toc"><ol><li><a href="text/c2.xhtml">Second toc</a></li></ol></nav>
<nav epub:type="page-list" hidden=""><ol><li><a href="text/c1.xhtml#one">1</a></li><li><a href="text/notes.xhtml">2</a></li><li><a href="text/c2.xhtml">3</a></li></ol></nav>`)
	book, err := Open(epubtest.Write(t, files))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	index, err := BuildIndex(book)
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := book.Chapter(1)
	// An image's label is its alt text.
	if len(index.TOC) != 2 || index.TOC[0].Title != "One" || index.TOC[1].Title != "Two" {
		t.Errorf("TOC %+v", index.TOC)
	}
	// The page in the non-linear notes is left out.
	wantPages := []TOCEntry{{Title: "1", Chapter: 1, Offset: doc.IDs["one"]}, {Title: "3", Chapter: 2}}
	if !slices.Equal(index.PageList, wantPages) {
		t.Errorf("page list %+v, want %+v", index.PageList, wantPages)
	}
	wantLandmarks := []TOCEntry{{Title: "Cover", Chapter: 0, Type: "cover"}, {Title: "Start", Chapter: 1, Offset: doc.IDs["here"], Type: "bodymatter"}}
	if !slices.Equal(index.Landmarks, wantLandmarks) {
		t.Errorf("landmarks %+v, want %+v", index.Landmarks, wantLandmarks)
	}
}

// What stands in for scripts, media and MathML past the limits of its layout.
func TestFallbackContent(t *testing.T) {
	tooDeep := strings.Repeat("<mrow>", 60) + "<mi>x</mi>" + strings.Repeat("</mrow>", 60)
	book := openPackage(t, "", "", `
<item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/>
<item id="poster" href="poster.png" media-type="image/png"/>
<item id="formula" href="formula.png" media-type="image/png"/>`,
		`><itemref idref="chapter"/>`,
		map[string]string{
			"OEBPS/poster.png":  pngOf(t, image.Pt(10, 10)),
			"OEBPS/formula.png": pngOf(t, image.Pt(10, 10)),
			"OEBPS/chapter.xhtml": xhtml(`<script>run()</script><noscript><p>No script</p></noscript>
<video poster="poster.png" src="movie.mp4"><p>Video fallback</p></video>
<video><source src="movie.mp4"/><track src="subtitles.vtt"/><p>Video text</p></video>
<audio src="song.mp3"><p>Audio text</p></audio>
<object data="applet.class"><param name="speed" value="1"/><p>Object text</p></object>
<p>Area <math altimg="formula.png">` + tooDeep + `</math></p>
<p>So <math alttext="x squared">` + tooDeep + `</math>.</p>
<p>Then <math>` + tooDeep + `<mi>y</mi></math>.</p>`),
		})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"No script", "[image OEBPS/poster.png]", "Video text", "Audio text", "Object text",
		"Area [image OEBPS/formula.png]", "So x squared.", "Then xy.",
	}
	if got := texts(doc); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("blocks\n%q\nwant\n%q", got, want)
	}
	if runs := doc.Blocks[7].Runs; len(runs) != 3 || strings.TrimSpace(runs[1].Text) != "xy" || runs[1].Style != document.Italic {
		t.Errorf("MathML's tokens: %+v, want xy in italics", runs)
	}
}

// MathML is a formula, laid out with the page: an image run in its line, or a block of its own
// if its display is block; laid out before its image or its text alternative.
func TestMathML(t *testing.T) {
	book := openPackage(t, "", "", `<item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/>`,
		`><itemref idref="chapter"/>`,
		map[string]string{
			"OEBPS/chapter.xhtml": xhtml(`<p>So <a href="#n"><math alttext="x squared" altimg="formula.png"><msup><mi>x</mi><mn>2</mn></msup></math></a>, and
<math display="block"><mfrac><mn>1</mn><mi>n</mi></mfrac><mo>&InvisibleTimes;</mo><mi>&alpha;</mi></math>
<math><mi>y</mi></math></p>`),
		})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"So [image mathml:<math><msup><mi>x</mi><mn>2</mn></msup></math>], and",
		"[image mathml:<math display=\"block\"><mfrac><mn>1</mn><mi>n</mi></mfrac><mo>\u2062</mo><mi>α</mi></math>]",
		// A formula alone in its paragraph stays a line of text.
		"[image mathml:<math><mi>y</mi></math>]",
	}
	if got := texts(doc); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("blocks\n%q\nwant\n%q", got, want)
	}
	runs := doc.Blocks[0].Runs
	if run := runs[slices.IndexFunc(runs, func(run document.Run) bool { return run.Image != "" })]; run.Link != "OEBPS/chapter.xhtml#n" || run.Style != 0 {
		t.Errorf("formula's run %+v, want in its link, in no style", run)
	}
	// An image counts one rune, as BuildIndex measures it.
	if doc.Chars != len([]rune("So "))+1+len([]rune(", and"))+1+1 {
		t.Errorf("%d runes", doc.Chars)
	}
}

// Word's Symbol-font bullet, a private-use character, is shown as •.
func TestSymbolBullet(t *testing.T) {
	book := openPackage(t, "", "", `<item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/>`,
		`><itemref idref="chapter"/>`,
		map[string]string{
			"OEBPS/chapter.xhtml": xhtml("<p><span style=\"font-family: Symbol\">\uF0B7</span> First</p><p>&#xF0B7;\u00a0Second</p>"),
		})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := texts(doc), []string{"• First", "•\u00a0Second"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("blocks %q, want %q", got, want)
	}
}

// An <svg> of shapes or text in a chapter is an image; one with a raster image shows it as the
// chapter's content.
func TestInlineSVG(t *testing.T) {
	book := openPackage(t, "", "", `<item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/>`,
		`><itemref idref="chapter"/>`,
		map[string]string{
			"OEBPS/picture.png": pngOf(t, image.Pt(800, 600)),
			"OEBPS/chapter.xhtml": xhtml(`<p>Before</p>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 10" style="width: 50%"><g><rect width="20" height="10"/></g></svg>
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><image xlink:href="picture.png"/></svg>
<svg xmlns="http://www.w3.org/2000/svg"><text>Title</text></svg>
<p>After</p>`),
		})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	got := texts(doc)
	if len(got) != 5 || got[0] != "Before" || !strings.HasPrefix(got[1], "[image data:image/svg+xml;base64,") ||
		got[2] != "[image OEBPS/picture.png]" || !strings.HasPrefix(got[3], "[image data:image/svg+xml;base64,") || got[4] != "After" {
		t.Fatalf("blocks %q", got)
	}
	drawing := doc.Blocks[1]
	markup, err := uri.DecodeData(drawing.Image)
	if err != nil || !strings.HasPrefix(string(markup), "<svg") || !strings.HasSuffix(string(markup), "</svg>") {
		t.Errorf("the SVG's markup %q, %v", markup, err)
	}
	if size, err := book.ImageSize(drawing.Image); err != nil || size != image.Pt(20, 10) || drawing.ImageWidth != 50 {
		t.Errorf("the SVG %v (%v), %d%% wide; want 20x10, 50%%", size, err, drawing.ImageWidth)
	}
}

// The chapter's stylesheets reach an inline <svg>'s elements, through the attribute the SVG
// rasterizer reads them from; the markup is otherwise as written.
func TestInlineSVGDocumentStyle(t *testing.T) {
	book := openPackage(t, "", "", `<item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/>`,
		`><itemref idref="chapter"/>`,
		map[string]string{
			"OEBPS/chapter.xhtml": `<html xmlns="http://www.w3.org/1999/xhtml"><head><style>
div.figure svg .dark { fill: gray; margin: 0 } svg { color: #333 } circle { display: none }
</style></head><body><div class="figure"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 2 1">` +
				`<rect class="dark" width="1" height="1"/><circle r="1"></circle><rect x="1" width="1" height="1"/></svg></div></body></html>`,
		})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Blocks) != 1 || !uri.IsData(doc.Blocks[0].Image) {
		t.Fatalf("blocks %q, want the SVG as an image", texts(doc))
	}
	markup, err := uri.DecodeData(doc.Blocks[0].Image)
	if err != nil {
		t.Fatal(err)
	}
	want := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 2 1" data-inkshelf-css="color: #333">` +
		`<rect class="dark" width="1" height="1" data-inkshelf-css="fill: gray"/><circle r="1" data-inkshelf-css="display: none"></circle>` +
		`<rect x="1" width="1" height="1"/></svg>`
	if string(markup) != want {
		t.Errorf("markup\n%s\nwant\n%s", markup, want)
	}
}

func TestDataImage(t *testing.T) {
	source := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte(pngOf(t, image.Pt(4, 2))))
	book := openPackage(t, "", "", `<item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/>`,
		`><itemref idref="chapter"/>`,
		map[string]string{"OEBPS/chapter.xhtml": xhtml(`<p><img src="` + source + `"/></p>`)})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Blocks) != 1 || doc.Blocks[0].Image != source {
		t.Fatalf("blocks %+v, want the data: URI as an image", doc.Blocks)
	}
	if size, err := book.ImageSize(source); err != nil || size != image.Pt(4, 2) {
		t.Errorf("ImageSize = %v, %v, want 4x2", size, err)
	}
}

// Nothing a book carries runs, and nothing is fetched: a script and an event handler leave the
// static text around them and add none of their own; a remote image is a missing one, its text
// alternative kept; a remote font is skipped for the next source; an iframe, a form and a
// WebAssembly object load nothing.
func TestNothingRunsOrIsFetched(t *testing.T) {
	book := openStyled(t, `
@font-face { font-family: Remote; src: url(https://fonts.example.com/remote.ttf) }
@font-face { font-family: Mirrored; src: url(https://fonts.example.com/mirrored.ttf), url(fonts/mirrored.ttf) }
.remote { font-family: Remote, sans-serif } .mirrored { font-family: Mirrored, serif }`,
		`<p>Before <script>document.write("Injected")</script>after.</p>
<p onclick="alert('Clicked')">Click <a href="javascript:alert(1)" onmouseover="steal()">here</a>.</p>
<p><img src="https://images.example.com/lighthouse.png" alt="A lighthouse
  on a rock"/></p>
<p class="remote">Remote font</p><p class="mirrored">Mirrored font</p>
<iframe src="other.xhtml"></iframe>
<form action="https://example.com/send"><input type="text" value="Typed"/><button>Send</button></form>
<object type="application/wasm" data="module.wasm"><p>No module</p></object>`,
		map[string]string{
			"OEBPS/other.xhtml":        xhtml("<p>Framed</p>"),
			"OEBPS/module.wasm":        "\x00asm",
			"OEBPS/fonts/mirrored.ttf": string(gobold.TTF),
		})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Join(texts(doc), "|")
	for _, own := range []string{"Injected", "Clicked", "alert", "Framed", "Typed", "asm"} {
		if strings.Contains(all, own) {
			t.Errorf("%q shows in %q", own, all)
		}
	}
	for _, static := range []string{"Before after.", "Click here.", "Remote font", "Mirrored font", "No module"} {
		if !strings.Contains(all, static) {
			t.Errorf("%q missing from %q", static, all)
		}
	}

	index := slices.IndexFunc(doc.Blocks, func(block document.Block) bool { return block.Kind == document.Image })
	if index < 0 {
		t.Fatalf("no image block in %q", all)
	}
	remote := doc.Blocks[index]
	if remote.Alt != "A lighthouse on a rock" {
		t.Errorf("the remote image's text alternative %q", remote.Alt)
	}
	if _, err := book.Resource(remote.Image); err == nil || strings.Contains(remote.Image, "://") {
		t.Errorf("the remote image %q can be read (%v): want a missing image", remote.Image, err)
	}

	if doc.Fonts["remote"] != nil || runOf(t, doc, "Remote font").Family != "sans-serif" {
		t.Errorf("a remote font was used: fonts %v", doc.Fonts)
	}
	if doc.Fonts["mirrored"][fonts.Regular] == nil || runOf(t, doc, "Mirrored font").Family != "mirrored" {
		t.Errorf("the book's own source after a remote one was not used: fonts %v", doc.Fonts)
	}
}

// An image's text alternative is kept, not counted in runes: <img alt>, an inline SVG's own
// <title>, MathML's alttext.
func TestImageAlt(t *testing.T) {
	book := openPackage(t, "", "", `
<item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/>
<item id="picture" href="picture.png" media-type="image/png"/>`,
		`><itemref idref="chapter"/>`,
		map[string]string{
			"OEBPS/picture.png": pngOf(t, image.Pt(400, 300)),
			"OEBPS/chapter.xhtml": xhtml(`<p><img src="picture.png" alt=" A  harbour "/></p>
<p>Small <img src="picture.png" alt="an icon" style="height: 1em"/> inline.</p>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 10"><title>A red <tspan>square</tspan></title><g><title>Not this</title><rect width="20" height="10"/></g></svg>
<p>So <math alttext="x squared"><msup><mi>x</mi><mn>2</mn></msup></math>.</p>
<math display="block" alttext="one over n"><mfrac><mn>1</mn><mi>n</mi></mfrac></math>`),
		})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	var alts []string
	for _, block := range doc.Blocks {
		if block.Kind == document.Image {
			alts = append(alts, block.Alt)
		}
		for _, run := range block.Runs {
			if run.Image != "" {
				alts = append(alts, run.Alt)
			}
		}
	}
	want := []string{"A harbour", "an icon", "A red square", "x squared", "one over n"}
	if strings.Join(alts, "|") != strings.Join(want, "|") {
		t.Errorf("text alternatives %q, want %q", alts, want)
	}
	// Each image counts one rune, its text alternative none.
	if want := 1 + len([]rune("Small ")) + 1 + len([]rune(" inline.")) + 1 + len([]rune("So ")) + 1 + 1 + 1; doc.Chars != want {
		t.Errorf("%d runes, want %d", doc.Chars, want)
	}
}

// Audio and video never play: a video shows its poster when the book holds it, else its
// fallback content, as does audio; their sources and tracks show nothing.
func TestMediaElements(t *testing.T) {
	book := openPackage(t, "", "", `
<item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/>
<item id="poster" href="poster.png" media-type="image/png"/>
<item id="movie" href="movie.mp4" media-type="video/mp4"/>
<item id="song" href="song.mp3" media-type="audio/mpeg"/>`,
		`><itemref idref="chapter"/>`,
		map[string]string{
			"OEBPS/poster.png": pngOf(t, image.Pt(400, 300)),
			"OEBPS/movie.mp4":  "not a movie",
			"OEBPS/song.mp3":   "not a song",
			"OEBPS/chapter.xhtml": xhtml(`<video poster="poster.png" src="movie.mp4" controls="controls"><p>Poster fallback</p></video>
<video poster="https://videos.example.com/still.png"><source src="movie.mp4" type="video/mp4"/><p>Remote poster fallback</p></video>
<video poster="missing.png" src="movie.mp4"><p>Missing poster fallback</p></video>
<video src="movie.mp4"><track kind="captions" src="captions.vtt"/><p>No poster fallback</p></video>
<audio controls="controls"><source src="song.mp3" type="audio/mpeg"/><p>Audio fallback</p></audio>
<audio src="song.mp3"/>
<p>After</p>`),
		})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"[image OEBPS/poster.png]", "Remote poster fallback", "Missing poster fallback", "No poster fallback", "Audio fallback", "After"}
	if got := texts(doc); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("blocks\n%q\nwant\n%q", got, want)
	}
}

// A media overlay (read-aloud) is ignored: a book with one builds the same documents as without.
func TestMediaOverlayIgnored(t *testing.T) {
	chapter := xhtml(`<p id="s1">The first sentence.</p><p id="s2">The second one.</p>`)
	smil := `<smil xmlns="http://www.w3.org/ns/SMIL" xmlns:epub="http://www.idpf.org/2007/ops" version="3.0"><body><seq epub:textref="chapter.xhtml">
<par id="p1"><text src="chapter.xhtml#s1"/><audio src="voice.mp3" clipBegin="0s" clipEnd="2s"/></par>
<par id="p2"><text src="chapter.xhtml#s2"/><audio src="voice.mp3" clipBegin="2s" clipEnd="4s"/></par>
</seq></body></smil>`
	plain := openPackage(t, "", "", `<item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/>`,
		`><itemref idref="chapter"/>`, map[string]string{"OEBPS/chapter.xhtml": chapter})
	overlaid := openPackage(t, "", `<meta property="media:duration">0:00:04</meta><meta property="media:active-class">-epub-media-overlay-active</meta>`, `
<item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml" media-overlay="overlay"/>
<item id="overlay" href="chapter.smil" media-type="application/smil+xml"/>
<item id="voice" href="voice.mp3" media-type="audio/mpeg"/>`,
		`><itemref idref="chapter"/>`,
		map[string]string{"OEBPS/chapter.xhtml": chapter, "OEBPS/chapter.smil": smil, "OEBPS/voice.mp3": "not a voice"})
	if plain.Chapters() != overlaid.Chapters() {
		t.Fatalf("%d chapters with the overlay, %d without", overlaid.Chapters(), plain.Chapters())
	}
	want, err := plain.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := overlaid.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(texts(got), "|") != strings.Join(texts(want), "|") || got.Chars != want.Chars {
		t.Errorf("with the overlay %q (%d runes), want %q (%d runes)", texts(got), got.Chars, texts(want), want.Chars)
	}
}
