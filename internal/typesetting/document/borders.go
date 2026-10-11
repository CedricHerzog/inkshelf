package document

// BorderStyle is how a side of a box's border is drawn.
type BorderStyle uint8

// The border styles. CSS's others (groove, ridge, inset, outset) are drawn as solid.
const (
	NoBorder BorderStyle = iota
	Solid
	Double
	Dotted
	Dashed
)

// Border is a side of a box's border: its style, its width (in hundredths of an em in a BorderBox,
// in pixels in layout's PlacedBorderBox) and its gray.
type Border struct {
	Style BorderStyle
	Width int
	Gray  uint8
}

// BorderBox is the border and background of a block element, drawn around the blocks of the
// document from offset Start to End. Lengths are in hundredths of an em of the reader's font
// size; LeftEdge and RightEdge are where its border's outer edges are, from the text's margins.
// The Left and Right of its blocks already make room for its border and padding, and for those
// of the boxes around it. Boxes go in Document.Boxes in the order they open: by Start, the outer
// one first.
type BorderBox struct {
	Start, End                int
	Top, Right, Bottom, Left  Border
	PaddingTop, PaddingBottom Hundredths
	MarginTop, MarginBottom   Hundredths
	LeftEdge, RightEdge       Hundredths
	Background                uint8
	Filled                    bool // the Background is drawn: not transparent
	PageBreak                 bool // the box starts on a new page
}
