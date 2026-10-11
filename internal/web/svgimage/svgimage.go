package svgimage

import (
	"bytes"
	"encoding/xml"
	"errors"
	"image"
	"math"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html/charset"
)

// Browsers' size for an SVG that gives neither its size nor a viewBox, in pixels.
const (
	defaultWidth  = 300
	defaultHeight = 150
)

var errNotSVG = errors.New("not an SVG image")

// IsSVG reports whether data is an SVG image: its first element, after the XML declaration,
// comments and doctype, is <svg>.
func IsSVG(data []byte) bool {
	byteOrderMark := []byte("\xef\xbb\xbf")
	text := bytes.TrimLeft(bytes.TrimPrefix(data, byteOrderMark), " \t\r\n")
	if !bytes.HasPrefix(text, []byte("<")) { // spares running the XML decoder on a JPEG
		return false
	}
	_, err := readRoot(newDecoder(data))
	return err == nil
}

// Size returns the size of an SVG image in pixels: its width and height, else its viewBox's,
// else 300×150.
func Size(data []byte) (image.Point, error) {
	root, err := readRoot(newDecoder(data))
	if err != nil {
		return image.Point{}, err
	}
	width, height := root.size()
	return image.Pt(max(int(math.Round(width)), 1), max(int(math.Round(height)), 1)), nil
}

// newDecoder returns a lenient XML decoder for data, which leaves the entities it does not
// know as they are, and reads the charsets that x/text knows.
func newDecoder(data []byte) *xml.Decoder {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false
	decoder.CharsetReader = charset.NewReaderLabel
	return decoder
}

// svgRoot is what the root <svg> element says of the picture's size and placement.
type svgRoot struct {
	width, height float64 // in pixels, 0 when missing or a percentage
	viewBox       bounds  // empty when missing
	aspect        aspectRatio
}

// readRoot reads the root <svg> element from decoder.
func readRoot(decoder *xml.Decoder) (svgRoot, error) {
	for {
		token, err := decoder.Token()
		if err != nil {
			return svgRoot{}, errNotSVG
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "svg" {
			return svgRoot{}, errNotSVG
		}
		root := svgRoot{aspect: parseAspectRatio("")}
		for _, attribute := range start.Attr {
			switch attribute.Name.Local {
			case "width":
				root.width = positiveLength(attribute.Value)
			case "height":
				root.height = positiveLength(attribute.Value)
			case "viewBox":
				root.viewBox = parseViewBox(attribute.Value)
			case "preserveAspectRatio":
				root.aspect = parseAspectRatio(attribute.Value)
			}
		}
		return root, nil
	}
}

// size returns the picture's size in pixels: a missing width or height follows the viewBox's
// aspect ratio, or is the viewBox's, or the browsers' default.
func (root svgRoot) size() (float64, float64) {
	width, height := root.width, root.height
	if !root.viewBox.empty() {
		switch {
		case width == 0 && height == 0:
			width, height = root.viewBox.width, root.viewBox.height
		case width == 0:
			width = height * root.viewBox.width / root.viewBox.height
		case height == 0:
			height = width * root.viewBox.height / root.viewBox.width
		}
	}
	if width == 0 {
		width = defaultWidth
	}
	if height == 0 {
		height = defaultHeight
	}
	return width, height
}

// pixelsPer is the size of each unit of length in pixels, 96 to the inch; em and ex take a
// 16-pixel font.
var pixelsPer = map[string]float64{
	"": 1, "px": 1, "pt": 96.0 / 72, "pc": 16, "in": 96, "cm": 96 / 2.54, "mm": 96 / 25.4,
	"q": 96 / 101.6, "em": 16, "ex": 8,
}

// parseLength returns a length in pixels (or user units, which are pixels without a unit),
// false for a percentage or what is not a length.
func parseLength(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	number := strings.TrimRightFunc(value, unicode.IsLetter)
	scale, ok := pixelsPer[strings.ToLower(value[len(number):])]
	if !ok {
		return 0, false
	}
	length, err := strconv.ParseFloat(number, 64)
	if err != nil || math.IsNaN(length) || math.IsInf(length, 0) {
		return 0, false
	}
	return length * scale, true
}

// positiveLength is parseLength for a width or height: 0 when it is not a positive length.
func positiveLength(value string) float64 {
	length, ok := parseLength(value)
	if !ok || length <= 0 {
		return 0
	}
	return length
}

// bounds is a rectangle with float coordinates, in user units or in pixels: a viewBox, a
// viewport, the area of an image or of a pattern's tile.
type bounds struct{ x, y, width, height float64 }

func (area bounds) empty() bool { return area.width <= 0 || area.height <= 0 }

// rectangle returns area's pixels, its edges rounded.
func (area bounds) rectangle() image.Rectangle {
	return image.Rect(int(math.Round(area.x)), int(math.Round(area.y)),
		int(math.Round(area.x+area.width)), int(math.Round(area.y+area.height)))
}

// parseViewBox returns the box of a viewBox attribute, "x y width height" with spaces or
// commas, empty if it is not one.
func parseViewBox(value string) bounds {
	fields := strings.FieldsFunc(value, func(character rune) bool {
		return character == ',' || unicode.IsSpace(character)
	})
	if len(fields) != 4 {
		return bounds{}
	}
	var numbers [4]float64
	for index, field := range fields {
		number, err := strconv.ParseFloat(field, 64)
		if err != nil {
			return bounds{}
		}
		numbers[index] = number
	}
	return bounds{x: numbers[0], y: numbers[1], width: numbers[2], height: numbers[3]}
}

// aspectRatio is a preserveAspectRatio attribute: how content is scaled and aligned in a
// viewport of another shape.
type aspectRatio struct {
	none           bool    // stretched to fill the viewport
	alignX, alignY float64 // 0 at the viewport's left or top, 0.5 in its middle, 1 at its right or bottom
	slice          bool    // scaled to cover the viewport, else to fit in it
}

// alignments are the parts of preserveAspectRatio's xMidYMid and the like.
var alignments = map[string]float64{"Min": 0, "Mid": 0.5, "Max": 1}

// parseAspectRatio returns the aspect ratio of a preserveAspectRatio attribute: xMidYMid
// meet if it is empty or not one.
func parseAspectRatio(value string) aspectRatio {
	ratio := aspectRatio{alignX: 0.5, alignY: 0.5}
	fields := strings.Fields(value)
	if len(fields) > 0 && fields[0] == "defer" {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return ratio
	}
	if fields[0] == "none" {
		ratio.none = true
	} else if align := fields[0]; len(align) == len("xMidYMid") && align[0] == 'x' && align[4] == 'Y' {
		alignX, okX := alignments[align[1:4]]
		alignY, okY := alignments[align[5:8]]
		if okX && okY {
			ratio.alignX, ratio.alignY = alignX, alignY
		}
	}
	ratio.slice = len(fields) > 1 && fields[1] == "slice"
	return ratio
}

// transform maps a point to x × scaleX + offsetX, y × scaleY + offsetY.
type transform struct{ scaleX, scaleY, offsetX, offsetY float64 }

func (mapping transform) apply(area bounds) bounds {
	return bounds{
		x: area.x*mapping.scaleX + mapping.offsetX, y: area.y*mapping.scaleY + mapping.offsetY,
		width: area.width * mapping.scaleX, height: area.height * mapping.scaleY,
	}
}

// place returns the transform that puts content in viewport as ratio says.
func (ratio aspectRatio) place(content, viewport bounds) transform {
	scaleX, scaleY := viewport.width/content.width, viewport.height/content.height
	if !ratio.none {
		scale := min(scaleX, scaleY)
		if ratio.slice {
			scale = max(scaleX, scaleY)
		}
		scaleX, scaleY = scale, scale
	}
	return transform{
		scaleX: scaleX, scaleY: scaleY,
		offsetX: viewport.x + (viewport.width-content.width*scaleX)*ratio.alignX - content.x*scaleX,
		offsetY: viewport.y + (viewport.height-content.height*scaleY)*ratio.alignY - content.y*scaleY,
	}
}
