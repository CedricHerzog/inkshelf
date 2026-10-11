package svgimage

import (
	"maps"
	"math"
	"strconv"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// DocumentStyleAttribute is the attribute where package epub writes, in the form of a style
// attribute, the declarations that a chapter's stylesheets give an element of an <svg> drawn
// inline. They rank above the presentation attributes and under the SVG's own stylesheets.
const DocumentStyleAttribute = "data-inkshelf-css"

// defaultFontSize is the font size of an SVG's text when nothing gives one, in user units.
const defaultFontSize = 16

// styledProperties are the properties the styler reads, as presentation attributes too.
var styledProperties = map[string]bool{
	"fill": true, "fill-opacity": true, "stroke": true, "stroke-width": true, "stroke-opacity": true,
	"stroke-linecap": true, "stroke-linejoin": true, "stroke-miterlimit": true, "stroke-dasharray": true,
	"stroke-dashoffset": true, "opacity": true, "stop-color": true, "stop-opacity": true, "color": true,
	"display": true, "visibility": true, "font-family": true, "font-size": true, "font-weight": true,
	"font-style": true, "text-anchor": true,
}

// Styled reports whether the styler follows property: its values in DocumentStyleAttribute
// count, others are left out.
func Styled(property string) bool { return styledProperties[property] }

// inheritedStyle is what an element's children inherit, as far as the styler follows it: the
// rest oksvg inherits itself.
type inheritedStyle struct {
	color       css.Color
	fill        string // as declared, currentColor resolved
	fillOpacity float64
	hidden      bool // visibility: hidden
	families    []string
	fontSize    float64 // in user units
	bold        bool
	italic      bool
	anchor      string // text-anchor: start, middle or end
}

// rootStyle is what the root element inherits: CSS's initial values.
var rootStyle = inheritedStyle{color: css.Black, fill: "black", fillOpacity: 1, fontSize: defaultFontSize, anchor: "start"}

// cascade computes the values of every element of the tree from root down: the presentation
// attributes, then the chapter's declarations (DocumentStyleAttribute), then the cascade of
// sheets with the style attribute.
func cascade(root *node, sheets *css.Cascade) {
	var walk func(element *node, parent inheritedStyle)
	walk = func(element *node, parent inheritedStyle) {
		values := map[string]string{}
		for _, attribute := range element.attributes {
			if name := strings.ToLower(attribute.Name.Local); attribute.Name.Space == "" && styledProperties[name] {
				values[name] = strings.TrimSpace(attribute.Value)
			}
		}
		if document, ok := element.Attribute(DocumentStyleAttribute); ok {
			maps.Copy(values, css.ParseDeclarations(document))
		}
		maps.Copy(values, sheets.Compute(element, ""))
		for property, value := range values {
			if !styledProperties[property] || strings.EqualFold(value, "inherit") {
				delete(values, property) // inherited from the parent, by oksvg or below
			}
		}
		element.values = values
		element.inherited = inheritStyle(parent, values)
		for _, child := range element.children {
			if child.local != "" {
				walk(child, element.inherited)
			}
		}
	}
	walk(root, rootStyle)
}

// fontSizeKeywords are the absolute font-size keywords, in pixels.
var fontSizeKeywords = map[string]float64{
	"xx-small": 9, "x-small": 10, "small": 13, "medium": 16, "large": 18, "x-large": 24, "xx-large": 32,
}

// inheritStyle returns what an element whose declared values are values passes on, its parent
// passing on parent.
func inheritStyle(parent inheritedStyle, values map[string]string) inheritedStyle {
	style := parent
	if value, ok := values["color"]; ok {
		if color, ok := css.ParseColor(value, parent.color); ok {
			style.color = color
		}
	}
	if value, ok := values["fill"]; ok {
		style.fill = resolveCurrentColor(value, style.color)
	}
	if value, ok := values["fill-opacity"]; ok {
		if opacity, ok := parseOpacity(value); ok {
			style.fillOpacity = opacity
		}
	}
	if value, ok := values["visibility"]; ok {
		style.hidden = strings.EqualFold(value, "hidden") || strings.EqualFold(value, "collapse")
	}
	if value, ok := values["font-family"]; ok {
		style.families = css.Families(value)
	}
	if value, ok := values["font-size"]; ok {
		style.fontSize = fontSizeOf(value, parent.fontSize)
	}
	if value, ok := values["font-weight"]; ok {
		switch value = strings.ToLower(value); value {
		case "bold", "bolder":
			style.bold = true
		case "normal", "lighter":
			style.bold = false
		default:
			if weight, err := strconv.Atoi(value); err == nil {
				style.bold = weight >= 600
			}
		}
	}
	if value, ok := values["font-style"]; ok {
		value = strings.ToLower(value)
		style.italic = value == "italic" || strings.HasPrefix(value, "oblique")
	}
	if value, ok := values["text-anchor"]; ok {
		if value = strings.ToLower(value); value == "start" || value == "middle" || value == "end" {
			style.anchor = value
		}
	}
	return style
}

// fontSizeOf returns a font-size in user units, relative to the parent's size.
func fontSizeOf(value string, parentSize float64) float64 {
	if size, ok := fontSizeKeywords[strings.ToLower(value)]; ok {
		return size
	}
	switch strings.ToLower(value) {
	case "smaller":
		return parentSize / 1.2
	case "larger":
		return parentSize * 1.2
	}
	length, ok := css.ParseLength(value)
	if !ok || length.Value < 0 {
		return parentSize
	}
	if pixels, ok := length.Pixels(); ok {
		return pixels
	}
	switch length.Unit {
	case "em", "rem":
		return length.Value * parentSize
	case "ex", "ch":
		return length.Value * parentSize / 2
	case "%":
		return length.Value * parentSize / 100
	case "":
		return length.Value // a bare number is in user units
	}
	return parentSize
}

// resolveCurrentColor returns a paint value with currentColor replaced by color.
func resolveCurrentColor(value string, color css.Color) string {
	if strings.EqualFold(strings.TrimSpace(value), "currentcolor") {
		return hexOf(color.Gray)
	}
	return value
}

// parseOpacity returns an opacity from 0 to 1: a number or a percentage.
func parseOpacity(value string) (float64, bool) {
	length, ok := css.ParseLength(value)
	if !ok || length.Unit != "" && length.Unit != "%" || math.IsNaN(length.Value) {
		return 0, false
	}
	if length.Unit == "%" {
		length.Value /= 100
	}
	return min(max(length.Value, 0), 1), true
}
