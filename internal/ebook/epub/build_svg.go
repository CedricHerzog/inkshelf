package epub

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"maps"
	"slices"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/container"
	"github.com/CedricHerzog/inkshelf/internal/web/svgimage"
)

// inlineSVG shows the <svg> just opened, whose markup data starts with, as an image if the
// SVG rasterizer can draw it all: decoder then skips its content, which is read as the
// chapter's otherwise.
func (parser *builder) inlineSVG(decoder *xml.Decoder, data []byte) {
	svg := parser.top()
	if svg.hidden {
		return
	}
	markup, ok := parser.drawnSVG(svg, data)
	if !ok || decoder.Skip() != nil {
		return
	}
	parser.addImage("data:image/svg+xml;base64,"+base64.StdEncoding.EncodeToString(markup), svg)
	parser.end("svg")
}

// drawnSVG returns the markup of the <svg> just opened, from data, which starts with its start
// tag, to its end tag: an image to draw, each element given what the chapter's stylesheets
// declare for it in svgimage's DocumentStyleAttribute. It returns false for one that holds a
// raster image or HTML, which the chapter shows as its own content (its <image>, its
// <foreignObject>'s text): an <image> in a data: SVG has nothing to read its file from.
func (parser *builder) drawnSVG(svg *element, data []byte) ([]byte, bool) {
	decoder := container.NewDecoder(data)
	var markup bytes.Buffer
	copied := 0 // the bytes of data copied to markup so far
	var open []*element
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, false
		}
		switch token := token.(type) {
		case xml.StartElement:
			name := strings.ToLower(token.Name.Local)
			if len(open) == 0 && name != "svg" || name == "image" || name == "foreignobject" {
				return nil, false
			}
			opened := svg
			if len(open) > 0 {
				parent := open[len(open)-1]
				opened = &element{name: name, attributes: attributesOf(token.Attr), parent: parent, previous: parent.lastChild}
				opened.classes = strings.Fields(opened.attributes["class"])
			}
			open = append(open, opened)
			// Before the end of the start tag, "/>" or ">".
			end := int(decoder.InputOffset()) - 1
			if end > 0 && data[end-1] == '/' {
				end--
			}
			if declarations := svgDeclarations(parser.cascade.Compute(opened, "")); declarations != "" && end > copied {
				markup.Write(data[copied:end])
				markup.WriteString(" " + svgimage.DocumentStyleAttribute + `="`)
				_ = xml.EscapeText(&markup, []byte(declarations))
				markup.WriteByte('"')
				copied = end
			}
		case xml.EndElement:
			closed := open[len(open)-1]
			if open = open[:len(open)-1]; len(open) == 0 {
				markup.Write(data[copied:decoder.InputOffset()])
				return markup.Bytes(), true
			}
			open[len(open)-1].lastChild = closed
		}
	}
}

// svgDeclarations returns the values of the properties that the SVG rasterizer follows, as a
// style attribute, in the order of their names.
func svgDeclarations(values map[string]string) string {
	var declarations []string
	for _, property := range slices.Sorted(maps.Keys(values)) {
		if svgimage.Styled(property) {
			declarations = append(declarations, property+": "+values[property])
		}
	}
	return strings.Join(declarations, "; ")
}
