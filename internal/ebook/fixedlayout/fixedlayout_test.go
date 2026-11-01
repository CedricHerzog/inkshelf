package fixedlayout

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
	"github.com/CedricHerzog/inkshelf/internal/web/webimage"
)

func TestParseViewport(t *testing.T) {
	tests := []struct {
		content string
		size    image.Point
		ok      bool
	}{
		{"width=1200, height=1800", image.Pt(1200, 1800), true},
		{"width=1200,height=1800", image.Pt(1200, 1800), true},
		{" width = 1200px ; height = 1800.4px ", image.Pt(1200, 1800), true},
		{"height=900, width=600, width=300", image.Pt(600, 900), true}, // the first width wins
		{"width=device-width, initial-scale=1", image.Pt(0, 0), false},
		{"width=1200", image.Pt(1200, 0), false},
		{"junk", image.Point{}, false},
		{"", image.Point{}, false},
	}
	for _, test := range tests {
		size, ok := ParseViewport(test.content)
		if size != test.size || ok != test.ok {
			t.Errorf("ParseViewport(%q) = %v, %v; want %v, %v", test.content, size, ok, test.size, test.ok)
		}
	}
}

// files are the files of a test book, by path.
type files map[string][]byte

func (book files) Resource(path string) ([]byte, error) {
	data, ok := book[path]
	if !ok {
		return nil, errors.New(path + ": not in the book")
	}
	return data, nil
}

func (book files) ImageSize(path string) (image.Point, error) {
	data, err := book.Resource(path)
	if err != nil {
		return image.Point{}, err
	}
	return webimage.Size(data)
}

// blackPNG returns a black PNG of size.
func blackPNG(t *testing.T, size image.Point) []byte {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewGray(image.Rectangle{Max: size})); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

// imageDocument returns a document of one image block.
func imageDocument(imagePath string) *document.Document {
	doc := &document.Document{}
	doc.Add(document.Block{Kind: document.Image, Image: imagePath})
	return doc
}

// darkIn reports whether page has a dark pixel in rect.
func darkIn(page *image.Gray, rect image.Rectangle) bool {
	rect = rect.Intersect(page.Bounds())
	for row := rect.Min.Y; row < rect.Max.Y; row++ {
		for column := rect.Min.X; column < rect.Max.X; column++ {
			if page.GrayAt(column, row).Y < 128 {
				return true
			}
		}
	}
	return false
}

// A page that is one image: the page, 600x800, fits a 300x300 screen as 225x300, and the
// image, 400x400 in it, is fitted and centred: 225x225 from y 37.
func TestRenderImagePage(t *testing.T) {
	book := files{"page.png": blackPNG(t, image.Pt(400, 400))}
	page := &Page{Size: image.Pt(600, 800), EmPixels: 16, Regions: []*Region{{Rect: image.Rect(0, 0, 600, 800), Content: imageDocument("page.png")}}}
	drawn, err := page.Render(image.Pt(300, 300), book)
	if err != nil {
		t.Fatal(err)
	}
	if size := drawn.Bounds().Size(); size != image.Pt(225, 300) {
		t.Fatalf("page %v, want 225x300", size)
	}
	if !darkIn(drawn, image.Rect(112, 150, 113, 151)) || darkIn(drawn, image.Rect(0, 0, 225, 36)) || darkIn(drawn, image.Rect(0, 264, 225, 300)) {
		t.Error("the image is not fitted and centred in the page")
	}
}

// The text of a positioned box is laid out in it, at the page's scale, and nothing of it is
// drawn outside; its background image, sized 100% 100%, is stretched over it.
func TestRenderPositionedText(t *testing.T) {
	text := &document.Document{}
	text.Add(document.Block{Runs: []document.Run{{Text: "Il était une fois une très longue phrase qui ne tient pas sur une ligne."}}})
	box := image.Rect(100, 200, 500, 400)
	page := &Page{Size: image.Pt(600, 800), EmPixels: 16, Regions: []*Region{
		{Rect: image.Rect(0, 0, 600, 800)},
		{Rect: box, Content: text},
		{Rect: image.Rect(0, 700, 600, 800), Layers: layersOf(t, "background: url(dark.png) 0 0 / 100% 100% no-repeat")},
	}}
	drawn, err := page.Render(image.Pt(1200, 1600), files{"dark.png": blackPNG(t, image.Pt(10, 10))})
	if err != nil {
		t.Fatal(err)
	}
	scaledBox := image.Rect(200, 400, 1000, 800)
	if !darkIn(drawn, scaledBox) {
		t.Error("no text in the box")
	}
	outside := []image.Rectangle{image.Rect(0, 0, 1200, 400), image.Rect(0, 400, 200, 1400), image.Rect(1000, 400, 1200, 1400), image.Rect(0, 800, 1200, 1400)}
	for _, area := range outside {
		if darkIn(drawn, area) {
			t.Errorf("text drawn in %v, outside the box", area)
		}
	}
	if !darkIn(drawn, image.Rect(0, 1400, 1, 1401)) || !darkIn(drawn, image.Rect(1199, 1599, 1200, 1600)) {
		t.Error("the background is not stretched over its box")
	}
}

// An SVG page is drawn at its size: its 100x200 viewBox fills a 50x100 page.
func TestRenderSVGPage(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="200" viewBox="0 0 100 200"><rect x="0" y="100" width="100" height="100" fill="black"/></svg>`)
	page := &Page{Size: image.Pt(100, 200), EmPixels: 16, Regions: []*Region{{Rect: image.Rect(0, 0, 100, 200), Content: imageDocument("page.svg")}}}
	drawn, err := page.Render(image.Pt(100, 100), files{"page.svg": svg})
	if err != nil {
		t.Fatal(err)
	}
	if size := drawn.Bounds().Size(); size != image.Pt(50, 100) {
		t.Fatalf("page %v, want 50x100", size)
	}
	if darkIn(drawn, image.Rect(0, 0, 50, 48)) || !darkIn(drawn, image.Rect(25, 75, 26, 76)) {
		t.Error("the SVG's lower half is not black and its upper half white")
	}
}

func FuzzRenderBackground(f *testing.F) {
	for _, seed := range []string{"background: linear-gradient(to top left, red, blue 10% 20%, #0008)",
		"background: url(half.png) 3px 4px / 7px auto space, repeating-radial-gradient(circle, black, white 2px)",
		"background: linear-gradient(-45deg, black -50%, white 150%) 0 0 / 1px 1px"} {
		f.Add(seed, 7, 3)
	}
	files := files{"half.png": transparentPNG(f, image.Pt(4, 4))}
	f.Fuzz(func(t *testing.T, style string, width, height int) {
		var layers []document.BackgroundLayer
		for _, layer := range css.Backgrounds(css.ParseDeclarations(style), css.Black) {
			layers = append(layers, document.BackgroundLayer{BackgroundLayer: layer, ImageSize: image.Pt(4, 4)})
		}
		page := &Page{Size: image.Pt(64, 64), EmPixels: 16, Regions: []*Region{{Rect: image.Rect(-2, -2, width%64, height%64), Layers: layers}}}
		if _, err := page.Render(image.Pt(32, 32), files); err != nil {
			t.Fatal(err)
		}
	})
}

// layersOf returns the background layers of a style, in CSS pixels.
func layersOf(t testing.TB, style string) []document.BackgroundLayer {
	t.Helper()
	var layers []document.BackgroundLayer
	for _, layer := range css.Backgrounds(css.ParseDeclarations(style), css.Black) {
		layers = append(layers, document.BackgroundLayer{BackgroundLayer: layer})
	}
	if len(layers) == 0 {
		t.Fatalf("no layer in %q", style)
	}
	return layers
}

// transparentPNG returns a PNG of size whose left half is black and right half transparent.
func transparentPNG(t testing.TB, size image.Point) []byte {
	t.Helper()
	picture := image.NewNRGBA(image.Rectangle{Max: size})
	for row := range size.Y {
		for column := range size.X / 2 {
			picture.Set(column, row, color.NRGBA{A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

// A box's color, then its layers from the bottom one up: the gradient from white to black, top
// to bottom, over a half-transparent gray; the transparent half of the image shows them.
func TestRenderBackgrounds(t *testing.T) {
	page := &Page{Size: image.Pt(100, 100), EmPixels: 16, Regions: []*Region{
		{Rect: image.Rect(0, 0, 100, 50), Color: css.Color{Gray: 0, Alpha: 128}},
		{Rect: image.Rect(0, 50, 100, 100), Layers: layersOf(t, "background: url(half.png) 0 0 / 100% 100%, linear-gradient(to bottom, white, black)")},
	}}
	drawn, err := page.Render(image.Pt(100, 100), files{"half.png": transparentPNG(t, image.Pt(20, 20))})
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		point     image.Point
		low, high uint8
	}{
		{image.Pt(50, 25), 125, 129}, // the color over white
		{image.Pt(20, 75), 0, 10},    // the image's black half
		{image.Pt(80, 51), 240, 255}, // the gradient, white at its top
		{image.Pt(80, 75), 120, 136},
		{image.Pt(80, 98), 0, 15},
	} {
		if gray := drawn.GrayAt(check.point.X, check.point.Y).Y; gray < check.low || gray > check.high {
			t.Errorf("gray at %v = %d, want %d to %d", check.point, gray, check.low, check.high)
		}
	}
}
