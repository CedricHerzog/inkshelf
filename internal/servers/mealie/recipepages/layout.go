// Package recipepages lays a recipe out into pages and draws them as images, for a server
// whose books are recipes (Mealie). A recipe has no file of its own to read: its title, times,
// ingredients, steps and notes are placed line by line on pages the size of the reader's
// screen, in the Go fonts compiled into the app, then each page is drawn as a PNG that the app
// prepares like a comic's page.
package recipepages

import (
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

// The size of the pages, in InkPad 3 pixels: the app fits them to the screen anyway.
const (
	Width  = 1404
	Height = 1872
)

const (
	margin = 90
	// photoHeight is the height of the box the photo is fitted in, at the top of page 1.
	photoHeight = 680
	// photoGap is the space under the photo.
	photoGap = 50
	// markerWidth is the room left of an ingredient or a step for its bullet or number.
	markerWidth = 70
)

// style is how a line of text is drawn.
type style int

const (
	bodyStyle style = iota
	titleStyle
	headingStyle    // Ingredients, Instructions, Notes
	subheadingStyle // a section of ingredients or steps, a note's title
	metaStyle       // times, yield, tools
)

// styleSpec is the font and size of a style.
type styleSpec struct {
	bold bool
	size float64 // pixels
}

// styles are the styles' fonts and sizes.
var styles = map[style]styleSpec{
	bodyStyle:       {size: 40},
	titleStyle:      {bold: true, size: 68},
	headingStyle:    {bold: true, size: 52},
	subheadingStyle: {bold: true, size: 42},
	metaStyle:       {size: 36},
}

// lineHeight returns the height of a line of text of style.
func lineHeight(textStyle style) int {
	return int(styles[textStyle].size * 1.3)
}

// The Go fonts, compiled in: they have the accents, œ and ° of French recipes.
var (
	regularFont = mustParse(goregular.TTF)
	boldFont    = mustParse(gobold.TTF)
)

// mustParse parses a font. It panics if it cannot, which would be a bug: the fonts are
// compiled in.
func mustParse(data []byte) *sfnt.Font {
	parsed, err := opentype.Parse(data)
	if err != nil {
		panic(err)
	}
	return parsed
}

// faces are a face for each style. A face is not safe for concurrent use: each layout and
// each page drawn makes its own.
type faces map[style]font.Face

// newFaces returns a face for each style.
func newFaces() faces {
	all := faces{}
	for textStyle, spec := range styles {
		parsed := regularFont
		if spec.bold {
			parsed = boldFont
		}
		// At 72 DPI, a point is a pixel.
		face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: spec.size, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			panic(err) // the sizes are constants: a bug
		}
		all[textStyle] = face
	}
	return all
}

// Recipe is what a recipe's pages show. A server turns its own model of a recipe into one,
// choosing which of its fields make each line.
type Recipe struct {
	Name        string
	Times       string // the preparation and cooking times and the yield, on one line, or ""
	Tools       string // the utensils needed, separated by commas, or ""
	Description string
	Ingredients []Entry
	Steps       []Entry // numbered, from 1, those with a Text
	Notes       []Entry
}

// Entry is an ingredient, a step or a note. A non-empty Title is a heading shown before its
// Text: for an ingredient or a step, it starts a section of them. An entry with neither is left
// out (a server may keep empty lines).
type Entry struct {
	Title string
	Text  string
}

// Page is a laid-out page of a recipe, ready to be drawn by Render.
type Page struct {
	photo bool // the recipe's photo goes at the top
	lines []placedLine
}

// placedLine is a line of text at its place on a page: x and y are its top-left corner.
type placedLine struct {
	textStyle style
	text      string
	x, y      int
}

// line is a line of text before it is placed: marker (a bullet or a step number) goes left
// of the text, which is indented by markerWidth if the line belongs to a marked item.
type line struct {
	textStyle style
	text      string
	marker    string
	indented  bool
}

// block is lines kept on one page when they fit, spaceBefore pixels under the previous
// block. keepWithNext keeps it on the page of the next block (a heading).
type block struct {
	lines        []line
	spaceBefore  int
	keepWithNext bool
}

// height returns the height of the block's lines.
func (item block) height() int {
	total := 0
	for _, textLine := range item.lines {
		total += lineHeight(textLine.textStyle)
	}
	return total
}

// Layout lays out recipe into pages. With photo, page 1 starts with the box Render fits the
// recipe's photo in.
func Layout(recipe Recipe, photo bool) []Page {
	return paginate(recipeBlocks(recipe, newFaces()), photo)
}

// paginate places blocks on pages, starting a new page before a block that does not fit on
// the current one but would on an empty one, and splitting blocks taller than a page.
func paginate(blocks []block, photo bool) []Page {
	const top, bottom = margin, Height - margin
	pages := []Page{{photo: photo}}
	y := top
	if photo {
		y += photoHeight + photoGap
	}
	pageStart := y
	newPage := func() {
		pages = append(pages, Page{})
		y, pageStart = top, top
	}
	for index, item := range blocks {
		if y > pageStart {
			y += item.spaceBefore
		}
		needed := item.height()
		if item.keepWithNext && index+1 < len(blocks) && len(blocks[index+1].lines) > 0 {
			next := blocks[index+1]
			needed += next.spaceBefore + next.height()
			if needed > bottom-top { // the next block is split anyway: keep its first line
				needed = item.height() + next.spaceBefore + lineHeight(next.lines[0].textStyle)
			}
		}
		if y+needed > bottom && y > pageStart && needed <= bottom-top {
			newPage()
		}
		for _, textLine := range item.lines {
			height := lineHeight(textLine.textStyle)
			if y+height > bottom && y > pageStart {
				newPage()
			}
			current := &pages[len(pages)-1]
			x := margin
			if textLine.indented {
				x += markerWidth
			}
			if textLine.marker != "" {
				current.lines = append(current.lines, placedLine{textStyle: textLine.textStyle, text: textLine.marker, x: margin, y: y})
			}
			current.lines = append(current.lines, placedLine{textStyle: textLine.textStyle, text: textLine.text, x: x, y: y})
			y += height
		}
	}
	return pages
}

// The space between blocks, in pixels.
const (
	sectionSpace = 56 // before a heading
	itemSpace    = 10 // between ingredients
	stepSpace    = 28 // between steps, notes and paragraphs
)

// recipeBlocks returns the blocks of recipe: its title, times, description, ingredients,
// steps and notes. faces measure the text to wrap it.
func recipeBlocks(recipe Recipe, measure faces) []block {
	textWidth := Width - 2*margin
	var blocks []block
	paragraph := func(textStyle style, text string, spaceBefore int) {
		if lines := wrap(measure[textStyle], text, textWidth); len(lines) > 0 {
			blocks = append(blocks, block{lines: styled(textStyle, lines), spaceBefore: spaceBefore})
		}
	}
	heading := func(textStyle style, text string, spaceBefore int) {
		blocks = append(blocks, block{
			lines:        styled(textStyle, wrap(measure[textStyle], text, textWidth)),
			spaceBefore:  spaceBefore,
			keepWithNext: true,
		})
	}
	item := func(marker, text string, spaceBefore int) {
		lines := styled(bodyStyle, wrap(measure[bodyStyle], text, textWidth-markerWidth))
		for index := range lines {
			lines[index].indented = true
		}
		lines[0].marker = marker
		blocks = append(blocks, block{lines: lines, spaceBefore: spaceBefore})
	}

	paragraph(titleStyle, recipe.Name, 0)
	paragraph(metaStyle, recipe.Times, stepSpace/2)
	if tools := strings.TrimSpace(recipe.Tools); tools != "" {
		paragraph(metaStyle, "Tools: "+tools, 0)
	}
	paragraph(bodyStyle, recipe.Description, stepSpace)

	if ingredients := nonEmpty(recipe.Ingredients); len(ingredients) > 0 {
		heading(headingStyle, "Ingredients", sectionSpace)
		for _, ingredient := range ingredients {
			if title := strings.TrimSpace(ingredient.Title); title != "" {
				heading(subheadingStyle, title, stepSpace)
			}
			if text := strings.TrimSpace(ingredient.Text); text != "" {
				item("•", text, itemSpace)
			}
		}
	}

	if steps := nonEmpty(recipe.Steps); len(steps) > 0 {
		heading(headingStyle, "Instructions", sectionSpace)
		number := 0
		for _, step := range steps {
			if title := strings.TrimSpace(step.Title); title != "" {
				heading(subheadingStyle, title, stepSpace)
			}
			if text := strings.TrimSpace(step.Text); text != "" {
				number++
				item(strconv.Itoa(number)+".", text, stepSpace)
			}
		}
	}

	if notes := nonEmpty(recipe.Notes); len(notes) > 0 {
		heading(headingStyle, "Notes", sectionSpace)
		for _, note := range notes {
			if title := strings.TrimSpace(note.Title); title != "" {
				heading(subheadingStyle, title, stepSpace)
				paragraph(bodyStyle, note.Text, 0)
			} else {
				paragraph(bodyStyle, note.Text, stepSpace)
			}
		}
	}
	return blocks
}

// styled returns texts as lines of textStyle.
func styled(textStyle style, texts []string) []line {
	lines := make([]line, len(texts))
	for index, text := range texts {
		lines[index] = line{textStyle: textStyle, text: text}
	}
	return lines
}

// nonEmpty returns the entries that show something.
func nonEmpty(entries []Entry) []Entry {
	var kept []Entry
	for _, entry := range entries {
		if strings.TrimSpace(entry.Text) != "" || strings.TrimSpace(entry.Title) != "" {
			kept = append(kept, entry)
		}
	}
	return kept
}

// wrap splits text into lines at most width pixels wide in face: at its line breaks, then
// between words, and inside a word too long for a line.
func wrap(face font.Face, text string, width int) []string {
	var lines []string
	for paragraph := range strings.SplitSeq(strings.TrimSpace(text), "\n") {
		current := ""
		for word := range strings.FieldsSeq(paragraph) {
			candidate := word
			if current != "" {
				candidate = current + " " + word
			}
			if font.MeasureString(face, candidate).Ceil() <= width {
				current = candidate
				continue
			}
			if current != "" {
				lines = append(lines, current)
			}
			current = word
			for font.MeasureString(face, current).Ceil() > width {
				head, tail := splitWord(face, current, width)
				lines = append(lines, head)
				current = tail
			}
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	return lines
}

// splitWord splits a word wider than width into its longest start that fits (at least one
// letter) and the rest.
func splitWord(face font.Face, word string, width int) (head, tail string) {
	runes := []rune(word)
	count := 1
	for count < len(runes) && font.MeasureString(face, string(runes[:count+1])).Ceil() <= width {
		count++
	}
	return string(runes[:count]), string(runes[count:])
}
