package svgimage

import (
	"image"
	"math"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// maxTextSize caps the size text is drawn at, in pixels.
const maxTextSize = 2000

// TextStyle is how a part of an SVG's text is drawn.
type TextStyle struct {
	Families     []string // font-family's, in order; none for the reader's typeface
	Size         int      // in pixels
	Bold, Italic bool
	Gray         uint8
}

// TextDrawer measures and draws an SVG's text in the reader's fonts, with fallbacks for the
// characters a font lacks. internal/typesetting/layout's Faces give one.
type TextDrawer interface {
	// Width returns the width of text in pixels.
	Width(style TextStyle, text string) int
	// Draw draws text into dst, the left end of its baseline at origin.
	Draw(dst *image.Gray, style TextStyle, origin image.Point, text string) error
}

// textElement is a <text> of an SVG, taken out before oksvg draws the rest: its spans, laid
// out once the fonts are known.
type textElement struct {
	spans     []textSpan
	transform matrix // its user space's to the root's
}

// textSpan is a run of text in one style: the text of a <text> or a <tspan>.
type textSpan struct {
	text   string
	x, y   float64 // where its first character goes, NaN if not given: it follows the last
	dx, dy float64 // then moved by
	style  inheritedStyle
	gray   uint8
	drawn  bool // not hidden, nor without fill
}

// addText adds the text of a <text> element, with the transform of its user space.
func (work *styler) addText(element *node, transform matrix) {
	text := textElement{transform: transform}
	work.addSpans(&text, element)
	collapseWhitespace(text.spans)
	work.texts = append(work.texts, text)
}

// textContainers are the elements whose text is part of a <text>'s.
var textContainers = map[string]bool{"tspan": true, "a": true, "textpath": true}

// addSpans adds the spans of element's content: its text nodes, and those of the elements
// it holds. Its x, y, dx and dy (their first value) go to its first span.
func (work *styler) addSpans(text *textElement, element *node) {
	first := len(text.spans)
	for _, child := range element.children {
		switch {
		case child.local == "":
			text.spans = append(text.spans, work.span(element, child.text))
		case textContainers[child.local] && !strings.EqualFold(child.values["display"], "none"):
			work.addSpans(text, child)
		}
	}
	if first == len(text.spans) {
		return
	}
	span := &text.spans[first]
	if x, ok := firstLength(element, "x"); ok {
		span.x = x
	}
	if y, ok := firstLength(element, "y"); ok {
		span.y = y
	}
	if dx, ok := firstLength(element, "dx"); ok {
		span.dx += dx
	}
	if dy, ok := firstLength(element, "dy"); ok {
		span.dy += dy
	}
}

// span returns a span of content in the style of element.
func (work *styler) span(element *node, content string) textSpan {
	style := element.inherited
	span := textSpan{text: content, x: math.NaN(), y: math.NaN(), style: style}
	painted, ok := work.paintOf(style.fill, style.color)
	if !ok || painted.value == "none" || style.hidden {
		return span
	}
	if gradient, isGradient := strings.CutPrefix(painted.value, "url(#"); isGradient {
		painted = work.gradientPaint(strings.TrimSuffix(gradient, ")"))
	}
	color, ok := css.ParseColor(painted.value, css.Black)
	if !ok {
		return span
	}
	// Over white, as the page under it most often is.
	opacity := painted.opacity * style.fillOpacity
	span.gray = uint8(math.Round(255 - (255-float64(color.Gray))*opacity))
	span.drawn = opacity > 0
	return span
}

// firstLength returns the first length of an x, y, dx or dy attribute, in user units.
func firstLength(element *node, name string) (float64, bool) {
	value, ok := element.Attribute(name)
	if !ok {
		return 0, false
	}
	fields := strings.FieldsFunc(value, func(character rune) bool { return character == ',' || character == ' ' })
	if len(fields) == 0 {
		return 0, false
	}
	return lengthOf(fields[0], element.inherited.fontSize)
}

// collapseWhitespace collapses the whitespace of a text's spans, as xml:space="default" does:
// newlines out, tabs as spaces, runs of spaces as one, none at the start or the end.
func collapseWhitespace(spans []textSpan) {
	afterSpace := true // a space at the start goes
	lastSpace := -1    // the span that ends with a space, if it is the last character so far
	for index := range spans {
		var collapsed strings.Builder
		for _, character := range spans[index].text {
			switch character {
			case '\n', '\r':
				continue
			case '\t', ' ':
				if afterSpace {
					continue
				}
				afterSpace = true
				collapsed.WriteByte(' ')
			default:
				afterSpace = false
				collapsed.WriteRune(character)
			}
		}
		spans[index].text = collapsed.String()
		if spans[index].text != "" {
			lastSpace = -1
			if afterSpace {
				lastSpace = index
			}
		}
	}
	if lastSpace >= 0 {
		spans[lastSpace].text = strings.TrimSuffix(spans[lastSpace].text, " ")
	}
}

// placedSpan is a span placed in its text's user space.
type placedSpan struct {
	span  *textSpan
	x, y  float64
	style TextStyle
}

// drawTexts draws the texts of an SVG into canvas, userToPixels mapping its root's user space
// to the canvas. Glyphs stay upright whatever the transforms: only their positions follow.
func drawTexts(canvas *image.Gray, texts []textElement, userToPixels matrix, drawer TextDrawer) {
	for _, text := range texts {
		toPixels := userToPixels.multiply(text.transform)
		scale := toPixels.scale()
		if scale == 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
			continue
		}
		var chunk []placedSpan // the spans since the last absolute x, aligned together
		chunkWidth := 0.0
		flush := func() {
			if len(chunk) == 0 {
				return
			}
			shift := 0.0
			switch chunk[0].span.style.anchor {
			case "middle":
				shift = -chunkWidth / 2
			case "end":
				shift = -chunkWidth
			}
			for _, placed := range chunk {
				if !placed.span.drawn || placed.style.Size < 1 || placed.span.text == "" {
					continue
				}
				x, y := toPixels.apply(placed.x+shift, placed.y)
				if math.Abs(x) > 1e6 || math.Abs(y) > 1e6 || math.IsNaN(x) || math.IsNaN(y) {
					continue
				}
				_ = drawer.Draw(canvas, placed.style, image.Pt(int(math.Round(x)), int(math.Round(y))), placed.span.text) // left out if it fails
			}
			chunk, chunkWidth = chunk[:0], 0
		}
		penX, penY := 0.0, 0.0
		for index := range text.spans {
			span := &text.spans[index]
			if !math.IsNaN(span.x) {
				flush()
				penX = span.x
			}
			if !math.IsNaN(span.y) {
				penY = span.y
			}
			penX, penY = penX+span.dx, penY+span.dy
			style := TextStyle{
				Families: span.style.families, Size: int(math.Round(min(span.style.fontSize*scale, maxTextSize))),
				Bold: span.style.bold, Italic: span.style.italic, Gray: span.gray,
			}
			width := 0.0
			if span.text != "" && style.Size >= 1 {
				width = float64(drawer.Width(style, span.text)) / scale
			}
			chunk = append(chunk, placedSpan{span: span, x: penX, y: penY, style: style})
			chunkWidth += width
			penX += width
		}
		flush()
	}
}
