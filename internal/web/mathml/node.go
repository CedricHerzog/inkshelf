package mathml

import (
	"encoding/xml"
	"errors"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

// Limits on what a formula may hold: past them, Read and Parse fail, and the book shows the
// formula's text alternative.
const (
	maxNodes = 4000
	maxDepth = 48
	// maxText is the length of a formula's text, in bytes.
	maxText = 20000
)

// ErrLimit is the error of a formula past the limits of what is laid out.
var ErrLimit = errors.New("mathml: formula too large")

// Prefix starts the image path of a formula: its markup follows (Path).
const Prefix = "mathml:"

// Node is an element of a formula: its lowercase name, the attributes layout reads, and the
// text of a token element (mi, mn, mo, mtext, ms), its whitespace collapsed.
type Node struct {
	Name       string
	Attributes map[string]string
	Text       string
	Children   []*Node
}

// usedAttributes are the attributes Layout reads: Read keeps those only.
var usedAttributes = []string{
	"display", "mode", "displaystyle", "scriptlevel", "mathvariant", "mathsize", "fontweight", "fontstyle",
	"linethickness", "stretchy", "largeop", "movablelimits", "accent", "accentunder", "fence", "form",
	"lspace", "rspace", "symmetric", "width", "height", "depth", "voffset", "open", "close", "separators",
	"notation", "columnalign", "rowalign", "columnlines", "rowlines", "frame", "columnspacing", "rowspacing",
	"lquote", "rquote", "selection",
}

// tokens are the elements whose content is text.
var tokens = []string{"mi", "mn", "mo", "mtext", "ms"}

// Read reads the formula that decoder starts with, from the start tag of its <math> to its end
// tag, and returns its root. It fails on markup that isn't a <math> or past the limits.
func Read(decoder *xml.Decoder) (*Node, error) {
	nodes, textLength := 0, 0
	var root *Node
	var stack []*Node
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		if err != nil {
			return nil, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			name := strings.ToLower(token.Name.Local)
			if !isName(name) {
				name = "mrow" // what a lenient decoder lets through, which Markup couldn't write
			}
			if root == nil && name != "math" {
				return nil, errors.New("mathml: no <math>")
			}
			nodes++
			if nodes > maxNodes || len(stack) >= maxDepth {
				return nil, ErrLimit
			}
			node := &Node{Name: name, Attributes: attributesOf(token.Attr)}
			if root == nil {
				root = node
			} else {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, node)
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, errors.New("mathml: unbalanced markup")
			}
			closed := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if slices.Contains(tokens, closed.Name) {
				closed.Text = collapse(closed.Text)
			}
			if len(stack) == 0 {
				return root, nil
			}
		case xml.CharData:
			if len(stack) == 0 {
				continue
			}
			if textLength += len(token); textLength > maxText {
				return nil, ErrLimit
			}
			// A token's text, and that of the elements in it (<mglyph> aside, which has none).
			for _, s := range slices.Backward(stack) {
				if slices.Contains(tokens, s.Name) {
					s.Text += replaceEntities(string(token))
					break
				}
			}
		}
	}
}

// isName reports whether name is an element's name that Markup writes as it is: a letter, then
// letters, digits and hyphens.
func isName(name string) bool {
	for index, character := range name {
		if !('a' <= character && character <= 'z' || index > 0 && ('0' <= character && character <= '9' || character == '-')) {
			return false
		}
	}
	return name != ""
}

// Parse returns the formula of markup, which Markup wrote or a book holds.
func Parse(markup string) (*Node, error) {
	return Read(xml.NewDecoder(strings.NewReader(markup)))
}

// attributesOf returns the attributes of usedAttributes, by lowercase local name; nil if none.
func attributesOf(attributes []xml.Attr) map[string]string {
	var kept map[string]string
	for _, attribute := range attributes {
		name := strings.ToLower(attribute.Name.Local)
		if !slices.Contains(usedAttributes, name) {
			continue
		}
		if kept == nil {
			kept = map[string]string{}
		}
		kept[name] = strings.TrimSpace(attribute.Value)
	}
	return kept
}

// collapse returns text with its whitespace collapsed to single spaces and trimmed, as MathML
// does to a token's text.
func collapse(text string) string { return strings.Join(strings.Fields(text), " ") }

// entities are MathML's named characters that HTML lacks, which a lenient decoder leaves as
// they are written: the invisible operators mostly.
var entities = strings.NewReplacer(
	"&ApplyFunction;", "\u2061", "&af;", "\u2061", "&InvisibleTimes;", "\u2062", "&it;", "\u2062",
	"&InvisibleComma;", "\u2063", "&ic;", "\u2063", "&PlusMinus;", "±", "&MinusPlus;", "∓",
	"&Element;", "∈", "&Sum;", "∑", "&Integral;", "∫", "&PartialD;", "∂", "&DifferentialD;", "ⅆ",
	"&LeftAngleBracket;", "⟨", "&RightAngleBracket;", "⟩", "&RightArrow;", "→", "&LeftArrow;", "←",
	"&Rightarrow;", "⇒", "&Implies;", "⇒", "&NotEqual;", "≠", "&LessEqual;", "≤", "&GreaterEqual;", "≥",
	"&Infinity;", "∞", "&CenterDot;", "·", "&Cross;", "⨯", "&times;", "×", "&OverBar;", "‾", "&UnderBar;", "_",
)

// replaceEntities replaces MathML's named characters in text.
func replaceEntities(text string) string {
	if !strings.Contains(text, "&") {
		return text
	}
	return entities.Replace(text)
}

// Markup returns the formula as XML, what Parse reads: the attributes Layout reads and the
// tokens' text.
func (node *Node) Markup() string {
	var markup strings.Builder
	node.write(&markup)
	return markup.String()
}

// write writes the node's markup into markup.
func (node *Node) write(markup *strings.Builder) {
	markup.WriteString("<" + node.Name)
	names := make([]string, 0, len(node.Attributes))
	for name := range node.Attributes {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		markup.WriteString(" " + name + `="`)
		escape(markup, node.Attributes[name])
		markup.WriteString(`"`)
	}
	markup.WriteString(">")
	escape(markup, node.Text)
	for _, child := range node.Children {
		child.write(markup)
	}
	markup.WriteString("</" + node.Name + ">")
}

// escape writes text into markup, escaped for XML.
func escape(markup *strings.Builder, text string) {
	var escaped strings.Builder
	if err := xml.EscapeText(&escaped, []byte(text)); err != nil {
		return // a strings.Builder never fails
	}
	markup.WriteString(escaped.String())
}

// Path returns the image path of a formula in a document: Prefix and its markup.
func Path(root *Node) string { return Prefix + root.Markup() }

// Is reports whether an image path is a formula's.
func Is(path string) bool { return strings.HasPrefix(path, Prefix) }

// FromPath returns the formula of an image path that Path returned.
func FromPath(path string) (*Node, error) {
	if !Is(path) {
		return nil, errors.New("mathml: not a formula")
	}
	return Parse(path[len(Prefix):])
}

// Display reports whether the formula is a block of its own, centred, rather than in a line
// of text.
func (node *Node) Display() bool {
	return node.Attributes["display"] == "block" || node.Attributes["mode"] == "display"
}

// runeCount returns the number of characters of text.
func runeCount(text string) int { return utf8.RuneCountInString(text) }

// firstRune returns the first character of text.
func firstRune(text string) rune {
	character, _ := utf8.DecodeRuneInString(text)
	return character
}
