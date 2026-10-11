package mathml

import (
	"image"
	"math"

	"golang.org/x/image/vector"
)

// white is the gray of the paper, black that of the ink.
const (
	white = 0xff
	black = 0x00
)

// Draw draws the formula into dst, in black, the left end of its baseline at origin. What
// falls out of dst is clipped.
func (formula *Formula) Draw(dst *image.Gray, origin image.Point) {
	for _, placed := range formula.items {
		switch placed.kind {
		case textItem:
			formula.fonts.Draw(dst, placed.text, placed.variant, placed.size, origin.Add(placed.at))
		case ruleItem:
			fill(dst, placed.rect.Add(origin))
		case strokeItem:
			drawStroke(dst, placed.points, placed.thickness, origin)
		case stretchItem:
			formula.drawStretched(dst, placed, origin)
		}
	}
}

// Image returns the formula drawn on white, its baseline Ascent pixels from its top.
func (formula *Formula) Image() *image.Gray {
	drawn := image.NewGray(image.Rect(0, 0, max(formula.Width, 1), max(formula.Ascent+formula.Descent, 1)))
	for index := range drawn.Pix {
		drawn.Pix[index] = white
	}
	formula.Draw(drawn, image.Pt(0, formula.Ascent))
	return drawn
}

// fill paints rect black, clipped to dst.
func fill(dst *image.Gray, rect image.Rectangle) {
	rect = rect.Intersect(dst.Rect)
	for row := rect.Min.Y; row < rect.Max.Y; row++ {
		pixels := dst.Pix[dst.PixOffset(rect.Min.X, row):][:rect.Dx()]
		for index := range pixels {
			pixels[index] = black
		}
	}
}

// drawStroke draws a polyline through points, thickness pixels wide, moved by origin: each
// segment a quadrilateral, antialiased.
func drawStroke(dst *image.Gray, points []point, thickness float64, origin image.Point) {
	if len(points) < 2 {
		return
	}
	bounds := image.Rectangle{}
	for index, at := range points {
		corner := image.Pt(int(math.Floor(at.x-thickness)), int(math.Floor(at.y-thickness)))
		extent := image.Rectangle{Min: corner, Max: image.Pt(int(math.Ceil(at.x+thickness))+1, int(math.Ceil(at.y+thickness))+1)}
		if index == 0 {
			bounds = extent
		} else {
			bounds = bounds.Union(extent)
		}
	}
	clipped := bounds.Add(origin).Intersect(dst.Rect)
	if clipped.Empty() {
		return
	}
	rasterizer := vector.NewRasterizer(bounds.Dx(), bounds.Dy())
	half := thickness / 2
	for index := range len(points) - 1 {
		from, to := points[index], points[index+1]
		length := math.Hypot(to.x-from.x, to.y-from.y)
		if length == 0 {
			continue
		}
		// The normal of the segment, half the thickness long.
		normalX, normalY := -(to.y-from.y)/length*half, (to.x-from.x)/length*half
		offsetX, offsetY := float64(bounds.Min.X), float64(bounds.Min.Y)
		rasterizer.MoveTo(float32(from.x+normalX-offsetX), float32(from.y+normalY-offsetY))
		rasterizer.LineTo(float32(to.x+normalX-offsetX), float32(to.y+normalY-offsetY))
		rasterizer.LineTo(float32(to.x-normalX-offsetX), float32(to.y-normalY-offsetY))
		rasterizer.LineTo(float32(from.x-normalX-offsetX), float32(from.y-normalY-offsetY))
		rasterizer.ClosePath()
	}
	mask := image.NewAlpha(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	rasterizer.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
	darken(dst, clipped, func(x, y int) uint8 {
		return white - mask.Pix[mask.PixOffset(x-origin.X-bounds.Min.X, y-origin.Y-bounds.Min.Y)]
	})
}

// darken makes each pixel of rect in dst as dark as gray's, if it is darker.
func darken(dst *image.Gray, rect image.Rectangle, gray func(x, y int) uint8) {
	for row := rect.Min.Y; row < rect.Max.Y; row++ {
		pixels := dst.Pix[dst.PixOffset(rect.Min.X, row):][:rect.Dx()]
		for index := range pixels {
			pixels[index] = min(pixels[index], gray(rect.Min.X+index, row))
		}
	}
}

// drawStretched draws a stretched glyph: drawn at its size, then its ink scaled into its
// rectangle, up and down only or across only.
func (formula *Formula) drawStretched(dst *image.Gray, placed item, origin image.Point) {
	target := placed.rect.Add(origin)
	if target.Empty() || target.Intersect(dst.Rect).Empty() {
		return
	}
	_, measured := formula.fonts.Measure(placed.text, placed.variant, placed.size)
	if measured.Empty() {
		return
	}
	// The glyph drawn with a margin, then its ink measured as drawn.
	margin := placed.size / 4
	glyph := image.NewGray(image.Rect(0, 0, measured.Dx()+2*margin, measured.Dy()+2*margin))
	for index := range glyph.Pix {
		glyph.Pix[index] = white
	}
	pen := image.Pt(margin, margin).Sub(measured.Min)
	formula.fonts.Draw(glyph, placed.text, placed.variant, placed.size, pen)
	ink := inkBounds(glyph)
	if ink.Empty() {
		return
	}
	// Where the ink goes: the target along the stretch, the glyph's own place across it.
	destination := target
	if placed.vertical {
		destination.Min.X, destination.Max.X = target.Min.X+ink.Min.X-pen.X, target.Min.X+ink.Max.X-pen.X
	} else {
		baseline := origin.Y + placed.at.Y
		destination.Min.Y, destination.Max.Y = baseline+ink.Min.Y-pen.Y, baseline+ink.Max.Y-pen.Y
	}
	scaleX := float64(ink.Dx()) / float64(destination.Dx())
	scaleY := float64(ink.Dy()) / float64(destination.Dy())
	darken(dst, destination.Intersect(dst.Rect), func(x, y int) uint8 {
		return sample(glyph, ink, float64(x-destination.Min.X)*scaleX, float64(y-destination.Min.Y)*scaleY, scaleX, scaleY)
	})
}

// sample returns the gray of glyph's ink box at (x, y) from its corner, averaged over the
// scaleX by scaleY pixels a destination pixel covers (at least one).
func sample(glyph *image.Gray, ink image.Rectangle, x, y, scaleX, scaleY float64) uint8 {
	left, top := ink.Min.X+int(x), ink.Min.Y+int(y)
	right, bottom := max(ink.Min.X+int(x+scaleX), left+1), max(ink.Min.Y+int(y+scaleY), top+1)
	total, count := 0, 0
	for row := top; row < min(bottom, ink.Max.Y); row++ {
		for column := left; column < min(right, ink.Max.X); column++ {
			total += int(glyph.Pix[glyph.PixOffset(column, row)])
			count++
		}
	}
	if count == 0 {
		return white
	}
	return uint8(total / count)
}

// inkBounds returns the rectangle of an image's pixels that are not white.
func inkBounds(drawn *image.Gray) image.Rectangle {
	ink := image.Rectangle{}
	for row := drawn.Rect.Min.Y; row < drawn.Rect.Max.Y; row++ {
		pixels := drawn.Pix[drawn.PixOffset(drawn.Rect.Min.X, row):][:drawn.Rect.Dx()]
		for index, gray := range pixels {
			if gray != white {
				ink = ink.Union(image.Rect(drawn.Rect.Min.X+index, row, drawn.Rect.Min.X+index+1, row+1))
			}
		}
	}
	return ink
}
