package epub

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// openFixedLayout opens a pre-paginated book whose chapters are the XHTML pages, by path in
// the zip (sorted), with files.
func openFixedLayout(t *testing.T, files map[string]string, pages ...string) *Book {
	t.Helper()
	var manifest, spine strings.Builder
	for index, page := range pages {
		id := fmt.Sprintf("page%d", index)
		mediaType := "application/xhtml+xml"
		if strings.HasSuffix(page, ".svg") {
			mediaType = "image/svg+xml"
		}
		manifest.WriteString(`<item id="` + id + `" href="` + strings.TrimPrefix(page, "OEBPS/") + `" media-type="` + mediaType + `"/>`)
		spine.WriteString(`<itemref idref="` + id + `"/>`)
	}
	return openPackage(t, "", `<dc:title>Album</dc:title><meta property="rendition:layout">pre-paginated</meta>`,
		manifest.String()+`<item id="bg" href="img/bg.png" media-type="image/png"/>`, ">"+spine.String(), files)
}

// fixedXHTML returns a page of a fixed-layout book: its head, then body.
func fixedXHTML(head, body string) string {
	return `<html xmlns="http://www.w3.org/1999/xhtml"><head>` + head + `</head><body>` + body + `</body></html>`
}

// A page's size is its viewport's; its positioned elements are boxes placed in it, their
// lengths in pixels, percentages or em, their text laid out in them at its own size; the
// body's background, named relative to its stylesheet, covers the page.
func TestFixedPage(t *testing.T) {
	files := map[string]string{
		"OEBPS/img/bg.png": pngOf(t, image.Pt(40, 60)),
		"OEBPS/css/page.css": `body { background: url("../img/bg.png") no-repeat }
.caption { position: absolute; left: 10%; top: 100px; width: 300px; height: 2em; font-size: 64px }
.corner { position: absolute; right: 20px; bottom: 30px }`,
		"OEBPS/text/page.xhtml": fixedXHTML(`<meta name="viewport" content="width=800, height=1200"/><meta name="viewport" content="width=1, height=1"/>
<link rel="stylesheet" href="../css/page.css"/>`, `<p>In the flow</p><div class="caption"><p>Caption</p></div><img class="corner" src="../img/bg.png"/><p>Flow again</p>`),
	}
	book := openFixedLayout(t, files, "OEBPS/text/page.xhtml")
	if !book.FixedLayout() {
		t.Fatal("the book is not fixed-layout")
	}
	page, err := book.fixedPage(book.ChapterItem(0))
	if err != nil {
		t.Fatal(err)
	}
	if page.Size != image.Pt(800, 1200) {
		t.Errorf("page size %v, want the first viewport's", page.Size)
	}
	if len(page.Regions) != 4 {
		t.Fatalf("%d regions, want the flow, the body's background, the caption and the corner image", len(page.Regions))
	}
	flow, background, caption, corner := page.Regions[0], page.Regions[1], page.Regions[2], page.Regions[3]
	if got := texts(flow.Content); strings.Join(got, "|") != "In the flow|Flow again" || flow.Rect != image.Rect(0, 0, 800, 1200) {
		t.Errorf("flow %v in %v", got, flow.Rect)
	}
	if background.Background != "OEBPS/img/bg.png" || background.Rect != image.Rect(0, 0, 800, 1200) || background.Content != nil {
		t.Errorf("background %+v", background)
	}
	if caption.Rect != image.Rect(80, 100, 380, 228) || strings.Join(texts(caption.Content), "|") != "Caption" {
		t.Errorf("caption %v in %v", texts(caption.Content), caption.Rect)
	}
	if scale := caption.Content.Blocks[0].FontScale; scale != 400 {
		t.Errorf("caption at %d%%, want 64px: 400%%", scale)
	}
	if blocks := corner.Content.Blocks; corner.Rect != image.Rect(740, 1110, 780, 1170) || len(blocks) != 1 || blocks[0].Kind != document.Image {
		t.Errorf("corner image %+v in %v", blocks, corner.Rect)
	}
}

// Background colors and gradients make boxes: the body's covers the page, a positioned
// element's is its box; an element in the flow, whose box is not known, keeps none.
func TestFixedPageColors(t *testing.T) {
	files := map[string]string{
		"OEBPS/page.xhtml": fixedXHTML(`<meta name="viewport" content="width=600, height=800"/><style>
body { background: #808080 }
.band { position: absolute; top: 0; height: 100px; width: 600px; background-image: linear-gradient(to right, white, black) }
.dim { position: absolute; top: 200px; width: 10px; height: 10px; color: black; background-color: currentColor }
p { background-color: red }
</style>`, `<div class="band"></div><div class="dim"></div><p>Text</p>`),
	}
	book := openFixedLayout(t, files, "OEBPS/page.xhtml")
	page, err := book.fixedPage(book.ChapterItem(0))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Regions) != 4 {
		t.Fatalf("%d regions, want the flow, the body, the band and the dim box", len(page.Regions))
	}
	body, band, dim := page.Regions[1], page.Regions[2], page.Regions[3]
	if body.Rect != image.Rect(0, 0, 600, 800) || body.Color != (css.Color{Gray: 128, Alpha: 255}) || body.Gradient != nil {
		t.Errorf("body %+v", body)
	}
	if band.Rect != image.Rect(0, 0, 600, 100) || band.Gradient == nil || band.Gradient.Angle != 90 || band.Color.Alpha != 0 {
		t.Errorf("band %+v", band)
	}
	if dim.Color != css.Black {
		t.Errorf("dim box %+v, want black", dim)
	}
	drawn, err := book.RenderPage(0, image.Pt(600, 800))
	if err != nil {
		t.Fatal(err)
	}
	if left, right, middle := drawn.GrayAt(5, 50).Y, drawn.GrayAt(595, 50).Y, drawn.GrayAt(300, 500).Y; left < 245 || right > 10 || middle < 120 || middle > 136 {
		t.Errorf("grays %d, %d (the band's ends), %d (the body)", left, right, middle)
	}
}

// A grid of gradients tiled by background-size (W3C's lay-pp-xhtml-icb) keeps its color: one
// gradient stretched over the page would hide it.
func TestFixedPageTiledGradients(t *testing.T) {
	files := map[string]string{
		"OEBPS/page.xhtml": fixedXHTML(`<meta name="viewport" content="width=600, height=800"/><style>
html { background-color: white;
  background-image: linear-gradient(black 1px, transparent 1px), linear-gradient(to right, black 1px, transparent 1px);
  background-size: 50px 50px }
.stripes { position: absolute; top: 0; width: 100px; height: 100px; background: linear-gradient(white, black); background-size: 10px 10px }
</style>`, `<div class="stripes"></div>`),
	}
	book := openFixedLayout(t, files, "OEBPS/page.xhtml")
	page, err := book.fixedPage(book.ChapterItem(0))
	if err != nil {
		t.Fatal(err)
	}
	for _, box := range page.Regions {
		if box.Gradient != nil {
			t.Errorf("box %v has a gradient, want its color only", box.Rect)
		}
	}
	drawn, err := book.RenderPage(0, image.Pt(600, 800))
	if err != nil {
		t.Fatal(err)
	}
	if gray := drawn.GrayAt(300, 400).Y; gray < 245 {
		t.Errorf("the page is %d, want white", gray)
	}
}

// A page without a viewport is the size of its first image; an SVG page, of the SVG.
func TestFixedPageSize(t *testing.T) {
	files := map[string]string{
		"OEBPS/img/bg.png":  pngOf(t, image.Pt(300, 500)),
		"OEBPS/page.xhtml":  fixedXHTML("", `<div><img src="img/bg.png"/></div>`),
		"OEBPS/picture.svg": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><rect width="200" height="100"/></svg>`,
	}
	book := openFixedLayout(t, files, "OEBPS/page.xhtml", "OEBPS/picture.svg")
	for index, want := range []image.Point{image.Pt(300, 500), image.Pt(200, 100)} {
		page, err := book.fixedPage(book.ChapterItem(index))
		if err != nil {
			t.Fatal(err)
		}
		if page.Size != want {
			t.Errorf("page %d is %v, want %v", index, page.Size, want)
		}
	}
	drawn, err := book.RenderPage(1, image.Pt(100, 100))
	if err != nil {
		t.Fatal(err)
	}
	if size := drawn.Bounds().Size(); size != image.Pt(100, 50) {
		t.Errorf("SVG page drawn %v, want 100x50", size)
	}
}

// In a book that is not all pre-paginated, a pre-paginated chapter is one image: its page,
// drawn by Resource.
func TestMixedLayouts(t *testing.T) {
	files := map[string]string{
		"OEBPS/text.xhtml": xhtml("<p>Text</p>"),
		"OEBPS/page.xhtml": fixedXHTML(`<meta name="viewport" content="width=600, height=600"/>`, `<p>Page</p>`),
	}
	book := openPackage(t, "", `<dc:title>Mixed</dc:title>`,
		`<item id="text" href="text.xhtml" media-type="application/xhtml+xml"/><item id="page" href="page.xhtml" media-type="application/xhtml+xml"/>`,
		`><itemref idref="text"/><itemref idref="page" properties="rendition:layout-pre-paginated"/>`, files)
	if book.FixedLayout() {
		t.Error("a mixed book is fixed-layout")
	}
	doc, err := book.Chapter(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Blocks) != 1 || doc.Blocks[0].Kind != document.Image {
		t.Fatalf("the pre-paginated chapter is %+v, want one image", doc.Blocks)
	}
	imagePath := doc.Blocks[0].Image
	size, err := book.ImageSize(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	data, err := book.Resource(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	drawn, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if size != image.Pt(1404, 1404) || drawn.Bounds().Size() != size {
		t.Errorf("page image %v, drawn %v; want 1404x1404", size, drawn.Bounds().Size())
	}
}
