package svgimage

import (
	"bytes"
	"encoding/xml"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// oksvg, which draws the shapes, follows only a part of SVG: no CSS stylesheets, no classes,
// few units and color forms. So the styler writes each SVG back before oksvg reads it, with
// each element's computed values in a style attribute in forms that oksvg reads, and with what
// oksvg would draw wrong left out: text (drawn apart), patterns, masks, clip paths, markers.

// hexOf returns a gray as #rrggbb.
func hexOf(gray uint8) string {
	const digits = "0123456789abcdef"
	pair := string([]byte{digits[gray>>4], digits[gray&0xf]})
	return "#" + pair + pair + pair
}

// formatNumber writes a number as oksvg reads it: no exponent.
func formatNumber(number float64) string {
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return "0"
	}
	return strconv.FormatFloat(number, 'f', -1, 64)
}

// styler writes an SVG back for oksvg: each element's computed values in its style attribute,
// and its text taken out, for Rasterize to draw.
type styler struct {
	ids map[string]*node // the elements with an id
	// patternColors are the flat colors that patterns paint with, by id, once found. resolving
	// holds the patterns being looked at, against patterns that name each other.
	patternColors map[string]paint
	resolving     map[string]bool
	gradients     []*node // in document order, for the tiles of patterns
	texts         []textElement
	output        bytes.Buffer
	// shapes counts the shapes written, in document order, out of referenced content. modeOf
	// says how each shape is written; it is nil in the first pass, which writes them all and
	// finds the pattern fills.
	shapes int
	modeOf func(index int) shapeMode
	found  []foundFill
}

// shapeMode is how a shape is written in a pass of the styler.
type shapeMode int

const (
	shapeDrawn      shapeMode = iota
	shapeLeftOut              // not written
	shapeStrokeOnly           // without its fill: its pattern fill is drawn under the layer
	shapeMask                 // filled black, with its fill's opacities, without its stroke
	shapeFlatFill             // its fill alone, its pattern's flat color
)

// drawable are the elements counted as shapes, which a layer draws or leaves out.
var drawable = map[string]bool{
	"rect": true, "circle": true, "ellipse": true, "line": true, "polyline": true, "polygon": true,
	"path": true, "use": true,
}

// notDrawn are the elements left out, with their content: what oksvg would draw where it
// should not, or choke on; text is drawn by Rasterize.
var notDrawn = map[string]bool{
	"style": true, "script": true, "title": true, "desc": true, "metadata": true, "foreignobject": true,
	"pattern": true, "mask": true, "clippath": true, "marker": true, "symbol": true, "text": true,
	"filter": true,
}

// restyled is an SVG written back for oksvg, and its text.
type restyled struct {
	markup []byte // the whole SVG, pattern fills as flat colors
	// With pattern fills, layers are the SVG cut at its pattern-filled shapes, drawn in order,
	// fills[index] after layers[index].
	layers [][]byte
	fills  []patternFill
	texts  []textElement
}

// restyle reads an SVG, computes its styles from its stylesheets, extraCSS (under them) and
// its attributes, and writes it back for oksvg.
func restyle(data []byte, extraCSS string, media css.Media) (restyled, error) {
	root, err := parseTree(data)
	if err != nil {
		return restyled{}, err
	}
	sheets := css.NewCascade()
	budget := maxStyleBytes
	addSheet := func(text string) {
		if len(text) > budget {
			return
		}
		budget -= len(text)
		sheets.Add(css.Parse(text, media))
	}
	addSheet(extraCSS)
	work := styler{ids: map[string]*node{}, patternColors: map[string]paint{}, resolving: map[string]bool{}}
	var collect func(element *node)
	collect = func(element *node) {
		if id := element.ID(); id != "" {
			if _, taken := work.ids[id]; !taken {
				work.ids[id] = element
			}
		}
		if element.local == "lineargradient" || element.local == "radialgradient" {
			work.gradients = append(work.gradients, element)
		}
		if element.local == "style" {
			if kind, ok := element.Attribute("type"); !ok || strings.EqualFold(strings.TrimSpace(kind), "text/css") {
				var text strings.Builder
				for _, child := range element.children {
					text.WriteString(child.text)
				}
				addSheet(text.String())
			}
		}
		for _, child := range element.children {
			if child.local != "" {
				collect(child)
			}
		}
	}
	collect(root)
	cascade(root, sheets)
	work.write(root, identity, false)
	result := restyled{markup: bytes.Clone(work.output.Bytes()), texts: work.texts}
	if len(work.found) > 0 {
		result.layers, result.fills = work.layers(root)
	}
	return result, nil
}

// write writes element and its content, transform being its parent's user space's to the
// root's; referenced is set inside a <defs>, whose text is not drawn.
func (work *styler) write(element *node, transform matrix, referenced bool) {
	if element.local == "" {
		_ = xml.EscapeText(&work.output, []byte(element.text))
		return
	}
	if strings.EqualFold(element.values["display"], "none") {
		return
	}
	own, hasTransform := identity, false
	if value, ok := element.Attribute("transform"); ok {
		own, hasTransform = parseTransform(value)
		transform = transform.multiply(own)
	}
	if element.local == "text" && !referenced && work.modeOf == nil {
		work.addText(element, transform)
	}
	referenced = referenced || element.local == "defs"
	if notDrawn[element.local] {
		return
	}
	mode := shapeDrawn
	if drawable[element.local] && !referenced {
		index := work.shapes
		work.shapes++
		if work.modeOf != nil {
			mode = work.modeOf(index)
		} else {
			work.findFill(element, index, transform)
		}
		if mode == shapeLeftOut {
			return
		}
	}
	work.output.WriteByte('<')
	writeName(&work.output, element.name)
	for _, attribute := range element.attributes {
		name := strings.ToLower(attribute.Name.Local)
		switch {
		case attribute.Name.Space != "" && attribute.Name.Space != "xlink" && attribute.Name.Space != "xmlns" && attribute.Name.Space != "xml",
			attribute.Name.Space == "" && (styledProperties[name] || name == "style" || name == "class" || name == "transform" || name == DocumentStyleAttribute):
			continue
		}
		writeAttribute(&work.output, attribute.Name, attribute.Value)
	}
	if hasTransform {
		writeAttribute(&work.output, xml.Name{Local: "transform"}, own.String())
	}
	if element.local == "stop" {
		work.writeStop(element)
	} else if style := work.style(element, mode); style != "" {
		writeAttribute(&work.output, xml.Name{Local: "style"}, style)
	}
	work.output.WriteByte('>')
	for _, child := range element.children {
		work.write(child, transform, referenced)
	}
	work.output.WriteString("</")
	writeName(&work.output, element.name)
	work.output.WriteByte('>')
}

// writeName writes an element's or attribute's name, with its prefix.
func writeName(output *bytes.Buffer, name xml.Name) {
	if name.Space != "" {
		output.WriteString(name.Space)
		output.WriteByte(':')
	}
	output.WriteString(name.Local)
}

// writeAttribute writes an attribute, a space before it.
func writeAttribute(output *bytes.Buffer, name xml.Name, value string) {
	output.WriteByte(' ')
	writeName(output, name)
	output.WriteString(`="`)
	_ = xml.EscapeText(output, []byte(value))
	output.WriteByte('"')
}

// lengthProperties are the properties whose value is a length, written as a number of user
// units.
var lengthProperties = []string{"stroke-width", "stroke-miterlimit", "stroke-dashoffset"}

// keywordProperties are the properties whose value is a keyword, and the keywords oksvg reads.
var keywordProperties = []struct {
	property string
	keywords []string
}{
	{"stroke-linecap", []string{"butt", "round", "square"}},
	{"stroke-linejoin", []string{"miter", "miter-clip", "round", "bevel", "arcs"}},
}

// style returns the style attribute of element for oksvg, written as mode says: its values
// that oksvg reads, in forms it reads. A value that it could not read is left out.
func (work *styler) style(element *node, mode shapeMode) string {
	var declarations declarationList
	work.addPaint(&declarations, element, mode)
	addStrokeShape(&declarations, element)
	switch mode { // last, over what the element declares and inherits
	case shapeStrokeOnly:
		declarations.add("fill", "none")
	case shapeMask:
		declarations.add("fill", "#000000")
		declarations.add("stroke", "none")
	case shapeFlatFill:
		declarations.add("stroke", "none")
	}
	return strings.Join(declarations, ";")
}

// declarationList is the declarations of a style attribute, "property:value".
type declarationList []string

// add adds a declaration.
func (list *declarationList) add(property, value string) {
	*list = append(*list, property+":"+value)
}

// addPaint adds element's fill and stroke and their opacities. The alpha of a color becomes
// an opacity, multiplied with the one declared, as oksvg reads colors without alpha. A mask
// leaves the fill to its mode.
func (work *styler) addPaint(declarations *declarationList, element *node, mode shapeMode) {
	values := element.values
	opacities := map[string]float64{}
	for _, property := range []string{"opacity", "fill-opacity", "stroke-opacity"} {
		if opacity, ok := parseOpacity(values[property]); ok {
			opacities[property] = opacity
		}
	}
	for _, property := range []string{"fill", "stroke"} {
		value, ok := values[property]
		if !ok || mode == shapeMask && property == "fill" {
			continue
		}
		painted, ok := work.paintOf(value, element.inherited.color)
		if !ok {
			continue
		}
		declarations.add(property, painted.value)
		if painted.opacity < 1 {
			opacity, declared := opacities[property+"-opacity"]
			if !declared {
				opacity = 1
			}
			opacities[property+"-opacity"] = opacity * painted.opacity
		}
	}
	if element.inherited.hidden {
		declarations.add("fill", "none")
		declarations.add("stroke", "none")
	}
	for _, property := range []string{"opacity", "fill-opacity", "stroke-opacity"} {
		if opacity, ok := opacities[property]; ok {
			declarations.add(property, formatNumber(opacity))
		}
	}
}

// addStrokeShape adds the properties that shape element's stroke: its width, caps, joins and
// dashes, lengths as numbers of user units.
func addStrokeShape(declarations *declarationList, element *node) {
	values := element.values
	for _, property := range lengthProperties {
		if length, ok := lengthOf(values[property], element.inherited.fontSize); ok && length >= 0 {
			declarations.add(property, formatNumber(length))
		}
	}
	for _, keyword := range keywordProperties {
		if value := strings.ToLower(values[keyword.property]); slices.Contains(keyword.keywords, value) {
			declarations.add(keyword.property, value)
		}
	}
	if value, ok := values["stroke-dasharray"]; ok {
		if dashes, ok := dashArrayOf(value, element.inherited.fontSize); ok {
			declarations.add("stroke-dasharray", dashes)
		}
	}
}

// writeStop writes the stop-color and stop-opacity of a gradient's <stop> as attributes, as
// oksvg reads them there only.
func (work *styler) writeStop(element *node) {
	color, opacity := css.Black, 1.0
	if value, ok := element.values["stop-color"]; ok {
		if parsed, ok := css.ParseColor(value, element.inherited.color); ok {
			color = parsed
		}
	}
	if value, ok := element.values["stop-opacity"]; ok {
		if parsed, ok := parseOpacity(value); ok {
			opacity = parsed
		}
	}
	opacity *= float64(color.Alpha) / 255
	writeAttribute(&work.output, xml.Name{Local: "stop-color"}, hexOf(color.Gray))
	if opacity < 1 {
		writeAttribute(&work.output, xml.Name{Local: "stop-opacity"}, formatNumber(opacity))
	}
}

// lengthOf returns a length in user units: a number, or a length in absolute units or ems
// of fontSize. Percentages are left out.
func lengthOf(value string, fontSize float64) (float64, bool) {
	length, ok := css.ParseLength(value)
	if !ok || math.IsNaN(length.Value) || math.IsInf(length.Value, 0) {
		return 0, false
	}
	if pixels, ok := length.Pixels(); ok {
		return pixels, true
	}
	switch length.Unit {
	case "":
		return length.Value, true
	case "em", "rem":
		return length.Value * fontSize, true
	}
	return 0, false
}

// dashArrayOf returns a stroke-dasharray as oksvg reads it: none, or numbers.
func dashArrayOf(value string, fontSize float64) (string, bool) {
	if strings.EqualFold(value, "none") {
		return "none", true
	}
	fields := strings.FieldsFunc(value, func(character rune) bool { return character == ',' || character == ' ' })
	if len(fields) == 0 {
		return "", false
	}
	numbers := make([]string, len(fields))
	for index, field := range fields {
		length, ok := lengthOf(field, fontSize)
		if !ok || length < 0 {
			return "", false
		}
		numbers[index] = formatNumber(length)
	}
	return strings.Join(numbers, ","), true
}
