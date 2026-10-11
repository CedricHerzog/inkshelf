package svgimage

import (
	"encoding/xml"
	"errors"
	"io"
	"slices"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// The work an SVG may ask of the styler, against hostile books. Past maxElements elements (and
// text nodes) the SVG is not drawn; elements deeper than maxDepth are left out; stylesheets
// past maxStyleBytes in all are ignored.
const (
	maxElements   = 100_000
	maxDepth      = 256
	maxStyleBytes = 256 << 10
)

var errTooLarge = errors.New("svgimage: too many elements")

// node is an element of an SVG, or a text node (whose name is empty). The SVG is read whole
// into a tree, so that CSS selectors can be matched against it and the result written back
// for oksvg.
type node struct {
	name       xml.Name // as written: Space is the prefix
	local      string   // the local name, lowercase
	attributes []xml.Attr
	classes    []string
	text       string // a text node's
	children   []*node
	parent     *node
	previous   *node // the previous element among the parent's children
	// values are the declared values of the properties the styler follows, from the
	// presentation attributes, the chapter's CSS, the SVG's stylesheets and its style
	// attribute. inherited is what its text and its children take from it.
	values    map[string]string
	inherited inheritedStyle
}

// A node is a css.Element, which selectors are matched against.

func (element *node) Name() string { return element.local }
func (element *node) ID() string {
	value, _ := element.Attribute("id")
	return value
}
func (element *node) Classes() []string { return element.classes }

func (element *node) Attribute(name string) (string, bool) {
	for _, attribute := range element.attributes {
		if strings.EqualFold(attribute.Name.Local, name) && (attribute.Name.Space == "" || attribute.Name.Space == "xlink") {
			return attribute.Value, true
		}
	}
	return "", false
}

func (element *node) Parent() css.Element {
	if element.parent == nil {
		return nil
	}
	return element.parent
}

func (element *node) PreviousSibling() css.Element {
	if element.previous == nil {
		return nil
	}
	return element.previous
}

// lastElement returns the last element among the node's children, nil if none.
func (element *node) lastElement() *node {
	for _, child := range slices.Backward(element.children) {
		if child.local != "" {
			return child
		}
	}
	return nil
}

// treeReader builds the tree of an SVG from its tokens.
type treeReader struct {
	root  *node
	stack []*node // the open elements
	nodes int     // the elements and text nodes read so far
	// skipped is how deep inside an element left out (past maxDepth, or after the root) the
	// decoder is.
	skipped int
}

// parseTree reads an SVG into a tree of nodes, leniently: what follows broken markup is left
// out.
func parseTree(data []byte) (*node, error) {
	decoder := newDecoder(data)
	var reader treeReader
	for {
		token, err := decoder.RawToken()
		if err != nil {
			if reader.root == nil || !errors.Is(err, io.EOF) && len(reader.stack) == 0 {
				return nil, errNotSVG
			}
			break
		}
		switch token := token.(type) {
		case xml.StartElement:
			err = reader.open(token)
		case xml.EndElement:
			reader.close(token)
		case xml.CharData:
			err = reader.addText(string(token))
		}
		if err != nil {
			return nil, err
		}
	}
	return reader.root, nil
}

// open adds the element that token starts to the tree, under the innermost open element.
func (reader *treeReader) open(token xml.StartElement) error {
	if reader.skipped > 0 || len(reader.stack) >= maxDepth || reader.root != nil && len(reader.stack) == 0 {
		reader.skipped++
		return nil
	}
	reader.nodes++
	if reader.nodes > maxElements {
		return errTooLarge
	}
	opened := &node{name: token.Name, local: strings.ToLower(token.Name.Local), attributes: slices.Clone(token.Attr)}
	if class, ok := opened.Attribute("class"); ok {
		opened.classes = strings.Fields(class)
	}
	if reader.root == nil {
		if opened.local != "svg" {
			return errNotSVG
		}
		reader.root = opened
	} else {
		parent := reader.stack[len(reader.stack)-1]
		opened.parent, opened.previous = parent, parent.lastElement()
		parent.children = append(parent.children, opened)
	}
	reader.stack = append(reader.stack, opened)
	return nil
}

// close closes the element that token ends, if it is open, and the elements left open in it.
func (reader *treeReader) close(token xml.EndElement) {
	if reader.skipped > 0 {
		reader.skipped--
		return
	}
	for index, v := range slices.Backward(reader.stack) {
		if v.name.Local == token.Name.Local {
			reader.stack = reader.stack[:index]
			return
		}
	}
}

// addText adds text to the innermost open element, joined to the text node that ends it if
// any.
func (reader *treeReader) addText(text string) error {
	if reader.skipped > 0 || len(reader.stack) == 0 {
		return nil
	}
	parent := reader.stack[len(reader.stack)-1]
	if last := len(parent.children) - 1; last >= 0 && parent.children[last].local == "" {
		parent.children[last].text += text
		return nil
	}
	reader.nodes++
	if reader.nodes > maxElements {
		return errTooLarge
	}
	parent.children = append(parent.children, &node{text: text, parent: parent})
	return nil
}
