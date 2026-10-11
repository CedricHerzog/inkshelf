package svgimage

import (
	"fmt"
	"image"
	"strings"
	"testing"
)

// patternSVG is a 20×20 SVG of content, a 20×20 image of it.
func patternSVG(t *testing.T, content string) *image.Gray {
	t.Helper()
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20">` + content + `</svg>`
	return rasterizeGray(t, svg, image.Pt(20, 20), Options{})
}

// expectGrays checks the gray of canvas at each point, within 2.
func expectGrays(t *testing.T, name string, canvas *image.Gray, want map[image.Point]uint8) {
	t.Helper()
	for point, gray := range want {
		if got := canvas.GrayAt(point.X, point.Y).Y; int(got)-int(gray) > 2 || int(gray)-int(got) > 2 {
			t.Errorf("%s: gray %d at %v, want %d", name, got, point, gray)
		}
	}
}

// A 4×4 tile, black on its left half and empty on its right, repeats over a rectangle: what
// the tile leaves empty shows the white under it.
func TestPatternTiled(t *testing.T) {
	canvas := patternSVG(t, `<defs><pattern id="stripes" width="4" height="4" patternUnits="userSpaceOnUse">`+
		`<rect width="2" height="4" fill="black"/></pattern></defs><rect width="20" height="20" fill="url(#stripes)"/>`)
	for x := range 20 {
		want := uint8(255)
		if x%4 < 2 {
			want = 0
		}
		expectGrays(t, "stripes", canvas, map[image.Point]uint8{{x, 3}: want, {x, 17}: want})
	}
}

// By default, a pattern's box is in fractions of the shape's, its tiles starting at the
// shape's corner; in user space, at the origin.
func TestPatternUnits(t *testing.T) {
	shape := `<rect x="5" y="0" width="20" height="20" fill="url(#squares)"/>`
	// A 10×10 tile, black on its first 5×5.
	boundingBox := patternSVG(t, `<pattern id="squares" width=".5" height="50%"><rect width="5" height="5"/></pattern>`+shape)
	expectGrays(t, "objectBoundingBox", boundingBox, map[image.Point]uint8{
		{7, 2}: 0, {12, 2}: 255, {17, 2}: 0, {7, 7}: 255, {7, 12}: 0, {2, 2}: 255,
	})
	userSpace := patternSVG(t, `<pattern id="squares" width="10" height="10" patternUnits="userSpaceOnUse"><rect width="5" height="5"/></pattern>`+shape)
	expectGrays(t, "userSpaceOnUse", userSpace, map[image.Point]uint8{
		{7, 2}: 255, {12, 2}: 0, {17, 2}: 255, {2, 2}: 255,
	})
	// The content in fractions of the shape's box, or through a viewBox.
	contentUnits := patternSVG(t, `<pattern id="squares" width=".5" height=".5" patternContentUnits="objectBoundingBox"><rect width=".25" height=".25"/></pattern>`+shape)
	expectGrays(t, "patternContentUnits", contentUnits, map[image.Point]uint8{{7, 2}: 0, {12, 2}: 255, {17, 2}: 0})
	viewBox := patternSVG(t, `<pattern id="squares" width=".5" height=".5" viewBox="0 0 2 2"><rect width="1" height="1"/></pattern>`+shape)
	expectGrays(t, "viewBox", viewBox, map[image.Point]uint8{{7, 2}: 0, {12, 2}: 255, {17, 2}: 0})
	// patternTransform moves the tiles.
	moved := patternSVG(t, `<pattern id="squares" width="10" height="10" patternUnits="userSpaceOnUse" patternTransform="translate(5)"><rect width="5" height="5"/></pattern>`+shape)
	expectGrays(t, "patternTransform", moved, map[image.Point]uint8{{7, 2}: 0, {12, 2}: 255, {17, 2}: 0})
}

// A pattern takes the attributes and content it lacks from the pattern its href names.
func TestPatternHref(t *testing.T) {
	canvas := patternSVG(t, `<pattern id="base" width="4" height="4" patternUnits="userSpaceOnUse"><rect width="2" height="4"/></pattern>`+
		`<pattern id="wider" href="#base" width="8"/><rect width="20" height="20" fill="url(#wider)"/>`)
	expectGrays(t, "href", canvas, map[image.Point]uint8{{1, 1}: 0, {3, 1}: 255, {5, 1}: 255, {9, 1}: 0})
}

// The pattern fill comes in its place among the shapes: over those before it, which show
// where the tile is empty, under its own stroke and the shapes after it; with its opacity.
func TestPatternPaintOrder(t *testing.T) {
	canvas := patternSVG(t, `<pattern id="stripes" width="4" height="4" patternUnits="userSpaceOnUse"><rect width="2" height="4"/></pattern>`+
		`<rect width="20" height="20" fill="#808080"/>`+
		`<rect width="16" height="16" fill="url(#stripes)" stroke="white" stroke-width="2"/>`+
		`<rect x="8" width="4" height="4" fill="white"/>`+
		`<rect y="18" width="20" height="2" fill="url(#stripes)" fill-opacity="0.5"/>`)
	expectGrays(t, "paint order", canvas, map[image.Point]uint8{
		{4, 8}: 0, {6, 8}: 128, // the stripes, the gray under them
		{9, 1}:  255, // a shape after them
		{15, 8}: 255, // their stroke
		{17, 8}: 128, // past them
		{4, 19}: 64,  // half black over gray
	})
}

// Past the caps, a pattern fill keeps its flat color: the fill of its largest shape.
func TestPatternCaps(t *testing.T) {
	// Tiled, the tiles half black would be gray.
	tiny := patternSVG(t, `<pattern id="dust" width=".001" height=".001"><rect width=".01" height="1"/></pattern>`+
		`<rect width="20" height="20" fill="url(#dust)"/>`)
	expectGrays(t, "too many tiles", tiny, map[image.Point]uint8{{5, 5}: 0, {15, 15}: 0})
	huge := patternSVG(t, `<pattern id="wide" width="4000" height="4000" patternUnits="userSpaceOnUse"><rect width="1" height="1"/><rect x="10" width="3000" height="3000" fill="#808080"/></pattern>`+
		`<rect width="20" height="20" fill="url(#wide)"/>`)
	expectGrays(t, "tile too large", huge, map[image.Point]uint8{{5, 5}: 128, {0, 0}: 128})

	// Each 2×2 square is tiled with a pattern black on its first pixel, but those past
	// maxPatternFills, flat black.
	var shapes strings.Builder
	shapes.WriteString(`<pattern id="dot" width="2" height="2" patternUnits="userSpaceOnUse"><rect width="1" height="1"/></pattern>`)
	count := maxPatternFills + 2
	for index := range count {
		fmt.Fprintf(&shapes, `<rect x="%d" y="%d" width="2" height="2" fill="url(#dot)"/>`, 2*(index%10), 2*(index/10))
	}
	canvas := patternSVG(t, shapes.String())
	for index := range count {
		want := uint8(255)
		if index >= maxPatternFills {
			want = 0
		}
		x, y := 2*(index%10), 2*(index/10)
		expectGrays(t, fmt.Sprint("fill ", index), canvas, map[image.Point]uint8{{x, y}: 0, {x + 1, y + 1}: want})
	}
}
