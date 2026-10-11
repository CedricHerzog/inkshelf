package layout

import (
	"image"
	"log"
	"unicode/utf8"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/mathml"
)

// preparedText is a block's text cut into measured words at a font size, ready to be broken
// into lines. Measuring is the expensive part, so table cells measure their widths from it too.
type preparedText struct {
	block       document.Block // with DirectionAuto resolved, which the alignment needs
	words       []word
	text        blockText
	rightToLeft bool
	baseFont    *fonts.Font
	lineHeight  int
	left, width int // where the text goes across the page
}

// prepareText prepares block's text at size: it gives each run a format, finds what its scripts
// need (break opportunities, bidirectional levels, shaping), and cuts it into words. A block
// without text has no words.
func (layout *paginator) prepareText(block document.Block, size int) (preparedText, error) {
	baseFont, err := layout.faces.Font(0, size)
	if err != nil {
		return preparedText{}, err
	}
	left, width := layout.textArea(block)
	lineHeight := layout.lineHeight(block, size)
	// An inline image must fit in a line, and its line in a page.
	formats, err := layout.makeFormats(block, size, image.Pt(width, layout.pageHeight-max(lineHeight-baseFont.Ascent(), 0)))
	if err != nil {
		return preparedText{}, err
	}
	found := block.Scripts
	if !found.Scanned {
		found = document.ScanScripts(block.Runs)
	}
	text := blockText{start: block.Start, breaks: breakOpportunities(block, found.Wide), shaped: found.Shaped, doc: layout.doc}
	var rightToLeft bool
	text.levels, rightToLeft = bidiLevels(block, found.RightToLeft)
	block.Direction = document.LeftToRight
	if rightToLeft {
		block.Direction = document.RightToLeft
	}
	all := splitFragments(&block, formats, layout.faces, &text)
	if len(all) == 0 {
		return preparedText{}, nil
	}
	return preparedText{
		block: block, words: groupWords(all), text: text, rightToLeft: rightToLeft,
		baseFont: baseFont, lineHeight: lineHeight, left: left, width: width,
	}, nil
}

// makeFormats returns the format of each of block's runs at size: its font opened, its inline
// image or formula sized to fit in limit, and its emphasis marks' font.
func (layout *paginator) makeFormats(block document.Block, size int, limit image.Point) ([]*runFormat, error) {
	formats := make([]runFormat, len(block.Runs))
	pointers := make([]*runFormat, len(block.Runs))
	for index, run := range block.Runs {
		runSize := size
		if run.FontScale > 0 {
			runSize = size * run.FontScale / 100
		}
		font, err := layout.faces.FamilyFont(run.Family, run.Style, runSize)
		if err != nil {
			return nil, err
		}
		formats[index] = runFormat{
			family: run.Family, style: run.Style, size: runSize, spacing: run.LetterSpacing * runSize / 100,
			font: font, shift: run.Shift * size / 100, link: run.Link, gray: run.Gray, image: run.Image,
		}
		switch {
		case mathml.Is(run.Image):
			if run.Style&document.Small != 0 {
				runSize = runSize * smallScale / 100
			}
			formula, mathSize, err := layout.faces.formula(run.Image, runSize, limit)
			if err != nil {
				log.Printf("layout: formula: %v", err)
				break
			}
			formats[index].imageSize = image.Pt(formula.Width, formula.Ascent+formula.Descent)
			formats[index].imageDescent, formats[index].mathSize = formula.Descent, mathSize
		case run.Image != "":
			formats[index].imageSize = layout.inlineImageSize(run, runSize, limit)
		}
		if run.Emphasis.Mark != 0 {
			markSize := max(font.Size()*emphasisScale/100, 1)
			markFont, err := layout.faces.Font(0, markSize)
			if err != nil {
				return nil, err
			}
			emphasis := run.Emphasis
			if !layout.faces.canDraw(markFont, emphasis.Mark) {
				// Few fonts have the sesame mark: a dot stands for any mark that no font draws.
				emphasis.Mark = '•'
				if run.Emphasis.Mark == '﹆' {
					emphasis.Mark = '◦'
				}
			}
			formats[index].marks = &emphasisMarks{Emphasis: emphasis, format: &runFormat{size: markSize, font: markFont}}
		}
		formats[index].space = formats[index].width(layout.faces, " ")
		pointers[index] = &formats[index]
	}
	return pointers, nil
}

// inlineImageSize returns the size of a run's inline image at size: ImageHeight high if the book
// sets it, else its own size, fitted in limit. Unlike an image block it is never enlarged: it sits
// in a line of text. An image whose size cannot be read takes no room.
func (layout *paginator) inlineImageSize(run document.Run, size int, limit image.Point) image.Point {
	natural, err := layout.sizer(run.Image)
	if err != nil || natural.X <= 0 || natural.Y <= 0 {
		log.Printf("layout: image %s: %v", run.Image, err)
		return image.Point{}
	}
	wanted := natural
	if run.ImageHeight > 0 {
		height := max(run.ImageHeight*size/100, 1)
		wanted = image.Pt(max(natural.X*height/natural.Y, 1), height)
	}
	return fitIn(wanted, limit, natural)
}

// emphasisScale is the size of emphasis marks, in percent of their text's size.
const emphasisScale = 50

// emphasisMarks are the marks a run puts on its characters, and the format they are drawn in.
type emphasisMarks struct {
	document.Emphasis
	format *runFormat
}

// runFormat is everything about a run that its fragments need to be measured, placed and drawn.
// The fragments of a run share one format: a pointer, as there may be thousands of them.
type runFormat struct {
	textFormat
	font      *fonts.Font
	shift     int // how far its baseline is raised, in pixels (a superscript)
	link      string
	gray      uint8
	image     string // the path of an inline image, "" for text
	imageSize image.Point
	// imageDescent is how far the image goes under the baseline. Only a formula does, as it has
	// a baseline of its own; mathSize is the font size it is laid out at.
	imageDescent, mathSize int
	marks                  *emphasisMarks // nil for a run without emphasis marks
	space                  int            // the width of a space, measured once for every gap
	// shapedFormats are the variants of the format for shaped text, left to right then right to
	// left, made when a fragment first needs one.
	shapedFormats *[2]*runFormat
}

// shapedFormat returns the variant of the format for text shaped in a direction and a language.
func (format *runFormat) shapedFormat(rightToLeft bool, language string) *runFormat {
	direction := 0
	if rightToLeft {
		direction = 1
	}
	if format.shapedFormats == nil {
		format.shapedFormats = &[2]*runFormat{}
	}
	if format.shapedFormats[direction] == nil {
		variant := *format
		variant.shaping = &Shaping{RightToLeft: rightToLeft, Language: language}
		variant.shapedFormats = nil
		format.shapedFormats[direction] = &variant
	}
	return format.shapedFormats[direction]
}

// width returns the width of text in the format. Text that its font covers, unshaped and without
// small capitals, is measured by the font directly, which is much quicker than through Faces.
func (format *runFormat) width(faces *Faces, text string) int {
	if format.style&document.SmallCaps != 0 || format.shaping != nil {
		return faces.width(format.textFormat, text)
	}
	width, covers := format.font.Measure(text)
	if !covers {
		return faces.width(format.textFormat, text)
	}
	if format.spacing != 0 {
		width += format.spacing * utf8.RuneCountInString(text)
	}
	return width
}

// blockText is what the scripts of a block's text need from line breaking and placement: where
// lines may break, the bidirectional level of each rune (from the block's start), and whether
// some of its text needs shaping. breaks and levels are nil for the text that does not need
// them, which is most text: Latin, Greek, Cyrillic.
type blockText struct {
	start  int // the offset of the block's first rune
	breaks []bool
	levels []int8
	shaped bool
	doc    *document.Document // for the languages of runs
}

// languageOf returns the language tag of run, which shaping needs.
func (text *blockText) languageOf(run document.Run) string {
	if run.Language == 0 || int(run.Language) > len(text.doc.Languages) {
		return text.doc.Language
	}
	return text.doc.Languages[run.Language-1]
}

// level returns the bidirectional level of the rune at offset: 0, left to right, without levels.
func (text *blockText) level(offset int) int8 {
	if text.levels == nil {
		return 0
	}
	return text.levels[offset-text.start]
}
