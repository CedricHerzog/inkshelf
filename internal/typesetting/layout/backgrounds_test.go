package layout

import (
	"errors"
	"image"
	"image/draw"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// layersOf returns the layers of a style's background, its lengths taken as box units.
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

func whiteArea(size image.Point) *image.Gray {
	canvas := image.NewGray(image.Rectangle{Max: size})
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	return canvas
}

func noTiles(string, image.Point) (Tile, error) { return Tile{}, errors.New("no image") }

func TestPaintBackgroundGrid(t *testing.T) {
	// The W3C ICB tests' grid: two gradients, each a 1-pixel line, tiled every 50 pixels.
	layers := layersOf(t, "background-image: linear-gradient(currentColor 1px, transparent 1px), "+
		"linear-gradient(to right, currentColor 1px, transparent 1px); background-size: 50px 50px")
	canvas := whiteArea(image.Pt(200, 200))
	PaintBackground(canvas, canvas.Bounds(), layers, 1, noTiles)
	for _, test := range []struct {
		x, y int
		dark bool
	}{
		{0, 25, true}, {50, 25, true}, {150, 199, true}, {25, 0, true}, {25, 100, true},
		{25, 25, false}, {75, 125, false}, {199, 199, false}, {49, 49, false},
	} {
		if dark := canvas.GrayAt(test.x, test.y).Y < 128; dark != test.dark {
			t.Errorf("pixel (%d,%d) = %d, want dark %v", test.x, test.y, canvas.GrayAt(test.x, test.y).Y, test.dark)
		}
	}
}

func TestPaintBackgroundRepeatingAndRadial(t *testing.T) {
	canvas := whiteArea(image.Pt(100, 100))
	stripes := layersOf(t, "background-image: repeating-linear-gradient(90deg, black, black 10px, white 10px, white 20px)")
	PaintBackground(canvas, canvas.Bounds(), stripes, 1, noTiles)
	for _, x := range []int{5, 25, 85} {
		if gray := canvas.GrayAt(x, 50).Y; gray > 10 {
			t.Errorf("stripe at x=%d is %d, want black", x, gray)
		}
	}
	for _, x := range []int{15, 35, 95} {
		if gray := canvas.GrayAt(x, 50).Y; gray < 245 {
			t.Errorf("gap at x=%d is %d, want white", x, gray)
		}
	}

	canvas = whiteArea(image.Pt(100, 100))
	PaintBackground(canvas, canvas.Bounds(), layersOf(t, "background-image: radial-gradient(circle closest-side, black, white)"), 1, noTiles)
	centre, edge, corner := canvas.GrayAt(50, 50).Y, canvas.GrayAt(50, 1).Y, canvas.GrayAt(1, 1).Y
	if !(centre < 10 && edge > 240 && corner > 250) {
		t.Errorf("radial: centre %d, edge %d, corner %d; want black to white at the closest side", centre, edge, corner)
	}
	// A unit scales lengths: half the radius at a unit of 2 is the same circle.
	canvas = whiteArea(image.Pt(100, 100))
	PaintBackground(canvas, canvas.Bounds(), layersOf(t, "background-image: radial-gradient(25px at 25px 25px, black, white)"), 2, noTiles)
	if centre, edge := canvas.GrayAt(50, 50).Y, canvas.GrayAt(50, 1).Y; centre > 10 || edge < 240 {
		t.Errorf("scaled radial: centre %d, edge %d", centre, edge)
	}
}

// A gradient to a corner and at an angle: CSS's gradient line, as long as the box's corners
// are apart along it.
func TestGradientDirections(t *testing.T) {
	for _, test := range []struct {
		value       string
		dark, light image.Point
	}{
		{"linear-gradient(90deg, white, black)", image.Pt(99, 50), image.Pt(0, 50)},
		{"linear-gradient(to top left, white, black)", image.Pt(0, 0), image.Pt(99, 99)},
		{"linear-gradient(0.5turn, black, white)", image.Pt(50, 0), image.Pt(50, 99)},
	} {
		canvas := whiteArea(image.Pt(100, 100))
		PaintBackground(canvas, canvas.Bounds(), layersOf(t, "background-image: "+test.value), 1, noTiles)
		if dark, light := canvas.GrayAt(test.dark.X, test.dark.Y).Y, canvas.GrayAt(test.light.X, test.light.Y).Y; dark > 10 || light < 245 {
			t.Errorf("%s: %d at %v, %d at %v", test.value, dark, test.dark, light, test.light)
		}
	}
}

func TestPaintBackgroundImageTiles(t *testing.T) {
	var asked []image.Point
	black := func(_ string, size image.Point) (Tile, error) {
		asked = append(asked, size)
		return Tile{Gray: image.NewGray(image.Rectangle{Max: size})}, nil
	}
	image10 := func(style string) []document.BackgroundLayer {
		layers := layersOf(t, style)
		layers[0].ImageSize = image.Pt(10, 20)
		return layers
	}
	for _, test := range []struct {
		style      string
		size       image.Point
		darkPixels int
	}{
		{"background: url(a.png) no-repeat", image.Pt(10, 20), 200},
		{"background: url(a.png) no-repeat right bottom", image.Pt(10, 20), 200},
		{"background: url(a.png) repeat-x", image.Pt(10, 20), 100 * 20},
		{"background: url(a.png) 0 0 / 50% auto no-repeat", image.Pt(50, 100), 50 * 100},
		{"background: url(a.png) 0 0 / cover no-repeat", image.Pt(100, 200), 100 * 100},
		{"background: url(a.png) center / contain no-repeat", image.Pt(50, 100), 50 * 100},
		{"background: url(a.png) space", image.Pt(10, 20), 100 * 100}, // 10 by 5 whole tiles fill it
		{"background: url(a.png) 0 0 / 30px 30px round", image.Pt(33, 33), 100 * 100},
	} {
		asked = nil
		canvas := whiteArea(image.Pt(100, 100))
		PaintBackground(canvas, canvas.Bounds(), image10(test.style), 1, black)
		dark := 0
		for _, pixel := range canvas.Pix {
			if pixel == 0 {
				dark++
			}
		}
		if len(asked) != 1 || asked[0] != test.size || dark != test.darkPixels {
			t.Errorf("%s: tiles asked %v, %d dark pixels; want one of %v, %d", test.style, asked, dark, test.size, test.darkPixels)
		}
	}
	// Right bottom puts the tile in the corner.
	canvas := whiteArea(image.Pt(100, 100))
	PaintBackground(canvas, canvas.Bounds(), image10("background: url(a.png) no-repeat right bottom"), 1, black)
	if canvas.GrayAt(95, 90).Y != 0 || canvas.GrayAt(5, 5).Y != 255 {
		t.Error("right bottom: the tile is not in the bottom right corner")
	}
}

func TestPaintBackgroundTooManyTiles(t *testing.T) {
	canvas := whiteArea(image.Pt(1000, 1000))
	layers := layersOf(t, "background-image: linear-gradient(black, black); background-size: 2px 2px")
	PaintBackground(canvas, canvas.Bounds(), layers, 1, noTiles)
	if canvas.GrayAt(500, 500).Y != 255 {
		t.Errorf("a layer of %d tiles was drawn, past the cap of %d", 500*500, maxBackgroundTiles)
	}
}

func BenchmarkPaintBackground(b *testing.B) {
	layers := layersOf(b, "background-image: repeating-linear-gradient(45deg, black, black 4px, transparent 4px, transparent 12px), "+
		"radial-gradient(circle, white, gray); background-size: 200px 200px, auto")
	canvas := whiteArea(image.Pt(1404, 1872))
	b.ReportAllocs()
	for b.Loop() {
		PaintBackground(canvas, canvas.Bounds(), layers, 1, noTiles)
	}
}

func FuzzPaintBackground(f *testing.F) {
	for _, seed := range []string{
		"background: repeating-radial-gradient(circle 3px at 1px 2px, red, blue 2px) 0 0 / 7px 9px round space",
		"background: linear-gradient(red 1px, transparent 1px) right 10% / 1px auto, url(a.png) center / contain, gray",
		"background: radial-gradient(ellipse 0 10px, red, blue)", "background: url(a) 0 0 / 1e9px 1e9px",
	} {
		f.Add(seed, uint16(100), uint16(80), uint8(3))
	}
	tiles := func(_ string, size image.Point) (Tile, error) {
		return Tile{Gray: image.NewGray(image.Rectangle{Max: size}), Alpha: image.NewGray(image.Rectangle{Max: size})}, nil
	}
	f.Fuzz(func(t *testing.T, style string, width, height uint16, unit uint8) {
		var layers []document.BackgroundLayer
		for _, layer := range css.Backgrounds(css.ParseDeclarations(style), css.Black) {
			layers = append(layers, document.BackgroundLayer{BackgroundLayer: layer, ImageSize: image.Pt(int(width%64), int(height%64))})
		}
		canvas := whiteArea(image.Pt(200, 200))
		box := image.Rect(-20, 10, int(width%400)-20, int(height%400)+10)
		start := time.Now()
		PaintBackground(canvas, box, layers, float64(unit)/4, tiles)
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("%q took %v", style, elapsed)
		}
	})
}
