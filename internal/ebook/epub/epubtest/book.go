package epubtest

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// Options describe the book that Files writes. A zero field takes its default.
type Options struct {
	// Title is the book's title, "A Test Book" by default.
	Title string
	// Chapters is the number of chapters, 10 by default; of pages for a fixed-layout book.
	Chapters int
	// Paragraphs is the number of paragraphs of each chapter, 40 by default: with Words, about
	// the 20,000 characters of a novel's chapter.
	Paragraphs int
	// Words is the number of words of each paragraph, 80 by default.
	Words int
	// FixedLayout makes each chapter a pre-paginated page (rendition:layout pre-paginated) of
	// FixedPageSize: the illustration over the whole page, the SVG figure placed in a corner
	// and a paragraph placed at its bottom. Paragraphs and Words are then unused.
	FixedLayout bool
}

// The paths of the files of every book of Files, in the zip.
const (
	ContainerPath  = "META-INF/container.xml"
	PackagePath    = "OEBPS/content.opf"
	NavPath        = "OEBPS/nav.xhtml"
	NCXPath        = "OEBPS/toc.ncx"
	StylesheetPath = "OEBPS/style.css"
	CoverPath      = "OEBPS/images/cover.png"
	// IllustrationPath is a raster image: a figure of a reflowable book's chapters, the picture
	// that fills a fixed-layout book's pages.
	IllustrationPath = "OEBPS/images/illustration.png"
	FigurePath       = "OEBPS/images/figure.svg"
)

// FixedPageSize is the viewport of a fixed-layout book's pages, in CSS pixels: a comic's page.
var FixedPageSize = image.Pt(1200, 1800)

// IllustrationEvery is how often a reflowable chapter has the illustration, the SVG figure
// and the MathML formula: the first chapter and every IllustrationEvery-th after it, as a
// novel has a few plates.
const IllustrationEvery = 10

// ChapterPath returns the path in the zip of chapter number, counted from 1.
func ChapterPath(number int) string { return fmt.Sprintf("OEBPS/text/chapter%03d.xhtml", number) }

// withDefaults returns options with each zero field at its default.
func (options Options) withDefaults() Options {
	if options.Title == "" {
		options.Title = "A Test Book"
	}
	if options.Chapters <= 0 {
		options.Chapters = 10
	}
	if options.Paragraphs <= 0 {
		options.Paragraphs = 40
	}
	if options.Words <= 0 {
		options.Words = 80
	}
	return options
}

// Files returns the files of a book as options describe it, by path in the zip, for Zip or
// Write; a test may change or add files before.
func Files(tb testing.TB, options Options) map[string]string {
	tb.Helper()
	options = options.withDefaults()
	illustrationSize := image.Pt(800, 600)
	if options.FixedLayout {
		illustrationSize = FixedPageSize
	}
	files := map[string]string{
		ContainerPath:    containerXML,
		PackagePath:      packageDocument(options),
		NavPath:          navigationDocument(options),
		NCXPath:          ncx(options),
		StylesheetPath:   stylesheet,
		CoverPath:        pictureOf(tb, image.Pt(600, 900)),
		IllustrationPath: pictureOf(tb, illustrationSize),
		FigurePath:       figureSVG,
	}
	words := newWordSource(1)
	for index := range options.Chapters {
		number := index + 1
		if options.FixedLayout {
			files[ChapterPath(number)] = fixedPage(number, words)
		} else {
			files[ChapterPath(number)] = chapter(options, number, words)
		}
	}
	return files
}

// containerXML points to the package document.
const containerXML = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`

// stylesheet is the book's: indented paragraphs, centred headings and figures, and the boxes
// of a fixed-layout page.
const stylesheet = `body { margin: 0 }
p { margin: 0; text-indent: 1.5em; text-align: justify }
p.first { text-indent: 0 }
h1 { text-align: center; font-size: 1.6em; margin: 2em 0 1em; page-break-before: always }
.centre { text-align: center; text-indent: 0; margin: 1em 0 }
figure { margin: 1em 0; text-align: center }
figure img { max-width: 100% }
blockquote { margin: 1em 2em; font-style: italic }
aside { font-size: 0.8em }
.caption { position: absolute; left: 100px; top: 1450px; width: 1000px; height: 300px; font-size: 40px }
.figure { position: absolute; right: 60px; top: 60px; width: 300px; height: 300px }
`

// figureSVG is the SVG figure: shapes, a gradient and text, as an SVG diagram has.
const figureSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="400" height="300" viewBox="0 0 400 300">
<defs><linearGradient id="shade" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#fff"/><stop offset="1" stop-color="#444"/></linearGradient></defs>
<rect x="10" y="10" width="380" height="280" rx="20" fill="url(#shade)" stroke="black" stroke-width="4"/>
<circle cx="120" cy="150" r="70" fill="#888"/>
<path d="M 220 220 L 300 80 L 360 220 Z" fill="none" stroke="black" stroke-width="6"/>
<text x="200" y="280" font-size="24" text-anchor="middle">Figure</text>
</svg>`

// formula is the MathML formula of an illustrated chapter: a sum, a fraction and a root.
const formula = `<math display="block"><mrow><munderover><mo>&#8721;</mo><mrow><mi>k</mi><mo>=</mo><mn>1</mn></mrow><mi>n</mi></munderover>
<mfrac><mn>1</mn><msup><mi>k</mi><mn>2</mn></msup></mfrac><mo>&#8804;</mo><msqrt><mfrac><msup><mi>&#960;</mi><mn>4</mn></msup><mn>36</mn></mfrac></msqrt></mrow></math>`

// packageDocument returns the OPF: metadata, the manifest of every file and the spine of the
// chapters.
func packageDocument(options Options) string {
	var manifest, spine strings.Builder
	for index := range options.Chapters {
		number := index + 1
		fmt.Fprintf(&manifest, "    <item id=\"chapter%03d\" href=\"text/chapter%03d.xhtml\" media-type=\"application/xhtml+xml\"/>\n", number, number)
		fmt.Fprintf(&spine, "    <itemref idref=\"chapter%03d\"/>\n", number)
	}
	layout := ""
	if options.FixedLayout {
		layout = `<meta property="rendition:layout">pre-paginated</meta>`
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:uuid:epubtest-%d</dc:identifier>
    <dc:title>%s</dc:title>
    <dc:creator>Test Author</dc:creator>
    <dc:language>en</dc:language>
    %s
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="css" href="style.css" media-type="text/css"/>
    <item id="cover" href="images/cover.png" media-type="image/png" properties="cover-image"/>
    <item id="illustration" href="images/illustration.png" media-type="image/png"/>
    <item id="figure" href="images/figure.svg" media-type="image/svg+xml"/>
%s  </manifest>
  <spine toc="ncx">
%s  </spine>
</package>`, options.Chapters, options.Title, layout, manifest.String(), spine.String())
}

// chapterTitle returns the title of chapter number.
func chapterTitle(number int) string { return fmt.Sprintf("Chapter %d", number) }

// navigationDocument returns the EPUB 3 navigation: an entry for each chapter, the first with
// one for its footnote, and landmarks.
func navigationDocument(options Options) string {
	var entries strings.Builder
	for index := range options.Chapters {
		number := index + 1
		fmt.Fprintf(&entries, `<li><a href="text/chapter%03d.xhtml">%s</a>`, number, chapterTitle(number))
		if number == 1 && !options.FixedLayout {
			entries.WriteString(`<ol><li><a href="text/chapter001.xhtml#note1">A note</a></li></ol>`)
		}
		entries.WriteString("</li>\n")
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body>
<nav epub:type="toc"><h1>Contents</h1><ol>
` + entries.String() + `</ol></nav>
<nav epub:type="landmarks"><ol><li><a epub:type="bodymatter" href="text/chapter001.xhtml">Start</a></li></ol></nav>
</body></html>`
}

// ncx returns the EPUB 2 table of contents, which an EPUB 3 book keeps for older readers.
func ncx(options Options) string {
	var points strings.Builder
	for index := range options.Chapters {
		number := index + 1
		fmt.Fprintf(&points, `<navPoint id="point%d" playOrder="%d"><navLabel><text>%s</text></navLabel><content src="text/chapter%03d.xhtml"/></navPoint>`+"\n",
			number, number, chapterTitle(number), number)
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><navMap>
` + points.String() + `</navMap></ncx>`
}

// chapter returns the XHTML of reflowable chapter number: a heading, paragraphs with italics
// and bold, a quotation, a link to the next chapter and a footnote; and, in an illustrated
// chapter, the illustration, the SVG figure and the formula.
func chapter(options Options, number int, words *wordSource) string {
	var body strings.Builder
	fmt.Fprintf(&body, "<section epub:type=\"chapter\" id=\"chapter\">\n<h1 id=\"title\">%s</h1>\n", chapterTitle(number))
	illustrated := (number-1)%IllustrationEvery == 0
	for paragraph := range options.Paragraphs {
		quoted := paragraph > 0 && paragraph == options.Paragraphs/2
		switch {
		case paragraph == 0:
			body.WriteString(`<p class="first">`)
		case quoted:
			body.WriteString("<blockquote><p>")
		default:
			body.WriteString("<p>")
		}
		words.paragraph(&body, options.Words)
		if paragraph == 1 {
			body.WriteString(`<a epub:type="noteref" href="#note1">1</a>`)
		}
		if paragraph == options.Paragraphs-1 && number < options.Chapters {
			fmt.Fprintf(&body, ` <a href="chapter%03d.xhtml">Read on.</a>`, number+1)
		}
		if quoted {
			body.WriteString("</p></blockquote>\n")
		} else {
			body.WriteString("</p>\n")
		}
		if illustrated && paragraph == min(2, options.Paragraphs-1) {
			body.WriteString("<figure><img src=\"../images/illustration.png\" alt=\"An illustration\"/></figure>\n")
			body.WriteString("<p class=\"centre\"><img src=\"../images/figure.svg\" alt=\"A figure\"/></p>\n")
			body.WriteString(`<p class="centre">` + formula + "</p>\n")
		}
	}
	body.WriteString(`<aside epub:type="footnote" id="note1"><p>`)
	words.paragraph(&body, 20)
	body.WriteString("</p></aside>\n</section>")
	return xhtmlDocument(chapterTitle(number), "", body.String())
}

// fixedPage returns the XHTML of page number of a fixed-layout book: the illustration as the
// page, the SVG figure in a corner and a caption at the bottom.
func fixedPage(number int, words *wordSource) string {
	var body strings.Builder
	body.WriteString(`<img src="../images/illustration.png" alt=""/>` + "\n")
	body.WriteString(`<img class="figure" src="../images/figure.svg" alt=""/>` + "\n")
	fmt.Fprintf(&body, `<div class="caption"><p class="first"><b>Page %d.</b> `, number)
	words.paragraph(&body, 40)
	body.WriteString("</p></div>")
	viewport := fmt.Sprintf(`<meta name="viewport" content="width=%d, height=%d"/>`, FixedPageSize.X, FixedPageSize.Y)
	return xhtmlDocument(fmt.Sprintf("Page %d", number), viewport, body.String())
}

// xhtmlDocument returns an XHTML document titled title, linking the stylesheet, with head
// and body.
func xhtmlDocument(title, head, body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="en"><head>
<title>` + title + `</title>` + head + `
<link rel="stylesheet" type="text/css" href="../style.css"/></head>
<body>
` + body + `
</body></html>`
}

// pictureOf returns a PNG of size, gray shades in bands and a grid, the kind of picture that a
// scanned plate is: neither flat nor noise, so that it compresses and decodes as one would.
func pictureOf(tb testing.TB, size image.Point) string {
	tb.Helper()
	picture := image.NewGray(image.Rectangle{Max: size})
	for row := range size.Y {
		for column := range size.X {
			shade := uint8(255 - (row*160/size.Y + column*60/size.X))
			if row%50 < 3 || column%50 < 3 {
				shade /= 3
			}
			picture.Pix[row*picture.Stride+column] = shade
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		tb.Fatal(err)
	}
	return encoded.String()
}

// vocabulary is what the prose is made of: short and long words, French ones with accents and
// a few that hyphenation splits, as a novel has.
var vocabulary = strings.Fields(`the a of and to in was he she it that his her with for as
had on at by not but from they which you one all were we when there been their an said
would what so no if out about who into time could than other more these some two only
little very after over before through between under again never always nothing something
house window garden evening morning river mountain street letter silence question answer
remembered understood considered extraordinary unfortunately nevertheless incomprehensible
déjà café naïve élève château fenêtre thé être même où`)

// wordSource writes deterministic prose: a linear congruential generator picks the words, so
// that the text varies like prose without depending on a random seed.
type wordSource struct{ state uint64 }

// newWordSource returns a wordSource that starts from seed.
func newWordSource(seed uint64) *wordSource { return &wordSource{state: seed*2654435761 + 1} }

// next returns a number below limit.
func (words *wordSource) next(limit int) int {
	// Knuth's MMIX multiplier and increment; the high bits are the well-mixed ones.
	words.state = words.state*6364136223846793005 + 1442695040888963407
	return int((words.state >> 33) % uint64(limit))
}

// paragraph writes count words into body as sentences: capitalized, some words in italics or
// bold, commas, and a full stop every 8 to 20 words.
func (words *wordSource) paragraph(body *strings.Builder, count int) {
	sentenceLeft := 0
	for index := range count {
		word := vocabulary[words.next(len(vocabulary))]
		if sentenceLeft == 0 {
			sentenceLeft = 8 + words.next(13)
			first, size := utf8.DecodeRuneInString(word)
			word = string(unicode.ToUpper(first)) + word[size:]
		}
		if index > 0 {
			body.WriteByte(' ')
		}
		switch words.next(40) {
		case 0:
			body.WriteString("<em>" + word + "</em>")
		case 1:
			body.WriteString("<b>" + word + "</b>")
		default:
			body.WriteString(word)
		}
		sentenceLeft--
		switch {
		case sentenceLeft == 0 || index == count-1:
			body.WriteByte('.')
			sentenceLeft = 0
		case words.next(10) == 0:
			body.WriteByte(',')
		}
	}
}
