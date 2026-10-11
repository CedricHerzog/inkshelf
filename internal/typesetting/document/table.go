package document

// Table is a grid of cells; its blocks are the blocks of the document whose Cell is one of its
// cells. Its columns are as wide as their cells' text needs, fitted in the text's width (the
// font shrinks for a table too wide), and a row is as high as its tallest cell. A row too high
// for a page is cut across pages; the others are kept whole, under the header rows repeated.
type Table struct {
	Rows, Columns int
	// HeaderRows are the first rows, repeated at the top of each page the table continues on.
	HeaderRows int
	// Cells are the table's cells, row after row, empty ones too.
	Cells []*TableCell
	// Width is the table's width, in percent of the text's, 0 for its content's.
	Width Percent
	// ColumnWidths are the widths asked for the columns, in percent of the table's, 0 for none.
	ColumnWidths []Percent
	// Left and Right are its margins, MarginTop and MarginBottom the space above and below it,
	// in hundredths of an em; PageBreak starts it on a new page.
	Left, Right, MarginTop, MarginBottom Hundredths
	PageBreak                            bool
	// Frame are the table's own borders, around its cells.
	Frame Edges
	// Parent is the cell the table is in, nil for one in the text.
	Parent *TableCell
}

// TableCell is a cell of a table, at a row and a column from 0, spanning rows and columns (1
// at least). Its paddings are in hundredths of an em.
type TableCell struct {
	Table                                                *Table
	Row, Column, RowSpan, ColumnSpan                     int
	PaddingTop, PaddingRight, PaddingBottom, PaddingLeft Hundredths
	VerticalAlign                                        VerticalAlign
	Borders                                              Edges
}

// VerticalAlign is where the content of a cell sits in its row.
type VerticalAlign uint8

// The vertical alignments.
const (
	VerticalTop VerticalAlign = iota
	VerticalMiddle
	VerticalBottom
)

// Edges are sides of a box: those that have a border.
type Edges uint8

// The edges.
const (
	EdgeTop Edges = 1 << iota
	EdgeRight
	EdgeBottom
	EdgeLeft
	AllEdges = EdgeTop | EdgeRight | EdgeBottom | EdgeLeft
)

// The limits of a table, so that a broken or hostile book cannot make layout take forever: more
// columns go into the last one, and a longer span is cut.
const (
	MaxTableColumns = 32
	MaxTableRowSpan = 64
)
