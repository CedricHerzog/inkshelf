package layout

import (
	"image"
	"unicode/utf8"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// A float is a group of blocks taken out of the text and pushed to its left or right side, the
// text flowing around it: a picture with its caption, a sidebar (CSS float). The paginator lays
// the group out apart, in its own width, and places it where the next block starts; the lines
// of the blocks after it are shortened while they are beside it.
//
// A float that does not fit on the rest of the page waits for the next one while the text goes
// on. One too wide, or on a page too narrow for text beside it, stays in the text as ordinary
// blocks. A floated letter or two much larger than the paragraph after it is a drop cap: it sits
// on the baseline of one of the paragraph's first lines, as in print.

// Lengths are in hundredths of an em, widths in percent of the text's width.
const (
	// maxFloatWidth is the widest a float may be: a wider one stays in the text.
	maxFloatWidth = 60
	// autoFloatWidth is the widest a float of text is when the book gives it no width.
	autoFloatWidth = 40
	// minBesideWidth is the narrowest text beside floats: a few words at the reader's size.
	// Narrower text would break after every word, so it goes below the floats instead.
	minBesideWidth = 900
	// floatGap is the space between a float and the text beside it. Under a float is its bottom
	// margin, an em at most.
	floatGap = 50
	// dropCapGap is the space between a drop cap and its paragraph's lines.
	dropCapGap = 20
	// dropCapScale is how much larger than its paragraph's text a drop cap is at least, in
	// percent; dropCapRunes is how many runes it has at most (a letter and a quotation mark).
	dropCapScale = 150
	dropCapRunes = 3
)

// floatLayout is the floats of a document being paginated.
type floatLayout struct {
	waiting  [][]document.Block // the groups met since the last block placed, placed with the next
	deferred [][]document.Block // the groups that wait for the next page
	// deferredPage is the page the deferred groups did not fit on.
	deferredPage int
	placed       []placedFloat
}

// placedFloat is a float placed on a page: the area it keeps the text out of, gaps included.
type placedFloat struct {
	side document.Float
	rect image.Rectangle
	page int
}

// collectFloat keeps block with its float group if it floats, and reports whether it does. The
// group is placed when the next block in the text is, as it floats beside that block.
func (layout *paginator) collectFloat(block document.Block) bool {
	if block.Float == document.NoFloat {
		return false
	}
	floats := &layout.floats
	if last := len(floats.waiting) - 1; last >= 0 && floats.waiting[last][0].FloatGroup == block.FloatGroup {
		floats.waiting[last] = append(floats.waiting[last], block)
	} else {
		floats.waiting = append(floats.waiting, []document.Block{block})
	}
	return true
}

// placeFloats places, where block starts, the groups deferred from an earlier page, then those
// met before it, and moves block below the floats it clears: an image or a rule clears them all.
// block may lose its indent to a drop cap, or take the drop cap in as its first runs.
func (layout *paginator) placeFloats(block *document.Block) error {
	floats := &layout.floats
	if len(floats.placed) == 0 && len(floats.waiting) == 0 && len(floats.deferred) == 0 {
		return nil
	}
	if len(floats.deferred) > 0 && len(layout.pages)-1 > floats.deferredPage {
		deferred := floats.deferred
		floats.deferred = nil
		for _, group := range deferred {
			if err := layout.placeFloatGroup(group, nil); err != nil {
				return err
			}
		}
	}
	waiting := floats.waiting
	floats.waiting = nil
	for index, group := range waiting {
		var next *document.Block
		if index == len(waiting)-1 {
			next = block
		}
		if err := layout.placeFloatGroup(group, next); err != nil {
			return err
		}
	}
	sides := block.Clear
	if block.Kind == document.Image || block.Kind == document.Rule {
		sides = document.ClearBoth
	}
	if sides != 0 {
		layout.y = max(layout.y, layout.floatBottom(sides))
	}
	return nil
}

// finishFloats places the groups left at the end of the document: those met last where the
// text ends, and those deferred on pages of their own.
func (layout *paginator) finishFloats() error {
	floats := &layout.floats
	waiting := floats.waiting
	floats.waiting = nil
	for _, group := range waiting {
		if err := layout.placeFloatGroup(group, nil); err != nil {
			return err
		}
	}
	for len(floats.deferred) > 0 {
		deferred := floats.deferred
		floats.deferred = nil
		layout.newPage(max(deferred[0][0].Start, layout.lastPageEnd()))
		for _, group := range deferred {
			if err := layout.placeFloatGroup(group, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// lastPageEnd returns the offset past the last line of the last page, or its start if it has no
// lines.
func (layout *paginator) lastPageEnd() int {
	page := layout.pages[len(layout.pages)-1]
	end := page.Start
	for _, line := range page.Lines {
		end = max(end, line.End)
	}
	return end
}

// lastPageFloats returns the floats of the last page, forgetting those of earlier pages.
func (layout *paginator) lastPageFloats() []placedFloat {
	floats := layout.floats.placed
	if len(floats) > 0 && floats[0].page != len(layout.pages)-1 {
		layout.floats.placed = nil
		return nil
	}
	return floats
}

// floatBottom returns the bottom of the lowest float of sides on the last page, 0 if none.
func (layout *paginator) floatBottom(sides document.Clear) int {
	bottom := 0
	for _, float := range layout.lastPageFloats() {
		if float.side == document.FloatLeft && sides&document.ClearLeft != 0 || float.side == document.FloatRight && sides&document.ClearRight != 0 {
			bottom = max(bottom, float.rect.Max.Y)
		}
	}
	return bottom
}

// besideFloats reports whether a float of the last page goes below the current height.
func (layout *paginator) besideFloats() bool {
	for _, float := range layout.lastPageFloats() {
		if float.rect.Max.Y > layout.y {
			return true
		}
	}
	return false
}

// nextFloatBottom returns the first bottom of the last page's floats below top: where the text
// gets wider again. It returns top if no float goes below it.
func (layout *paginator) nextFloatBottom(top int) int {
	bottom := 0
	for _, float := range layout.lastPageFloats() {
		if float.rect.Max.Y > top && (bottom == 0 || float.rect.Max.Y < bottom) {
			bottom = float.rect.Max.Y
		}
	}
	return max(bottom, top)
}

// floatInsets returns how many pixels the floats of the last page take off the left and the
// right of block's text from top to bottom.
func (layout *paginator) floatInsets(block document.Block, top, bottom int) (left, right int) {
	textLeft, width := layout.textArea(block)
	textRight := textLeft + width
	for _, float := range layout.lastPageFloats() {
		if float.rect.Min.Y >= bottom || float.rect.Max.Y <= top {
			continue
		}
		if float.side == document.FloatLeft {
			left = max(left, float.rect.Max.X-textLeft)
		} else {
			right = max(right, textRight-float.rect.Min.X)
		}
	}
	return left, right
}

// placeBeside places block's lines beside the floats of the page, each line as wide as the
// floats beside it leave it. Where that width changes, the rest of the block is laid out again in
// the new width; below the floats, it is placed as usual.
func (layout *paginator) placeBeside(block document.Block, next *document.Block) error {
	size := layout.blockSize(block)
	estimate := layout.lineHeight(block, size)
	layout.placeBorder(block, false)
	rest := block
	rest.BorderTop = false
	span := estimate // the height beside which the first line is laid out
	below := func() {
		// Move below the first float that ends.
		layout.y, span = layout.nextFloatBottom(layout.y), estimate
		if layout.roomLeft() < estimate {
			layout.newPage(rest.Start)
		}
	}
	for {
		if !layout.besideFloats() {
			return layout.placeText(rest, next)
		}
		left, right := layout.floatInsets(rest, layout.y, layout.y+span)
		if _, width := layout.textArea(rest); width-left-right < layout.settings.em(minBesideWidth) {
			below() // too narrow beside the floats
			continue
		}
		narrowed := rest
		narrowed.Left += layout.toEm(left)
		narrowed.Right += layout.toEm(right)
		lines, err := layout.layOutLines(narrowed, size)
		if err != nil || len(lines) == 0 {
			return err
		}
		if first := lines[0]; first.height > span {
			// The line is higher than it was laid out for and meets other floats: lay it out
			// again beside them, once, and below them if it is still too high.
			if lineLeft, lineRight := layout.floatInsets(rest, layout.y, layout.y+first.height); lineLeft != left || lineRight != right {
				if span > estimate {
					below()
				} else {
					span = first.height
				}
				continue
			}
		}
		span = estimate
		placed := 0
		for index, line := range lines {
			if line.height > layout.roomLeft() && !layout.pageIsEmpty() {
				layout.newPage(line.Start)
				break
			}
			if index > 0 {
				if lineLeft, lineRight := layout.floatInsets(rest, layout.y, layout.y+line.height); lineLeft != left || lineRight != right {
					break
				}
			}
			layout.addLine(line)
			placed++
		}
		if placed == len(lines) {
			return nil
		}
		rest = restOfBlock(rest, lines[placed].Start)
	}
}

// toEm converts pixels to hundredths of an em, rounded up.
func (layout *paginator) toEm(pixels int) int {
	return (pixels*100 + layout.settings.FontSize - 1) / layout.settings.FontSize
}

// restOfBlock returns block's text from offset on, as a block of its own: without the first
// line's indent nor the top border, which the part before had.
func restOfBlock(block document.Block, offset int) document.Block {
	rest := block
	rest.Start, rest.Indent, rest.BorderTop, rest.PageBreak = offset, 0, false, false
	rest.Runs, rest.Scripts = nil, document.Scripts{}
	position := block.Start
	for index, run := range block.Runs {
		length := 1
		if run.Image == "" {
			length = utf8.RuneCountInString(run.Text)
		}
		if position+length <= offset {
			position += length
			continue
		}
		if skipped := offset - position; skipped > 0 {
			cut := 0
			for range skipped {
				_, size := utf8.DecodeRuneInString(run.Text[cut:])
				cut += size
			}
			run.Text = run.Text[cut:]
		}
		rest.Runs = append(append(rest.Runs, run), block.Runs[index+1:]...)
		break
	}
	return rest
}

// placeFloatGroup places a float group where the text is; next is the block after it, nil if
// none.
func (layout *paginator) placeFloatGroup(group []document.Block, next *document.Block) error {
	if layout.isDropCap(group, next) {
		return layout.placeDropCap(group[0], next)
	}
	textWidth := layout.content.Dx()
	width, err := layout.floatGroupWidth(group)
	if err != nil {
		return err
	}
	if width <= 0 || width*100 > textWidth*maxFloatWidth {
		return layout.placeInText(group)
	}
	inside, err := layout.layOutApart(group, width, false)
	if err != nil {
		return err
	}
	// A word wider than the group would stick out of it: the group widens to hold it.
	width = max(width, layout.contentWidth(inside.pages))
	gap := layout.settings.em(floatGap)
	if len(inside.pages) > 1 || width*100 > textWidth*maxFloatWidth || textWidth-width-gap < layout.settings.em(minBesideWidth) {
		return layout.placeInText(group) // taller than a page, too wide, or on too narrow a page
	}
	height := inside.y
	top := layout.y
	if !layout.pageIsEmpty() {
		top += layout.settings.em(group[0].MarginTop)
	}
	if group[0].Clear != 0 {
		top = max(top, layout.floatBottom(group[0].Clear))
	}
	side := group[0].Float
	x, top := layout.floatPosition(group[0], top, height, width, gap)
	if top+height > layout.content.Max.Y && !layout.pageIsEmpty() {
		if len(layout.floats.deferred) == 0 {
			layout.floats.deferredPage = len(layout.pages) - 1
		}
		layout.floats.deferred = append(layout.floats.deferred, group)
		return nil
	}
	bottom := top + height + min(layout.settings.em(group[len(group)-1].MarginBottom), layout.settings.FontSize)
	rect := image.Rect(x, top, x+width+gap, bottom)
	if side == document.FloatRight {
		rect = image.Rect(x-gap, top, x+width, bottom)
	}
	layout.pages[len(layout.pages)-1].addMoved(inside.pages[0], image.Pt(x, top))
	layout.floats.placed = append(layout.lastPageFloats(), placedFloat{side: side, rect: rect, page: len(layout.pages) - 1})
	return nil
}

// floatPosition returns where a float group, whose first block is first, width wide and height
// high, goes from top: at its side of the text, beside the floats it meets if that leaves the
// text room enough, else below them.
func (layout *paginator) floatPosition(first document.Block, top, height, width, gap int) (x, placedTop int) {
	textLeft, textWidth := layout.textArea(first)
	for {
		left, right, lowest := textLeft, textLeft+textWidth, 0
		for _, float := range layout.lastPageFloats() {
			if float.rect.Min.Y >= top+height || float.rect.Max.Y <= top {
				continue
			}
			if float.side == document.FloatLeft {
				left = max(left, float.rect.Max.X)
			} else {
				right = min(right, float.rect.Min.X)
			}
			if lowest == 0 || float.rect.Max.Y < lowest {
				lowest = float.rect.Max.Y
			}
		}
		if lowest == 0 || right-left-width-gap >= layout.settings.em(minBesideWidth) {
			if first.Float == document.FloatRight {
				return right - width, top
			}
			return left, top
		}
		top = lowest
	}
}

// placeInText places the blocks of a group that cannot float where they are, as ordinary text.
func (layout *paginator) placeInText(group []document.Block) error {
	for index, block := range group {
		block.Float = document.NoFloat
		var next *document.Block
		if index+1 < len(group) {
			next = &group[index+1]
		}
		if err := layout.placeBlock(block, next); err != nil {
			return err
		}
	}
	return nil
}

// layOutApart lays a group's blocks out on a page of their own, width wide and as high as the
// text. With startAligned, the lines are neither centred nor justified, so that the widest line
// tells how wide the content is.
func (layout *paginator) layOutApart(group []document.Block, width int, startAligned bool) (*paginator, error) {
	settings := layout.settings
	settings.Width, settings.Height, settings.Margins = width, layout.content.Dy(), image.Rectangle{}
	inside := &paginator{settings: settings, faces: layout.faces, sizer: layout.sizer, hyphenator: layout.hyphenator,
		doc: layout.doc, content: settings.content(), pageHeight: layout.pageHeight}
	inside.newPage(group[0].Start)
	blocks := make([]document.Block, len(group))
	for index, block := range group {
		// The first block's margins are those of the text around the group: inside, they count
		// from there.
		block.Left, block.Right = max(block.Left-group[0].Left, 0), max(block.Right-group[0].Right, 0)
		block.Float, block.Clear, block.PageBreak = document.NoFloat, 0, false
		if block.Kind == document.Image && block.ImageWidth > 0 {
			// A percentage of the text's width becomes one of the group's.
			block.ImageWidth = min(block.ImageWidth*layout.content.Dx()/width, 100)
		}
		if startAligned {
			block.Align, block.AlignLast = document.AlignLeft, document.AlignLeft
		}
		blocks[index] = block
	}
	for index, block := range blocks {
		var next *document.Block
		if index+1 < len(blocks) {
			next = &blocks[index+1]
		}
		if err := inside.placeBlock(block, next); err != nil {
			return nil, err
		}
	}
	return inside, nil
}

// floatGroupWidth returns the width of a float group: its FloatWidth if the book gives one, else
// its content's, text being autoFloatWidth percent of the page's width at most. It is 0 for a
// group with nothing to show.
func (layout *paginator) floatGroupWidth(group []document.Block) (int, error) {
	textWidth := layout.content.Dx()
	if group[0].FloatWidth > 0 {
		return max(textWidth*group[0].FloatWidth/100, 1), nil
	}
	widest, hasText := 0, false
	for _, block := range group {
		switch {
		case block.Kind == document.Image && block.ImageWidth > 0:
			widest = max(widest, textWidth*block.ImageWidth/100)
		case block.Kind == document.Image:
			if size, err := layout.sizer(block.Image); err == nil && size.X > 0 {
				widest = max(widest, size.X*layout.settings.FontSize/16) // in CSS pixels
			}
		default:
			hasText = true
		}
	}
	if hasText {
		measured, err := layout.layOutApart(group, max(textWidth*autoFloatWidth/100, widest), true)
		if err != nil {
			return 0, err
		}
		// A pixel or two more than measured, as rounding may make the same lines a little wider.
		widest = max(widest, layout.contentWidth(measured.pages)+2)
	}
	return widest, nil
}

// contentWidth returns how far from the left of their pages the words and images of pages reach.
func (layout *paginator) contentWidth(pages []Page) int {
	width := 0
	for _, page := range pages {
		for _, line := range page.Lines {
			for _, word := range line.Words {
				width = max(width, word.X+layout.faces.width(word.format(), word.Text))
			}
		}
		for _, placed := range page.Images {
			width = max(width, placed.Rect.Max.X)
		}
	}
	return width
}

// addMoved copies what from shows onto page, moved by offset.
func (page *Page) addMoved(from Page, offset image.Point) {
	for _, line := range from.Lines {
		words := make([]PlacedWord, len(line.Words))
		for index, word := range line.Words {
			word.X += offset.X
			word.Top += offset.Y
			words[index] = word
		}
		line.Words = words
		page.Lines = append(page.Lines, line)
	}
	for _, placed := range from.Images {
		placed.Rect = placed.Rect.Add(offset)
		page.Images = append(page.Images, placed)
	}
	for _, rule := range from.Rules {
		page.Rules = append(page.Rules, rule.Add(offset))
	}
	for _, stroke := range from.Strokes {
		page.Strokes = append(page.Strokes, Stroke{Rectangle: stroke.Add(offset), Gray: stroke.Gray})
	}
	for _, link := range from.Links {
		link.Rect = link.Rect.Add(offset)
		page.Links = append(page.Links, link)
	}
}

// isDropCap reports whether a float group is a drop cap of next: a letter or two of text, much
// larger than next's, floated left just before it.
func (layout *paginator) isDropCap(group []document.Block, next *document.Block) bool {
	if len(group) != 1 || next == nil || next.Kind != document.Paragraph || group[0].Kind != document.Paragraph || group[0].Float != document.FloatLeft {
		return false
	}
	capBlock := group[0]
	if capBlock.Start+capBlock.Length() != next.Start {
		return false
	}
	runes, largest := 0, 0
	for _, run := range capBlock.Runs {
		if run.Image != "" {
			return false
		}
		runes += utf8.RuneCountInString(run.Text)
		largest = max(largest, layout.runSize(capBlock, run))
	}
	return runes > 0 && runes <= dropCapRunes && largest*100 >= layout.blockSize(*next)*dropCapScale
}

// runSize returns the font size of run of block, in pixels.
func (layout *paginator) runSize(block document.Block, run document.Run) int {
	size := layout.blockSize(block)
	if run.FontScale > 0 {
		size = size * run.FontScale / 100
	}
	return size
}

// placeDropCap places a drop cap at the left of the paragraph next, which loses its indent. The
// cap's baseline goes on the baseline of the paragraph's line nearest to it, and the lines down
// to that one go beside it. A cap less than two lines high, or with no room beside it, goes into
// the paragraph instead, as its first runs.
func (layout *paginator) placeDropCap(capBlock document.Block, next *document.Block) error {
	width, err := layout.floatGroupWidth([]document.Block{capBlock})
	if err != nil {
		return err
	}
	inside, err := layout.layOutApart([]document.Block{capBlock}, max(width, 1), false)
	if err != nil {
		return err
	}
	page := inside.pages[0]
	if len(inside.pages) > 1 || len(page.Lines) != 1 || len(page.Lines[0].Words) == 0 {
		layout.mergeDropCap(capBlock, next)
		return nil
	}
	word := page.Lines[0].Words[0]
	font, err := layout.faces.FamilyFont(word.Family, word.Style, word.Size)
	if err != nil {
		return err
	}
	capBaseline := word.Top + font.Ascent()
	size := layout.blockSize(*next)
	textFont, err := layout.faces.Font(0, size)
	if err != nil {
		return err
	}
	lineHeight, textAscent := layout.lineHeight(*next, size), textFont.Ascent()
	lines := (capBaseline - textAscent + lineHeight/2) / lineHeight // beside it, below the first
	gap := layout.settings.em(dropCapGap)
	if lines < 1 || layout.content.Dx()-width-gap < layout.settings.em(minBesideWidth) || layout.besideFloats() {
		layout.mergeDropCap(capBlock, next)
		return nil
	}
	if layout.y+(lines+1)*lineHeight > layout.content.Max.Y && !layout.pageIsEmpty() {
		layout.newPage(capBlock.Start)
	}
	// The cap may rise a little above the first line, but not into the floats above it.
	top := max(layout.y+lines*lineHeight+textAscent-capBaseline, layout.floatBottom(document.ClearBoth))
	left := layout.content.Min.X + layout.settings.em(capBlock.Left)
	layout.pages[len(layout.pages)-1].addMoved(page, image.Pt(left, top))
	rect := image.Rect(left, layout.y, left+width+gap, max(layout.y+(lines+1)*lineHeight, top+capBaseline))
	layout.floats.placed = append(layout.lastPageFloats(), placedFloat{side: document.FloatLeft, rect: rect, page: len(layout.pages) - 1})
	next.Indent = 0
	return nil
}

// mergeDropCap puts a drop cap back into its paragraph, as its first runs at their own size.
func (layout *paginator) mergeDropCap(capBlock document.Block, next *document.Block) {
	nextSize := layout.blockSize(*next)
	runs := make([]document.Run, 0, len(capBlock.Runs)+len(next.Runs))
	for _, run := range capBlock.Runs {
		run.FontScale = max(layout.runSize(capBlock, run)*100/nextSize, 1)
		runs = append(runs, run)
	}
	next.Runs = append(runs, next.Runs...)
	next.Start, next.Scripts = capBlock.Start, document.Scripts{}
}
