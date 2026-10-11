package svgimage

import (
	"bytes"
	"image"
	"math"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"

	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// Pattern fills. oksvg has no patterns, so the styler cuts the SVG into layers at each shape
// filled with one: the layers are drawn in order, and between two the shape's fill, tiled by
// Rasterize: the shape alone filled black is a mask, the pattern's tile is drawn on its own,
// and the tile is repeated over the canvas where the mask is set. The shape's stroke is in
// the next layer, over its fill. A pattern inside a tile, a stroke, or a fill past the caps
// below keeps the pattern's flat color (patternPaint).

// The caps on pattern fills, past which they keep their flat color: maxPatternFills of them,
// as long as the styler's passes write fewer than maxPatternWork elements in all, and their
// shapes' boxes cover maxPatternCanvases canvases in all; a tile drawn in at most
// maxTilePixels pixels, repeated at most maxPatternTiles times over its shape's box;
// maxPatternLinks patterns followed by href.
const (
	maxPatternFills    = 16
	maxPatternCanvases = 4
	maxPatternWork     = 2_000_000
	maxTilePixels      = 1 << 21
	maxPatternTiles    = 1 << 16
	maxPatternLinks    = 16
)

// foundFill is a shape filled with a pattern, found in the styler's first pass.
type foundFill struct {
	index     int // among the shapes, in document order
	transform matrix
	pattern   *node
}

// patternFill is a shape's pattern fill, drawn by Rasterize.
type patternFill struct {
	mask      []byte // the SVG with only the shape, filled black, with its fill's opacities
	flat      []byte // the SVG with only the shape's fill, in the pattern's flat color
	transform matrix // the shape's user space's to the root's
	tile      patternTile
	tiled     bool // else the flat color is drawn
}

// patternTile is a pattern's tile, its attributes followed through href.
type patternTile struct {
	area               bounds // its x, y, width and height: fractions of the shape's box unless userSpace
	userSpace          bool   // patternUnits="userSpaceOnUse"
	contentBoundingBox bool   // patternContentUnits="objectBoundingBox"
	transform          matrix
	viewBox            bounds // empty when missing
	aspect             aspectRatio
	markup             []byte // its content as an SVG of its own; nil paints nothing
}

// patternAttributes are the attributes of a <pattern> that one names by href inherits.
var patternAttributes = []string{
	"x", "y", "width", "height", "patternUnits", "patternContentUnits", "patternTransform",
	"viewBox", "preserveAspectRatio",
}

// patternOf returns the <pattern> a paint value names, nil if it names none.
func (work *styler) patternOf(value string) *node {
	address, ok := css.URL(strings.TrimSpace(value))
	if !ok {
		return nil
	}
	id, local := strings.CutPrefix(address, "#")
	if server, found := work.ids[id]; local && found && server.local == "pattern" {
		return server
	}
	return nil
}

// findFill notes element, the shape numbered index, if a pattern fills it, transform being
// its user space's to the root's.
func (work *styler) findFill(element *node, index int, transform matrix) {
	if len(work.found) >= maxPatternFills || element.local == "use" || element.local == "line" || element.inherited.hidden {
		return
	}
	if pattern := work.patternOf(element.inherited.fill); pattern != nil {
		work.found = append(work.found, foundFill{index: index, transform: transform, pattern: pattern})
	}
}

// layers writes the passes of an SVG with pattern fills, found by the first pass: the layers
// cut at each, and each fill's mask, flat fill and tile.
func (work *styler) layers(root *node) ([][]byte, []patternFill) {
	pass := func(modeOf func(index int) shapeMode) []byte {
		work.output.Reset()
		work.shapes, work.modeOf = 0, modeOf
		work.write(root, identity, false)
		return bytes.Clone(work.output.Bytes())
	}
	found := work.found
	// Each fill costs three passes, the layers one more: as many fills as fit in the work.
	elements := max(countElements(root), 1)
	found = found[:min(len(found), max((maxPatternWork/elements-1)/3, 0))]
	if len(found) == 0 {
		return nil, nil
	}
	layers := make([][]byte, 0, len(found)+1)
	for layer := range len(found) + 1 {
		after, before := -1, math.MaxInt
		if layer > 0 {
			after = found[layer-1].index
		}
		if layer < len(found) {
			before = found[layer].index
		}
		layers = append(layers, pass(func(index int) shapeMode {
			switch {
			case index == after:
				return shapeStrokeOnly
			case index > after && index < before:
				return shapeDrawn
			}
			return shapeLeftOut
		}))
	}
	fills := make([]patternFill, 0, len(found))
	for _, shape := range found {
		only := func(mode shapeMode) func(index int) shapeMode {
			return func(index int) shapeMode {
				if index == shape.index {
					return mode
				}
				return shapeLeftOut
			}
		}
		fill := patternFill{mask: pass(only(shapeMask)), flat: pass(only(shapeFlatFill)), transform: shape.transform}
		fill.tile, fill.tiled = work.tileOf(root, shape.pattern)
		fills = append(fills, fill)
	}
	work.modeOf = nil
	return layers, fills
}

// countElements returns the number of elements and text nodes under root, root included.
func countElements(root *node) int {
	count := 1
	for _, child := range root.children {
		count += countElements(child)
	}
	return count
}

// tileOf returns the tile of pattern, its attributes and content inherited from the patterns
// its href names when it lacks them; false if it cannot be drawn tiled.
func (work *styler) tileOf(root, pattern *node) (patternTile, bool) {
	attributes, content := work.followPatternLinks(pattern)
	tile, ok := tileFrom(attributes)
	if !ok {
		return patternTile{}, false
	}
	if content == nil || tile.area.width <= 0 || tile.area.height <= 0 {
		return tile, true // paints nothing
	}
	tile.markup = work.tileMarkup(root, content)
	return tile, true
}

// followPatternLinks returns the attributes of a pattern, each from the pattern itself or else
// from the first pattern down its chain of href links that has it, and the first pattern of
// the chain that has content.
func (work *styler) followPatternLinks(pattern *node) (attributes map[string]string, content *node) {
	attributes = map[string]string{}
	seen := map[*node]bool{}
	for current := pattern; current != nil && !seen[current] && len(seen) < maxPatternLinks; {
		seen[current] = true
		for _, name := range patternAttributes {
			if _, taken := attributes[name]; !taken {
				if value, ok := current.Attribute(name); ok {
					attributes[name] = strings.TrimSpace(value)
				}
			}
		}
		if content == nil && current.lastElement() != nil {
			content = current
		}
		href, _ := current.Attribute("href")
		next, found := work.ids[strings.TrimPrefix(strings.TrimSpace(href), "#")]
		if !found || next.local != "pattern" {
			break
		}
		current = next
	}
	return attributes, content
}

// tileFrom returns the tile that a pattern's attributes describe, without its content; false if
// a length doesn't parse.
func tileFrom(attributes map[string]string) (patternTile, bool) {
	tile := patternTile{
		userSpace:          attributes["patternUnits"] == "userSpaceOnUse",
		contentBoundingBox: attributes["patternContentUnits"] == "objectBoundingBox",
		viewBox:            parseViewBox(attributes["viewBox"]),
		aspect:             parseAspectRatio(attributes["preserveAspectRatio"]),
		transform:          identity,
	}
	if value, ok := attributes["patternTransform"]; ok {
		tile.transform, _ = parseTransform(value)
	}
	for _, length := range []struct {
		name  string
		value *float64
	}{{"x", &tile.area.x}, {"y", &tile.area.y}, {"width", &tile.area.width}, {"height", &tile.area.height}} {
		value, ok := attributes[length.name]
		if !ok {
			continue // 0
		}
		if *length.value, ok = patternLength(value, !tile.userSpace); !ok {
			return patternTile{}, false
		}
	}
	return tile, true
}

// tileMarkup returns the content of a pattern as an SVG of its own, with the SVG's gradients,
// which the content may paint with.
func (work *styler) tileMarkup(root, content *node) []byte {
	work.output.Reset()
	work.output.WriteString("<svg")
	for _, attribute := range root.attributes {
		if attribute.Name.Space == "xmlns" || attribute.Name.Space == "" && attribute.Name.Local == "xmlns" {
			writeAttribute(&work.output, attribute.Name, attribute.Value)
		}
	}
	work.output.WriteString("><defs>")
	for _, gradient := range work.gradients {
		work.write(gradient, identity, true)
	}
	work.output.WriteString("</defs><g>")
	for _, child := range content.children {
		work.write(child, identity, true)
	}
	work.output.WriteString("</g></svg>")
	return bytes.Clone(work.output.Bytes())
}

// patternLength returns a pattern's x, y, width or height: in user units, or for boundingBox
// a fraction of the box, a number or a percentage.
func patternLength(value string, boundingBox bool) (float64, bool) {
	if boundingBox {
		if percentage, ok := strings.CutSuffix(value, "%"); ok {
			fraction, ok := parseLength(percentage)
			return fraction / 100, ok
		}
	}
	return parseLength(value)
}

// drawShapes draws the shapes of styled on canvas, its user space mapped to pixels by
// toPixels: its layers and its pattern fills between them.
func drawShapes(canvas *image.RGBA, styled restyled, toPixels matrix) error {
	if len(styled.fills) == 0 {
		return drawMarkup(canvas, styled.markup, toPixels)
	}
	var mask *image.RGBA
	size := canvas.Bounds().Size()
	budget := maxPatternCanvases * size.X * size.Y // the pixels left to tile
	for index, layer := range styled.layers {
		if err := drawMarkup(canvas, layer, toPixels); err != nil {
			return err
		}
		if index == len(styled.fills) {
			break
		}
		if mask == nil {
			mask = image.NewRGBA(canvas.Bounds())
		} else {
			clear(mask.Pix)
		}
		if fill := styled.fills[index]; !drawPattern(canvas, mask, fill, toPixels, &budget) {
			_ = drawMarkup(canvas, fill.flat, toPixels) // drawn as a layer was, or left out
		}
	}
	return nil
}

// drawMarkup draws an SVG written by the styler on canvas.
func drawMarkup(canvas *image.RGBA, markup []byte, toPixels matrix) error {
	icon, err := oksvg.ReadIconStream(bytes.NewReader(markup))
	if err != nil {
		return err
	}
	drawIcon(canvas, icon, toPixels)
	return nil
}

// drawIcon draws what oksvg read on canvas. oksvg's own SetTarget misplaces a viewBox that
// does not start at 0, 0.
func drawIcon(canvas *image.RGBA, icon *oksvg.SvgIcon, toPixels matrix) {
	icon.Transform = rasterx.Matrix2D{A: toPixels.a, B: toPixels.b, C: toPixels.c, D: toPixels.d, E: toPixels.e, F: toPixels.f}
	size := canvas.Bounds().Size()
	scanner := rasterx.NewScannerGV(size.X, size.Y, canvas, canvas.Bounds())
	icon.Draw(rasterx.NewDasher(size.X, size.Y, scanner), 1)
}

// drawPattern draws a pattern fill on canvas, its shape's mask drawn in mask, cleared, the
// pixels it tiles taken from budget; false if it cannot be drawn tiled, for its flat color to be.
func drawPattern(canvas, mask *image.RGBA, fill patternFill, toPixels matrix, budget *int) bool {
	if !fill.tiled {
		return false
	}
	tile := fill.tile
	if tile.markup == nil {
		return true
	}
	shape, err := oksvg.ReadIconStream(bytes.NewReader(fill.mask))
	if err != nil {
		return false
	}
	shapeBounds, ok := pathBounds(shape.SVGPaths)
	if !ok {
		return true
	}
	area := tile.areaOn(shapeBounds)
	if area.empty() || math.IsInf(area.width, 0) || math.IsInf(area.height, 0) {
		return true // a shape without width or height: nothing to fill
	}
	shapeToPixels := toPixels.multiply(fill.transform)
	patternToPixels := shapeToPixels.multiply(tile.transform)
	pixelsToPattern, ok := patternToPixels.inverse()
	if !ok {
		return false
	}

	// The tile in pixels, as wide as the pattern's x axis is long on the canvas.
	tileWidth := math.Ceil(area.width * math.Hypot(patternToPixels.a, patternToPixels.b))
	tileHeight := math.Ceil(area.height * math.Hypot(patternToPixels.c, patternToPixels.d))
	if !(tileWidth*tileHeight <= maxTilePixels) { // NaN too
		return false
	}
	covered := pixelBox(shapeBounds, shapeToPixels).Intersect(canvas.Bounds())
	if covered.Empty() {
		return true
	}
	if covered.Dx()*covered.Dy() > *budget {
		return false
	}
	*budget -= covered.Dx() * covered.Dy()
	tilePixels := area.width * area.height * math.Abs(patternToPixels.a*patternToPixels.d-patternToPixels.b*patternToPixels.c)
	if !(float64(covered.Dx())*float64(covered.Dy()) <= maxPatternTiles*tilePixels) {
		return false
	}
	tileSize := image.Pt(max(int(tileWidth), 1), max(int(tileHeight), 1))
	tileImage, err := tile.draw(tileSize, area, shapeBounds)
	if err != nil {
		return false
	}

	drawIcon(mask, shape, toPixels)
	compositeTiled(canvas, mask, tileImage, covered, pixelsToPattern, area)
	return true
}

// areaOn returns the area of the tile in the pattern's space, for a shape of shapeBounds: its
// x, y, width and height are fractions of the shape's bounds, unless they are in user units.
func (tile *patternTile) areaOn(shapeBounds bounds) bounds {
	area := tile.area
	if tile.userSpace {
		return area
	}
	return bounds{
		x: shapeBounds.x + area.x*shapeBounds.width, y: shapeBounds.y + area.y*shapeBounds.height,
		width: area.width * shapeBounds.width, height: area.height * shapeBounds.height,
	}
}

// draw draws the tile's content once, in an image of size pixels that stands for area, its
// content mapped through its viewBox, or in fractions of shapeBounds.
func (tile *patternTile) draw(size image.Point, area, shapeBounds bounds) (*image.RGBA, error) {
	drawn := image.NewRGBA(image.Rectangle{Max: size})
	content := identity
	switch {
	case !tile.viewBox.empty():
		content = tile.aspect.place(tile.viewBox, bounds{width: area.width, height: area.height}).matrix()
	case tile.contentBoundingBox:
		content = matrix{a: shapeBounds.width, d: shapeBounds.height}
	}
	tileScale := matrix{a: float64(size.X) / area.width, d: float64(size.Y) / area.height}
	if err := drawMarkup(drawn, tile.markup, tileScale.multiply(content)); err != nil {
		return nil, err
	}
	return drawn, nil
}

// compositeTiled draws tile repeated over the pixels of covered, where mask is set:
// pixelsToPattern maps the canvas to the pattern's space, where the tile is area.
func compositeTiled(canvas, mask, tile *image.RGBA, covered image.Rectangle, pixelsToPattern matrix, area bounds) {
	tileSize := tile.Bounds().Size()
	for y := covered.Min.Y; y < covered.Max.Y; y++ {
		for x := covered.Min.X; x < covered.Max.X; x++ {
			offset := mask.PixOffset(x, y)
			coverage := uint32(mask.Pix[offset+3])
			if coverage == 0 {
				continue
			}
			patternX, patternY := pixelsToPattern.apply(float64(x)+0.5, float64(y)+0.5)
			column := tileIndex((patternX-area.x)/area.width, tileSize.X)
			row := tileIndex((patternY-area.y)/area.height, tileSize.Y)
			source := tile.Pix[tile.PixOffset(column, row):]
			alpha := coverage * uint32(source[3]) / 255
			if alpha == 0 {
				continue
			}
			pixel := canvas.Pix[offset : offset+3 : offset+3]
			for channel := range pixel {
				pixel[channel] = uint8((uint32(pixel[channel])*(255-alpha) + uint32(source[channel])*coverage) / 255)
			}
		}
	}
}

// tileIndex returns the column or row of a tile size pixels long at a position counted in
// tiles: its fractional part, in pixels.
func tileIndex(position float64, size int) int {
	fraction := position - math.Floor(position)
	return min(max(int(fraction*float64(size)), 0), size-1) // NaN to one end
}

// pixelBox returns the pixels that area covers once mapped by toPixels.
func pixelBox(area bounds, toPixels matrix) image.Rectangle {
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, corner := range [][2]float64{{area.x, area.y}, {area.x + area.width, area.y}, {area.x, area.y + area.height}, {area.x + area.width, area.y + area.height}} {
		x, y := toPixels.apply(corner[0], corner[1])
		minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
	}
	clamp := func(value float64) int { return int(min(max(value, -1<<24), 1<<24)) } // NaN too
	return image.Rect(clamp(math.Floor(minX))-1, clamp(math.Floor(minY))-1, clamp(math.Ceil(maxX))+1, clamp(math.Ceil(maxY))+1)
}

// curveSamples is the number of points a curve is sampled at for its box.
const curveSamples = 16

// pathBounds returns the box of the points of paths, in their own user units, their curves
// sampled; false if they have none.
func pathBounds(paths []oksvg.SvgPath) (bounds, bool) {
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	add := func(x, y float64) {
		minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
	}
	for _, path := range paths {
		points := path.Path
		point := func(index int) (float64, float64) { return float64(points[index]) / 64, float64(points[index+1]) / 64 }
		var currentX, currentY float64
		for index := 0; index < len(points); {
			switch rasterx.PathCommand(points[index]) {
			case rasterx.PathMoveTo, rasterx.PathLineTo:
				currentX, currentY = point(index + 1)
				add(currentX, currentY)
				index += 3
			case rasterx.PathQuadTo:
				controlX, controlY := point(index + 1)
				endX, endY := point(index + 3)
				for step := range curveSamples {
					along := float64(step+1) / curveSamples
					before := 1 - along
					add(before*before*currentX+2*before*along*controlX+along*along*endX,
						before*before*currentY+2*before*along*controlY+along*along*endY)
				}
				currentX, currentY = endX, endY
				index += 5
			case rasterx.PathCubicTo:
				firstX, firstY := point(index + 1)
				secondX, secondY := point(index + 3)
				endX, endY := point(index + 5)
				for step := range curveSamples {
					along := float64(step+1) / curveSamples
					before := 1 - along
					add(before*before*before*currentX+3*before*before*along*firstX+3*before*along*along*secondX+along*along*along*endX,
						before*before*before*currentY+3*before*before*along*firstY+3*before*along*along*secondY+along*along*along*endY)
				}
				currentX, currentY = endX, endY
				index += 7
			case rasterx.PathClose:
				index++
			default:
				return bounds{}, false
			}
		}
	}
	if minX > maxX {
		return bounds{}, false
	}
	return bounds{x: minX, y: minY, width: maxX - minX, height: maxY - minY}, true
}
