package epub

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
	"github.com/CedricHerzog/inkshelf/internal/web/webimage"
)

// hostileLimit is how long a hostile book may take to open, lay out and draw: the page budget,
// which the parsers' caps must keep it within with no worker to give up on it (SC-006).
const hostileLimit = layout.PageBudget

// pngHeader returns a PNG of 1×1 pixel whose header says it is width×height: a crafted image
// that would take gigabytes decoded.
func pngHeader(t *testing.T, width, height uint32) string {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewGray(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	data := encoded.Bytes()
	// The IHDR chunk follows the 8-byte signature: length, type, then width and height.
	const ihdr = 8 + 4
	binary.BigEndian.PutUint32(data[ihdr+4:], width)
	binary.BigEndian.PutUint32(data[ihdr+8:], height)
	binary.BigEndian.PutUint32(data[ihdr+4+13:], crc32.ChecksumIEEE(data[ihdr:ihdr+4+13]))
	return string(data)
}

// woffBomb returns a WOFF font of one table whose header announces 4 GB once decompressed.
func woffBomb() string {
	const headerSize, entrySize = 44, 20
	data := make([]byte, headerSize+entrySize+8)
	copy(data, "wOFF\x00\x01\x00\x00")
	binary.BigEndian.PutUint32(data[8:], uint32(len(data)))
	binary.BigEndian.PutUint16(data[12:], 1)
	entry := data[headerSize:]
	copy(entry, "glyf")
	binary.BigEndian.PutUint32(entry[4:], headerSize+entrySize) // offset
	binary.BigEndian.PutUint32(entry[8:], 8)                    // stored length
	binary.BigEndian.PutUint32(entry[12:], 0xffffffff)          // announced length
	return string(data)
}

// Books crafted to take the reader's time or memory open, lay out and draw within the page
// budget, without a panic: the caps of each parser hold.
func TestHostileBooks(t *testing.T) {
	var hugeSVG strings.Builder
	hugeSVG.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">`)
	for index := range 150_000 {
		hugeSVG.WriteString(`<rect x="` + string(rune('0'+index%10)) + `" width="1" height="1"/>`)
	}
	hugeSVG.WriteString(`</svg>`)
	deepMath := `<math>` + strings.Repeat("<mrow>", 20_000) + "<mi>x</mi>" + strings.Repeat("</mrow>", 20_000) + `</math>`
	nestedMedia := strings.Repeat("@media screen { ", 10_000) + "p { color: red }" + strings.Repeat(" }", 10_000)
	nestedNot := "p" + strings.Repeat(":not(", 10_000) + "a" + strings.Repeat(")", 10_000) + " { color: red }"
	hugeStylesheet := strings.Repeat("p.c { margin: 1em } ", 400_000) // 8 MB

	for _, test := range []struct {
		name, css, body string
		files           map[string]string
	}{
		{name: "huge SVG", body: `<p><img src="huge.svg" alt="Huge"/></p>`, files: map[string]string{"OEBPS/huge.svg": hugeSVG.String()}},
		{name: "huge inline SVG", body: hugeSVG.String()},
		{name: "60 Mpx PNG", body: `<p><img src="huge.png" alt="Huge"/></p>`, files: map[string]string{"OEBPS/huge.png": pngHeader(t, 7746, 7746)}},
		{name: "deep MathML", body: `<p>So ` + deepMath + `.</p>`},
		{name: "nested CSS", css: nestedMedia + nestedNot, body: `<p>Text</p>`},
		{name: "huge stylesheet", css: hugeStylesheet, body: `<p class="c">Text</p>`},
		{name: "WOFF bomb", css: `@font-face { font-family: Bomb; src: url(bomb.woff) } p { font-family: Bomb }`, body: `<p>Text</p>`,
			files: map[string]string{"OEBPS/bomb.woff": woffBomb()}},
	} {
		t.Run(test.name, func(t *testing.T) {
			start := time.Now()
			book := openStyled(t, test.css, test.body, test.files)
			doc, err := book.Chapter(0)
			if err != nil {
				t.Fatal(err)
			}
			faces := layout.NewFaces(fonts.Serif)
			defer faces.Close()
			settings := layout.Settings{Width: 600, Height: 800, Margins: image.Rect(30, 30, 30, 30), FontSize: 24, LineSpacing: 130}
			pages, err := layout.Paginate(doc, settings, faces, book.ImageSize)
			if err != nil {
				t.Fatal(err)
			}
			images := webimage.NewLoader(book.Resource, faces.SVGText())
			for _, page := range pages {
				canvas := image.NewGray(image.Rect(0, 0, settings.Width, settings.Height))
				if err := page.Draw(canvas, faces, images, layout.Elements{}); err != nil {
					t.Fatal(err)
				}
			}
			if took := time.Since(start); took > hostileLimit {
				t.Errorf("took %v, over the page budget", took)
			}
		})
	}
}
