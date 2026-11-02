package reading

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/epubtest"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// spreadBook returns the files of a book of text, a picture across two pages (two chapters of
// one black image each, pinned left and right), then text.
func spreadBook(t *testing.T) map[string]string {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewGray(image.Rect(0, 0, 200, 300))); err != nil {
		t.Fatal(err)
	}
	page := func(body string) string {
		return `<html xmlns="http://www.w3.org/1999/xhtml"><body>` + body + `</body></html>`
	}
	return map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`,
		"OEBPS/content.opf": `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Spread</dc:title><dc:language>en</dc:language></metadata>
  <manifest><item id="before" href="before.xhtml" media-type="application/xhtml+xml"/>
    <item id="left" href="left.xhtml" media-type="application/xhtml+xml"/>
    <item id="right" href="right.xhtml" media-type="application/xhtml+xml"/>
    <item id="after" href="after.xhtml" media-type="application/xhtml+xml"/>
    <item id="black" href="black.png" media-type="image/png"/></manifest>
  <spine><itemref idref="before"/><itemref idref="left" properties="page-spread-left"/>
    <itemref idref="right" properties="page-spread-right"/><itemref idref="after"/></spine>
</package>`,
		"OEBPS/before.xhtml": page("<p>Before.</p>"),
		"OEBPS/left.xhtml":   page(`<div><img src="black.png" alt=""/></div>`),
		"OEBPS/right.xhtml":  page(`<div><img src="black.png" alt=""/></div>`),
		"OEBPS/after.xhtml":  page("<p>After.</p>"),
		"OEBPS/black.png":    encoded.String(),
	}
}

// The two chapters of an image spread read as one page, drawn in landscape: a turn forward
// goes past both, a turn back or a location in the second lands on the first.
func TestImageSpread(t *testing.T) {
	store := shelf.Store{Dir: t.TempDir()}
	book := shelf.Book{ID: "spread", Format: servers.Reflowable}
	data, err := os.ReadFile(epubtest.Write(t, spreadBook(t)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(strings.TrimSuffix(store.EPUBPath(book.ID), "book.epub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.EPUBPath(book.ID), data, 0o644); err != nil {
		t.Fatal(err)
	}
	text, err := Open(store, book, testSettings(24))
	if err != nil {
		t.Fatal(err)
	}
	defer text.Close()

	if moved, err := text.Turn(true); !moved || err != nil || !text.AtSpread() || text.At() != (PagePosition{Chapter: 1}) {
		t.Fatalf("turned to %v (%v, %v), want the spread at chapter 1", text.At(), moved, err)
	}
	drawn, err := text.Draw(text.At())
	if err != nil {
		t.Fatal(err)
	}
	settings := testSettings(24)
	if size := drawn.Bounds().Size(); size != image.Pt(settings.Height, settings.Width) {
		t.Errorf("the spread is %v, want the page turned to landscape", size)
	}
	// Each half is black against the gutter.
	if left, right := drawn.GrayAt(settings.Height/2-5, settings.Width/2).Y, drawn.GrayAt(settings.Height/2+5, settings.Width/2).Y; left > 64 || right > 64 {
		t.Errorf("by the gutter: %d and %d, want both images", left, right)
	}
	if moved, err := text.Turn(true); !moved || err != nil || text.At().Chapter != 3 || text.AtSpread() {
		t.Errorf("turned to %v, want chapter 3, past both of the spread's", text.At())
	}
	if moved, err := text.Turn(false); !moved || err != nil || text.At() != (PagePosition{Chapter: 1}) {
		t.Errorf("turned back to %v, want the spread", text.At())
	}
	if err := text.GoTo(shelf.Location{Chapter: 2}); err != nil || text.At() != (PagePosition{Chapter: 1}) {
		t.Errorf("a location in the spread's second chapter shows %v, want the spread", text.At())
	}
}
