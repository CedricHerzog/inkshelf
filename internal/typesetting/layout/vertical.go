package layout

import (
	"image"
	"image/color"
	"slices"
	"sort"
	"unicode/utf8"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

//go:generate go run gen_vertical_orientation.go ../../../build/ucd/VerticalOrientation-16.0.0.txt vertical_orientation.go

// Vertical text, as Chinese and Japanese books set it, goes down in columns, the next column to
// the left (vertical-rl), or to the right (vertical-lr). It is laid out as horizontal text, with
// the same line breaking, justification and pagination, in logical coordinates: a line goes
// across the page's height, and lines stack across its width. Only when a page is done is it
// turned onto the screen: the logical x of a word becomes its y, and its logical y its x (from
// the right in vertical-rl).
//
// What changes is how words are measured and drawn. Ideographs and kana stay upright, one under
// the other, and are shaped top to bottom, so that brackets and the long vowel mark take their
// vertical forms. Latin words are drawn as a horizontal line turned a quarter turn clockwise.
// Which characters stay upright is Unicode's Vertical_Orientation property (UAX #50), which
// text-orientation can override; a year or a number (text-combine-upright) is drawn across, in
// the cell of one character.
//
// A block whose lines go the other way than the page's (a horizontal table in a vertical book,
// or vertical text in a horizontal one) is laid out apart in its own direction, on a page of the
// screen's size, and placed as one box where the next block goes.

// VerticalText is how a word of vertical text is drawn.
type VerticalText uint8

// The ways of drawing vertical text.
const (
	// Horizontal is horizontal text: not vertical.
	Horizontal VerticalText = iota
	// Upright characters are drawn one under the other, upright, centred on the column.
	Upright
	// Sideways text is drawn as a horizontal line turned a quarter turn clockwise.
	Sideways
	// Combined text is drawn across, squeezed into the cell of one character.
	Combined
)

// verticalOrientation is a character's Vertical_Orientation (UAX #50): how vertical text draws
// it.
type verticalOrientation uint8

// The vertical orientations. The transformed ones are upright in a form of their own, which the
// font's vertical features give (a bracket turned, a small kana moved): they are drawn upright
// and shaped top to bottom.
const (
	orientationRotated            verticalOrientation = iota // R: sideways
	orientationUpright                                       // U
	orientationTransformedUpright                            // Tu
	orientationTransformedRotated                            // Tr
)

// orientationRange is a range of characters of one vertical orientation, both ends included.
type orientationRange struct {
	low, high   rune
	orientation verticalOrientation
}

// verticalOrientationOf returns character's Vertical_Orientation, from the table that
// gen_vertical_orientation.go generates.
func verticalOrientationOf(character rune) verticalOrientation {
	index := sort.Search(len(verticalOrientations), func(index int) bool { return verticalOrientations[index].high >= character })
	if index < len(verticalOrientations) && verticalOrientations[index].low <= character {
		return verticalOrientations[index].orientation
	}
	return orientationRotated
}

// characterForm returns how vertical text whose text-orientation is orientation draws character.
func characterForm(character rune, orientation document.TextOrientation) VerticalText {
	switch orientation {
	case document.OrientationUpright:
		return Upright
	case document.OrientationSideways:
		return Sideways
	}
	if verticalOrientationOf(character) == orientationRotated {
		return Sideways
	}
	return Upright
}

// combines reports whether vertical text draws run across, in one character's cell: all of it,
// or a number of no more digits than its Combine says.
func combines(run document.Run) bool {
	switch run.Combine {
	case document.CombineNone:
		return false
	case document.CombineAll:
		return run.Text != ""
	}
	digits := 0
	for _, character := range run.Text {
		if character < '0' || character > '9' {
			return false
		}
		digits++
	}
	return digits > 0 && digits <= int(run.Combine)
}

// turned returns the settings that a document in a vertical writing mode is laid out with, in
// logical coordinates: the page's width and height swapped, its top and bottom margins at the
// start and end of the lines, and the margin of the side lines stack from at the top.
func (settings Settings) turned(mode document.WritingMode) Settings {
	turned := settings
	turned.Width, turned.Height = settings.Height, settings.Width
	margins := settings.Margins // amounts: left and top in Min, right and bottom in Max
	first, last := margins.Max.X, margins.Min.X
	if mode == document.VerticalLR {
		first, last = margins.Min.X, margins.Max.X
	}
	// Not image.Rect, which would sort the corners: these are four amounts, not a rectangle.
	turned.Margins = image.Rectangle{Min: image.Pt(margins.Min.Y, first), Max: image.Pt(margins.Max.Y, last)}
	return turned
}

// turnedSizer returns sizer with the widths and heights swapped, as a vertical layout sees an
// image: as wide as it is high along the line. A nil sizer stays nil.
func turnedSizer(sizer ImageSizer) ImageSizer {
	if sizer == nil {
		return nil
	}
	return func(path string) (image.Point, error) {
		size, err := sizer(path)
		return image.Pt(size.Y, size.X), err
	}
}

// pageTurner maps the logical coordinates of a vertical layout onto the page, width pixels wide.
type pageTurner struct {
	mode  document.WritingMode
	width int
}

// across returns the x on the page of a logical y: the distance from the side lines stack from.
func (turner pageTurner) across(logicalY int) int {
	if turner.mode == document.VerticalLR {
		return logicalY
	}
	return turner.width - logicalY
}

// rect returns a logical rectangle on the page.
func (turner pageTurner) rect(rect image.Rectangle) image.Rectangle {
	return image.Rect(turner.across(rect.Min.Y), rect.Min.X, turner.across(rect.Max.Y), rect.Max.X)
}

// turnPage turns what a vertical layout placed on page onto the page. A word of vertical text
// gets its column's centre line as its X and the top of its text as its Top: its font's line
// from Top down was across the logical line, and the column's centre is that line's middle. An
// emphasis mark's square stays a square.
func (turner pageTurner) turnPage(page *Page, faces *Faces) error {
	for lineIndex := range page.Lines {
		words := page.Lines[lineIndex].Words
		for index := range words {
			word := &words[index]
			if word.Mark != NoMark {
				square := turner.rect(image.Rect(word.X, word.Top, word.X+word.Size, word.Top+word.Size))
				word.X, word.Top = square.Min.X, square.Min.Y
				continue
			}
			font, err := faces.FamilyFont(word.Family, word.Style, word.Size)
			if err != nil {
				return err
			}
			centre := word.Top + (font.Ascent()+font.Descent())/2
			word.X, word.Top = turner.across(centre), word.X
		}
	}
	for index := range page.Images {
		page.Images[index].Rect = turner.rect(page.Images[index].Rect)
	}
	for index := range page.Rules {
		page.Rules[index] = turner.rect(page.Rules[index])
	}
	for index := range page.Strokes {
		page.Strokes[index].Rectangle = turner.rect(page.Strokes[index].Rectangle)
	}
	for index := range page.Links {
		page.Links[index].Rect = turner.rect(page.Links[index].Rect)
	}
	for index := range page.Boxes {
		page.Boxes[index].Rect = turner.rect(page.Boxes[index].Rect)
	}
	for index := range page.orthogonal {
		page.orthogonal[index].rect = turner.rect(page.orthogonal[index].rect)
	}
	return nil
}

// orthogonalBox is blocks whose lines go the other way than the page's, laid out apart, as a page
// of their own, and placed on the page as one box.
type orthogonalBox struct {
	// rect is where the box goes, in the coordinates of the layout that placed it, then of the
	// page once turned.
	rect image.Rectangle
	// content is laid out on the page, in its coordinates, and used is the rectangle it covers.
	content Page
	used    image.Rectangle
}

// isOrthogonal reports whether block goes in an orthogonal box: text whose lines go the other way
// than the layout's. Images and rules have no direction of their own.
func (layout *paginator) isOrthogonal(block document.Block) bool {
	return layout.page != nil && !layout.page.Horizontal && block.Kind != document.Image && block.Kind != document.Rule &&
		block.WritingMode.Vertical() != layout.mode.Vertical()
}

// orthogonalGroup returns how many blocks from the first go in one orthogonal box: those whose
// lines go the same way as the first's, images and rules among them.
func (layout *paginator) orthogonalGroup(blocks []document.Block) int {
	vertical := blocks[0].WritingMode.Vertical()
	count := 1
	for count < len(blocks) && blocks[count].WritingMode.Vertical() == vertical {
		count++
	}
	return count
}

// placeOrthogonal places blocks whose lines go the other way than the layout's: laid out apart in
// their own writing mode, on pages of the page's size, each page placed as one box, from the
// start of the line, where the next block goes.
func (layout *paginator) placeOrthogonal(blocks []document.Block) error {
	first, last := blocks[0], blocks[len(blocks)-1]
	apart := &document.Document{
		Blocks: blocks, Chars: layout.doc.Chars, Language: layout.doc.Language, Languages: layout.doc.Languages,
		Fonts: layout.doc.Fonts, WritingMode: first.WritingMode,
	}
	end := last.Start + last.Length()
	for _, borderBox := range layout.doc.Boxes {
		if borderBox.Start >= first.Start && borderBox.End <= end {
			apart.Boxes = append(apart.Boxes, borderBox)
		}
	}
	pages, err := paginateIn(first.WritingMode, apart, *layout.page, layout.faces, layout.pageSizer)
	if err != nil {
		return err
	}
	layout.startBlock(first)
	for index, content := range pages {
		used, err := layout.faces.extent(content)
		if err != nil {
			return err
		}
		if used.Empty() {
			continue
		}
		along, across := used.Dx(), used.Dy() // in the layout's coordinates
		if layout.mode.Vertical() {
			along, across = used.Dy(), used.Dx()
		}
		start := content.Start
		if index == 0 {
			start = first.Start
		}
		if across > layout.roomLeft() && !layout.pageIsEmpty() {
			layout.newPage(start)
		}
		left := layout.content.Min.X
		page := &layout.pages[len(layout.pages)-1]
		page.orthogonal = append(page.orthogonal, orthogonalBox{rect: image.Rect(left, layout.y, left+along, layout.y+across), content: content, used: used})
		layout.y += across
	}
	layout.marginAfter = layout.settings.em(last.MarginBottom)
	return nil
}

// placeOrthogonalBoxes moves the content of page's orthogonal boxes onto it, each where its box
// went.
func placeOrthogonalBoxes(page *Page) {
	for _, box := range page.orthogonal {
		offset := box.rect.Min.Sub(box.used.Min)
		page.addMoved(box.content, offset)
		for _, placed := range box.content.Boxes {
			placed.Rect = placed.Rect.Add(offset)
			page.Boxes = append(page.Boxes, placed)
		}
	}
	page.orthogonal = nil
}

// extent returns the rectangle that what page shows covers, empty if it shows nothing.
func (faces *Faces) extent(page Page) (image.Rectangle, error) {
	var covered image.Rectangle
	for _, line := range page.Lines {
		for _, word := range line.Words {
			rect, err := faces.wordRect(word)
			if err != nil {
				return image.Rectangle{}, err
			}
			covered = covered.Union(rect)
		}
	}
	for _, placed := range page.Images {
		covered = covered.Union(placed.Rect)
	}
	for _, rule := range page.Rules {
		covered = covered.Union(rule)
	}
	for _, stroke := range page.Strokes {
		covered = covered.Union(stroke.Rectangle)
	}
	for _, placed := range page.Boxes {
		covered = covered.Union(placed.Rect)
	}
	return covered, nil
}

// wordRect returns the rectangle a placed word takes: its width by its font's line, or for
// vertical text its column by its length.
func (faces *Faces) wordRect(word PlacedWord) (image.Rectangle, error) {
	if word.Mark != NoMark {
		return image.Rect(word.X, word.Top, word.X+word.Size, word.Top+word.Size), nil
	}
	font, err := faces.FamilyFont(word.Family, word.Style, word.Size)
	if err != nil {
		return image.Rectangle{}, err
	}
	thickness, length := font.Ascent()+font.Descent(), faces.width(word.format(), word.Text)
	if word.Vertical != Horizontal {
		return image.Rect(word.X-thickness/2, word.Top, word.X+thickness-thickness/2, word.Top+length), nil
	}
	return image.Rect(word.X, word.Top, word.X+length, word.Top+thickness), nil
}

// uprightGlyphs returns part shaped top to bottom in font, as format draws it upright: with its
// letter spacing added after each glyph.
func uprightGlyphs(font *fonts.Font, part string, format textFormat) []fonts.Glyph {
	language := ""
	if format.shaping != nil {
		language = format.shaping.Language
	}
	glyphs := font.ShapeVertical(part, language)
	if format.spacing != 0 {
		glyphs = slices.Clone(glyphs)
		for index := range glyphs {
			glyphs[index].Advance += format.spacing
		}
	}
	return glyphs
}

// verticalWidth returns the length of vertical text in format, down its column.
func (faces *Faces) verticalWidth(format textFormat, text string) int {
	if format.vertical == Combined {
		font, err := faces.FamilyFont(format.family, format.style, format.size)
		if err != nil {
			return 0
		}
		return font.Size()
	}
	width := 0
	_ = faces.forEachFontPart(format, text, func(font *fonts.Font, part string, _ int) {
		width += fonts.WidthOf(uprightGlyphs(font, part, format))
	})
	return width
}

// drawVertical draws vertical text in format into dst, in gray: at's X is its column's centre
// line, and its Y the top of the text.
func (faces *Faces) drawVertical(dst *image.Gray, format textFormat, at image.Point, text string, gray color.Gray) error {
	switch format.vertical {
	case Sideways:
		return faces.drawSideways(dst, format, at, text, gray)
	case Combined:
		return faces.drawCombined(dst, format, at, text, gray)
	}
	y := at.Y
	return faces.forEachFontPart(format, text, func(font *fonts.Font, part string, _ int) {
		glyphs := uprightGlyphs(font, part, format)
		font.DrawGlyphsVertical(dst, image.Pt(at.X, y), glyphs, gray)
		y += fonts.WidthOf(glyphs)
	})
}

// drawSideways draws text turned a quarter turn clockwise, its baseline down the column so that
// its font's ascent and descent are centred on it, the tops of its letters to the right.
func (faces *Faces) drawSideways(dst *image.Gray, format textFormat, at image.Point, text string, gray color.Gray) error {
	font, err := faces.FamilyFont(format.family, format.style, format.size)
	if err != nil {
		return err
	}
	baseline := at.X - (font.Ascent()-font.Descent())/2
	horizontal := format
	horizontal.vertical = Horizontal
	// Fallback fonts share the baseline, whatever their ascent: the parts' tops are ignored.
	if shaping := format.shaping; shaping != nil {
		var parts []shapedPart
		err := faces.forEachFontPart(horizontal, text, func(font *fonts.Font, part string, _ int) {
			parts = append(parts, shapedPart{font: font, glyphs: font.Shape(part, shaping.RightToLeft, shaping.Language)})
		})
		if shaping.RightToLeft {
			slices.Reverse(parts)
		}
		y := at.Y
		for _, part := range parts {
			part.font.DrawGlyphsTurned(dst, image.Pt(baseline, y), part.glyphs, gray)
			y += fonts.WidthOf(part.glyphs)
		}
		return err
	}
	y := at.Y
	return faces.forEachFontPart(horizontal, text, func(font *fonts.Font, part string, _ int) {
		font.DrawTurned(dst, image.Pt(baseline, y), part, gray, format.spacing)
		y += font.Width(part) + format.spacing*utf8.RuneCountInString(part)
	})
}

// drawCombined draws text across, in the cell of one character centred on the column: drawn
// apart, then squeezed to the cell's width if it is wider.
func (faces *Faces) drawCombined(dst *image.Gray, format textFormat, at image.Point, text string, gray color.Gray) error {
	font, err := faces.FamilyFont(format.family, format.style, format.size)
	if err != nil {
		return err
	}
	horizontal := format
	horizontal.vertical = Horizontal
	width, height := faces.width(horizontal, text), font.Ascent()+font.Descent()
	if width <= 0 || height <= 0 {
		return nil
	}
	apart := image.NewGray(image.Rect(0, 0, width, height))
	for index := range apart.Pix {
		apart.Pix[index] = 0xff
	}
	if err := faces.draw(apart, horizontal, image.Point{}, text, gray); err != nil {
		return err
	}
	cell := font.Size()
	squeezed := min(width, cell)
	left, top := at.X-squeezed/2, at.Y+(cell-height)/2
	for row := range height {
		for column := range squeezed {
			// Each pixel of the cell averages the columns of the text it stands for.
			from, to := column*width/squeezed, (column+1)*width/squeezed
			sum := 0
			for source := from; source < to; source++ {
				sum += int(apart.Pix[row*apart.Stride+source])
			}
			point := image.Pt(left+column, top+row)
			if !point.In(dst.Rect) {
				continue
			}
			// The text is darker than its paper: the darker of the two shows.
			offset := dst.PixOffset(point.X, point.Y)
			dst.Pix[offset] = min(dst.Pix[offset], uint8(sum/(to-from)))
		}
	}
	return nil
}
