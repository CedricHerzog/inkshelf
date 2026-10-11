package layout

import (
	"image"
	"log"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/hyphen"
	"github.com/CedricHerzog/inkshelf/internal/web/mathml"
)

// Settings are how a document is laid out: the page, its margins and the reader's font size.
type Settings struct {
	Width, Height document.Pixels // the page
	// Margins are the space left around the text: Min is the top-left margins, the page size
	// minus Max the bottom-right ones.
	Margins image.Rectangle
	// FontSize is the size of the body text, in pixels.
	FontSize document.Pixels
	// LineSpacing is the height of a line, in percent of its font size.
	LineSpacing document.Percent
	// Justify justifies the paragraphs whose book does not say how to align them.
	Justify bool
	// HideFootnotes leaves out the footnotes that a link of the same document leads to, as the
	// reader shows them in a popup when their call is tapped. Notes linked from other chapters
	// (endnotes) stay in the text, where their links lead.
	HideFootnotes bool
}

// content returns the rectangle the text goes in.
func (settings Settings) content() image.Rectangle {
	return image.Rect(settings.Margins.Min.X, settings.Margins.Min.Y,
		settings.Width-settings.Margins.Max.X, settings.Height-settings.Margins.Max.Y)
}

// em converts length, in hundredths of an em, to pixels. Books size their margins and indents in
// ems so that they grow with the text; the em here is the reader's font size.
func (settings Settings) em(length int) int { return length * settings.FontSize / 100 }

// maxImageScale is how many times an image block may be enlarged at most: pictures made for a
// small screen stay sharp enough at three times their size.
const maxImageScale = 3

// ruleHeight is the thickness of a rule, in pixels, at a font size of 44 (the reader's default).
const ruleHeight = 2

// borderPadding is the space between a block's border and its content, in hundredths of an em.
const borderPadding = 25

// ImageSizer returns the size of an image of the document, in pixels, without decoding it.
type ImageSizer func(path string) (image.Point, error)

// paginator places blocks on pages, from top to bottom, starting a new page when the next thing
// does not fit. It also lays out what is laid out apart (a table cell, a float), with settings of
// its own, then copies the result onto the page.
type paginator struct {
	settings   Settings
	faces      *Faces
	sizer      ImageSizer
	hyphenator *hyphen.Hyphenator // nil if the document's language has no patterns
	doc        *document.Document
	content    image.Rectangle
	// pageHeight is the height that images are fitted in: the content's, but in a table cell,
	// which is laid out on one endless page, the height of the document's pages.
	pageHeight int
	pages      []Page
	y          int // where the next thing goes on the last page
	// marginAfter is the bottom margin of the last block placed. It collapses with the next
	// block's top margin, as in CSS: the larger of the two separates them, not their sum.
	marginAfter int
	// openBorderBoxes are the border boxes around the current block, the outer first, and
	// nextBorderBox is the index in doc.Boxes of the next one to open. marginKept keeps the next
	// block's top margin even at the top of a page, inside a box's border; marginTaken drops it,
	// as it collapsed with its box's.
	openBorderBoxes         []borderBoxPart
	nextBorderBox           int
	marginKept, marginTaken bool
	floats                  floatLayout
	// cell is the table cell being laid out, nil for the document. units, if not nil, receives
	// what is placed, line by line, so that a row too high for a page can be cut between lines.
	cell  *document.TableCell
	units *[]contentUnit
}

// Paginate lays doc out into pages. The first page starts at offset 0, and an empty document has
// one empty page. faces learn the fonts doc embeds; sizer gives the size of its images.
func Paginate(doc *document.Document, settings Settings, faces *Faces, sizer ImageSizer) ([]Page, error) {
	faces.embed(doc.Fonts)
	layout := &paginator{settings: settings, faces: faces, sizer: sizer, content: settings.content(), doc: doc}
	layout.pageHeight = layout.content.Dy()
	for _, block := range doc.Blocks {
		if block.Hyphenate {
			layout.hyphenator = hyphen.For(doc.Language)
			break
		}
	}
	layout.newPage(0)
	blocks := doc.Blocks
	if settings.HideFootnotes {
		blocks = withoutLinkedNotes(blocks)
	}
	if err := layout.placeBlocks(blocks); err != nil {
		return nil, err
	}
	if err := layout.finishFloats(); err != nil {
		return nil, err
	}
	layout.closeAllBorderBoxes()
	for index := range layout.pages {
		page := &layout.pages[index]
		if index+1 < len(layout.pages) {
			page.End = layout.pages[index+1].Start
		} else {
			page.End = doc.Chars
		}
		// A table row cut across pages shows text past the next page's start in its later cells.
		for _, line := range page.Lines {
			page.End = max(page.End, line.End)
		}
	}
	return layout.pages, nil
}

// placeBlocks places blocks on the pages; the consecutive blocks of a table are placed together,
// as its grid.
func (layout *paginator) placeBlocks(blocks []document.Block) error {
	for index := 0; index < len(blocks); {
		if layout.cell == nil && len(layout.doc.Boxes) > 0 {
			layout.enterBorderBoxes(blocks[index])
		}
		if table := layout.tableOf(blocks[index]); table != nil {
			// A table clears the floats, as an image does: a grid squeezed beside a float would
			// rarely fit.
			first := blocks[index]
			first.Clear = document.ClearBoth
			if err := layout.placeFloats(&first); err != nil {
				return err
			}
			end := index + 1
			for end < len(blocks) && layout.tableOf(blocks[end]) == table {
				end++
			}
			if err := layout.placeTable(table, blocks[index:end]); err != nil {
				return err
			}
			index = end
			continue
		}
		var next *document.Block
		if index+1 < len(blocks) {
			next = &blocks[index+1]
		}
		if err := layout.placeBlock(blocks[index], next); err != nil {
			return err
		}
		index++
	}
	return nil
}

// withoutLinkedNotes returns blocks without those of the footnotes that a link among them leads
// to. The other blocks keep their offsets, so a reading position means the same text with or
// without the notes.
func withoutLinkedNotes(blocks []document.Block) []document.Block {
	linked := map[string]bool{}
	for _, block := range blocks {
		for _, run := range block.Runs {
			if _, fragment, ok := strings.Cut(run.Link, "#"); ok {
				linked[fragment] = true
			}
		}
	}
	if len(linked) == 0 {
		return blocks
	}
	var kept []document.Block
	for _, block := range blocks {
		if block.Footnote == "" || !linked[block.Footnote] {
			kept = append(kept, block)
		}
	}
	return kept
}

// newPage starts a page whose first offset is start. The border boxes open on the last page are
// closed there and opened again on the new one.
func (layout *paginator) newPage(start int) {
	if len(layout.openBorderBoxes) > 0 {
		layout.breakBorderBoxes()
	}
	layout.pages = append(layout.pages, Page{Start: start})
	layout.y = layout.content.Min.Y
	layout.marginAfter = 0
	if len(layout.openBorderBoxes) > 0 {
		layout.restartBorderBoxes()
	}
}

// pageIsEmpty reports whether nothing is on the last page yet. A block that does not fit on an
// empty page goes there anyway: the next page would be no better.
func (layout *paginator) pageIsEmpty() bool {
	page := layout.pages[len(layout.pages)-1]
	return len(page.Lines) == 0 && len(page.Images) == 0 && len(page.Rules) == 0
}

// startBlock moves to where block goes: to a new page if it asks for one, then under its top
// margin, collapsed with the last block's bottom one. A block at the top of a page has no top
// margin, as the page's own margin is enough.
func (layout *paginator) startBlock(block document.Block) {
	if block.PageBreak && !layout.pageIsEmpty() {
		layout.newPage(block.Start)
	}
	if !layout.marginTaken && (!layout.pageIsEmpty() || layout.marginKept) {
		layout.y += max(layout.marginAfter, layout.settings.em(block.MarginTop))
	}
	layout.marginKept, layout.marginTaken = false, false
}

// roomLeft returns the height left on the last page.
func (layout *paginator) roomLeft() int { return layout.content.Max.Y - layout.y }

// placeBlock places block on the pages; next, if not nil, is the block after it, which a heading
// keeps on its page.
func (layout *paginator) placeBlock(block document.Block, next *document.Block) error {
	if layout.collectFloat(block) {
		return nil
	}
	layout.startBlock(block)
	if err := layout.placeFloats(&block); err != nil {
		return err
	}
	switch block.Kind {
	case document.Image:
		layout.placeImage(block)
	case document.Rule:
		layout.placeRule(block)
	default:
		if err := layout.placeText(block, next); err != nil {
			return err
		}
	}
	if block.BorderBottom && layout.borderHeight(true) <= layout.roomLeft() {
		layout.placeBorder(block, true)
	}
	layout.marginAfter = layout.settings.em(block.MarginBottom)
	return nil
}

// textArea returns the left and the width of block's text, inside its margins.
func (layout *paginator) textArea(block document.Block) (left, width int) {
	left = layout.content.Min.X + layout.settings.em(block.Left)
	return left, layout.content.Max.X - layout.settings.em(block.Right) - left
}

// placeImage places an image block, centred, on the current page if it fits, else on the next.
// It is ImageWidth percent of the text's width if the book says so, else its own size enlarged
// maxImageScale times at most; then it is fitted in the text's width and the page.
func (layout *paginator) placeImage(block document.Block) {
	if mathml.Is(block.Image) {
		layout.placeFormula(block)
		return
	}
	size, err := layout.sizer(block.Image)
	if err != nil || size.X <= 0 || size.Y <= 0 {
		log.Printf("layout: image %s: %v", block.Image, err)
		return
	}
	left, width := layout.textArea(block)
	scaled := image.Pt(size.X*maxImageScale, size.Y*maxImageScale)
	if block.ImageWidth > 0 {
		wanted := max(width*block.ImageWidth/100, 1)
		scaled = image.Pt(wanted, size.Y*wanted/size.X)
	}
	scaled = fitIn(scaled, image.Pt(width, layout.pageHeight), size)
	if layout.borderHeight(block.BorderTop)+scaled.Y > layout.roomLeft() && !layout.pageIsEmpty() {
		layout.newPage(block.Start)
	}
	layout.placeBorder(block, false)
	left += (width - scaled.X) / 2
	page := &layout.pages[len(layout.pages)-1]
	page.Images = append(page.Images, PlacedImage{Path: block.Image, Rect: image.Rect(left, layout.y, left+scaled.X, layout.y+scaled.Y)})
	layout.y += scaled.Y
	layout.recordUnit(layout.y-scaled.Y, block.Start)
}

// placeFormula places a displayed formula, centred, on the current page if it fits, else on the
// next. It is laid out at the text's size, smaller if it does not fit in the text's width and
// the page.
func (layout *paginator) placeFormula(block document.Block) {
	left, width := layout.textArea(block)
	formula, size, err := layout.faces.formula(block.Image, layout.settings.FontSize, image.Pt(width, layout.pageHeight))
	if err != nil {
		log.Printf("layout: formula: %v", err)
		return
	}
	height := formula.Ascent + formula.Descent
	if layout.borderHeight(block.BorderTop)+height > layout.roomLeft() && !layout.pageIsEmpty() {
		layout.newPage(block.Start)
	}
	layout.placeBorder(block, false)
	left += (width - formula.Width) / 2
	page := &layout.pages[len(layout.pages)-1]
	page.Images = append(page.Images, PlacedImage{Path: block.Image, Rect: image.Rect(left, layout.y, left+formula.Width, layout.y+height), MathSize: size})
	layout.y += height
	layout.recordUnit(layout.y-height, block.Start)
}

// fitIn returns wanted, shrunk if needed to fit in limit, keeping the aspect ratio of size.
func fitIn(wanted, limit, size image.Point) image.Point {
	if wanted.X > limit.X {
		wanted = image.Pt(limit.X, size.Y*limit.X/size.X)
	}
	if wanted.Y > limit.Y {
		wanted = image.Pt(size.X*limit.Y/size.Y, limit.Y)
	}
	return wanted
}

// ruleThickness returns the thickness of a rule, in pixels: it grows with the font size.
func (layout *paginator) ruleThickness() int { return max(ruleHeight*layout.settings.FontSize/44, 1) }

// placeRule places a rule across the middle half of the text.
func (layout *paginator) placeRule(block document.Block) {
	thickness := layout.ruleThickness()
	if layout.borderHeight(block.BorderTop)+thickness > layout.roomLeft() && !layout.pageIsEmpty() {
		layout.newPage(block.Start)
	}
	layout.placeBorder(block, false)
	quarter := layout.content.Dx() / 4
	page := &layout.pages[len(layout.pages)-1]
	page.Rules = append(page.Rules, image.Rect(layout.content.Min.X+quarter, layout.y, layout.content.Max.X-quarter, layout.y+thickness))
	layout.y += thickness
	layout.recordUnit(layout.y-thickness, block.Start)
}

// borderHeight returns the height a block's top or bottom border takes with its padding, 0 if
// border is false.
func (layout *paginator) borderHeight(border bool) int {
	if !border {
		return 0
	}
	return layout.ruleThickness() + layout.settings.em(borderPadding)
}

// placeBorder places block's top border, or its bottom one, if it has it: a rule across its
// text's width, padded from the text.
func (layout *paginator) placeBorder(block document.Block, bottom bool) {
	if !bottom && !block.BorderTop || bottom && !block.BorderBottom {
		return
	}
	thickness, padding := layout.ruleThickness(), layout.settings.em(borderPadding)
	if bottom {
		layout.y += padding
	}
	left, width := layout.textArea(block)
	page := &layout.pages[len(layout.pages)-1]
	page.Rules = append(page.Rules, image.Rect(left, layout.y, left+width, layout.y+thickness))
	layout.y += thickness
	layout.recordUnit(layout.y-thickness, block.Start)
	if !bottom {
		layout.y += padding
	}
}

// placeText breaks a paragraph or a heading into lines and places them. A paragraph's first two
// lines stay together, so that no page ends with a paragraph's lone first line (an orphan); a
// heading stays with the first two lines of the block after it.
func (layout *paginator) placeText(block document.Block, next *document.Block) error {
	if layout.besideFloats() {
		return layout.placeBeside(block, next)
	}
	size := layout.blockSize(block)
	lines, err := layout.layOutLines(block, size)
	if err != nil || len(lines) == 0 {
		return err
	}
	needed := layout.borderHeight(block.BorderTop)
	for index, line := range lines {
		if index < 2 || block.KeepWithNext {
			needed += line.height
		}
	}
	if block.KeepWithNext && next != nil && next.Kind != document.Image {
		needed += max(layout.settings.em(next.MarginTop), layout.settings.em(block.MarginBottom)) +
			2*layout.lineHeight(*next, layout.blockSize(*next))
	}
	if needed > layout.roomLeft() && !layout.pageIsEmpty() && needed <= layout.pageHeight {
		layout.newPage(lines[0].Start)
	}
	layout.placeBorder(block, false)
	for _, line := range lines {
		if line.height > layout.roomLeft() && !layout.pageIsEmpty() {
			layout.newPage(line.Start)
		}
		layout.addLine(line)
	}
	return nil
}

// addLine adds line to the last page at the current height, moving down what it placed.
func (layout *paginator) addLine(line laidOutLine) {
	page := &layout.pages[len(layout.pages)-1]
	down := image.Pt(0, layout.y)
	for index := range line.Words {
		line.Words[index].Top += layout.y
	}
	page.Lines = append(page.Lines, line.Line)
	for _, placed := range line.images {
		placed.Rect = placed.Rect.Add(down)
		page.Images = append(page.Images, placed)
	}
	for _, stroke := range line.strokes {
		page.Strokes = append(page.Strokes, Stroke{stroke.Add(down), stroke.Gray})
	}
	for _, link := range line.links {
		page.Links = append(page.Links, PlacedLink{Rect: link.Rect.Add(down), Target: link.Target})
	}
	layout.y += line.height
	layout.recordUnit(layout.y-line.height, line.Start)
}

// blockSize returns the font size of block's text.
func (layout *paginator) blockSize(block document.Block) int {
	scale := block.FontScale
	if scale == 0 {
		scale = 100
	}
	return layout.settings.FontSize * scale / 100
}

// lineHeight returns the height of block's lines of text at size, before a larger font or an
// image makes a line higher.
func (layout *paginator) lineHeight(block document.Block, size int) int {
	spacing := block.LineHeight
	if spacing == 0 {
		spacing = layout.settings.LineSpacing
	}
	return size * spacing / 100
}
