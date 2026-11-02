package layout

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"golang.org/x/image/vector"
)

// MarkShape is the shape of a CSS text-emphasis mark that is a keyword (dot, circle…). Such
// marks are drawn as shapes, not text: few fonts have the sesame, and in a font that has them
// the marks are not the size and place CSS gives them.
type MarkShape uint8

// The mark shapes, each filled or open.
const (
	NoMark MarkShape = iota
	DotMark
	CircleMark
	DoubleCircleMark
	TriangleMark
	SesameMark
)

// markShape is a keyword mark's shape, and whether it is open (an outline) rather than filled.
type markShape struct {
	shape MarkShape
	open  bool
}

// keywordMarks are the characters CSS's keywords stand for (document.Emphasis.Mark), by shape:
// filled and open dot, circle, double circle, triangle and sesame.
var keywordMarks = map[rune]markShape{
	'•': {DotMark, false}, '◦': {DotMark, true},
	'●': {CircleMark, false}, '○': {CircleMark, true},
	'◉': {DoubleCircleMark, false}, '◎': {DoubleCircleMark, true},
	'▲': {TriangleMark, false}, '△': {TriangleMark, true},
	'﹅': {SesameMark, false}, '﹆': {SesameMark, true},
}

// drawMark draws the mark of word, a PlacedWord whose Mark is set: its shape in a square of
// side word.Size whose top left is at word.X, word.Top, in its gray. Sizes are fractions of
// that side, as CSS's marks are of the em of their text.
func drawMark(dst *image.Gray, word PlacedWord) {
	side := float32(word.Size)
	if side < 1 {
		return
	}
	bounds := image.Rect(word.X, word.Top, word.X+word.Size, word.Top+word.Size).Intersect(dst.Bounds())
	if bounds.Empty() {
		return
	}
	rasterizer := vector.NewRasterizer(word.Size, word.Size)
	centre := side / 2
	stroke := max(side*0.09, 1) // an outline's width
	ring := func(radius float32) {
		circle(rasterizer, centre, centre, radius, radius, 0, false)
		if word.MarkOpen {
			circle(rasterizer, centre, centre, radius-stroke, radius-stroke, 0, true)
		}
	}
	switch word.Mark {
	case DotMark:
		ring(side * 0.18)
	case CircleMark:
		ring(side * 0.4)
	case DoubleCircleMark:
		circle(rasterizer, centre, centre, side*0.42, side*0.42, 0, false)
		circle(rasterizer, centre, centre, side*0.42-stroke, side*0.42-stroke, 0, true)
		ring(side * 0.22)
	case TriangleMark:
		// Pointing up, its centre (where its medians cross) in the square's centre.
		height := side * 0.75
		corners := [3][2]float32{{centre, centre - height*2/3}, {centre + height/sqrt3, centre + height/3}, {centre - height/sqrt3, centre + height/3}}
		triangle(rasterizer, corners)
		if word.MarkOpen {
			// The same triangle shrunk about its centre, its sides stroke inside the first's,
			// and gone round the other way.
			inradius := height / 3
			scale := max(inradius-stroke, 0) / inradius
			var inner [3][2]float32
			for index, corner := range corners {
				inner[2-index] = [2]float32{centre + (corner[0]-centre)*scale, centre + (corner[1]-centre)*scale}
			}
			triangle(rasterizer, inner)
		}
	case SesameMark:
		// A grain leaning to the right, as the CJK sesame is drawn.
		circle(rasterizer, centre, centre, side*0.18, side*0.34, -math.Pi/5, false)
		if word.MarkOpen {
			circle(rasterizer, centre, centre, side*0.18-stroke, side*0.34-stroke, -math.Pi/5, true)
		}
	default:
		return
	}
	mask := image.NewAlpha(image.Rect(0, 0, word.Size, word.Size))
	rasterizer.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
	draw.DrawMask(dst, bounds, image.NewUniform(color.Gray{Y: word.Gray}), image.Point{}, mask, bounds.Min.Sub(image.Pt(word.X, word.Top)), draw.Over)
}

// circle adds an ellipse to rasterizer as a polygon: centred at x, y, of radii radiusX and
// radiusY, turned by angle (radians, clockwise on the screen). Reversed, it goes the other way
// round, which cuts a hole in a shape around it.
func circle(rasterizer *vector.Rasterizer, x, y, radiusX, radiusY float32, angle float64, reversed bool) {
	if radiusX <= 0 || radiusY <= 0 {
		return
	}
	const sides = 32
	sine, cosine := math.Sincos(angle)
	for index := range sides + 1 {
		turn := 2 * math.Pi * float64(index) / sides
		if reversed {
			turn = -turn
		}
		pointX, pointY := float64(radiusX)*math.Cos(turn), float64(radiusY)*math.Sin(turn)
		turnedX, turnedY := x+float32(pointX*cosine-pointY*sine), y+float32(pointX*sine+pointY*cosine)
		if index == 0 {
			rasterizer.MoveTo(turnedX, turnedY)
		} else {
			rasterizer.LineTo(turnedX, turnedY)
		}
	}
	rasterizer.ClosePath()
}

// triangle adds a triangle to rasterizer, its corners (x, y) in the order given.
func triangle(rasterizer *vector.Rasterizer, corners [3][2]float32) {
	rasterizer.MoveTo(corners[0][0], corners[0][1])
	rasterizer.LineTo(corners[1][0], corners[1][1])
	rasterizer.LineTo(corners[2][0], corners[2][1])
	rasterizer.ClosePath()
}

// sqrt3 is the square root of 3: an equilateral triangle's side is its height times 2/sqrt3.
const sqrt3 = 1.7320508
