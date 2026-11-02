package layout

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/web/mathml"
	"github.com/CedricHerzog/inkshelf/internal/web/webimage"
)

// The grays layout draws with: formulas in black, rules in dark gray.
var (
	black    = color.Gray{Y: 0x00}
	darkGray = color.Gray{Y: 0x55}
)

// errNotReady is the error of an element that was not ready within its page's budget.
var errNotReady = errors.New("not ready in time")

// Draw draws page into dst, which is white, from the bottom layer up: the border boxes'
// backgrounds (their images through images) and borders, underlines and strike-throughs, rules
// in dark gray, images through images (one that cannot be loaded is left out), and the words in
// their gray. Images and formulas are prepared through elements: one not ready within the page's
// budget shows its text alternative in its box instead.
func (page Page) Draw(dst *image.Gray, faces *Faces, images webimage.Loader, elements Elements) error {
	elements = elements.Started()
	drawBorderBoxes(dst, page.Boxes, func(path string, size image.Point) (Tile, error) {
		picture, err := loadImage(elements, images, path, size)
		return Tile{Gray: picture}, err
	})
	for _, stroke := range page.Strokes {
		fill(dst, stroke.Rectangle, stroke.Gray)
	}
	for _, rule := range page.Rules {
		draw.Draw(dst, rule, image.NewUniform(darkGray), image.Point{}, draw.Src)
	}
	for _, placed := range page.Images {
		if mathml.Is(placed.Path) {
			drawFormula(dst, faces, elements, placed)
			continue
		}
		picture, err := loadImage(elements, images, placed.Path, placed.Rect.Size())
		if err != nil {
			if errors.Is(err, errNotReady) {
				log.Printf("layout: image %.80s: %v", placed.Path, err)
				drawAlt(dst, faces, placed)
			}
			continue
		}
		// The image was fitted in its area and may be a pixel smaller: centre it.
		offset := placed.Rect.Size().Sub(picture.Bounds().Size()).Div(2)
		draw.Draw(dst, picture.Bounds().Sub(picture.Bounds().Min).Add(placed.Rect.Min.Add(offset)), picture, picture.Bounds().Min, draw.Src)
	}
	for _, line := range page.Lines {
		for _, word := range line.Words {
			if word.Mark != NoMark {
				drawMark(dst, word)
				continue
			}
			if err := faces.draw(dst, word.format(), image.Pt(word.X, word.Top), word.Text, color.Gray{Y: word.Gray}); err != nil {
				return err
			}
		}
	}
	return nil
}

// loadImage loads the image at path fitted in size through elements, errNotReady if it was not
// ready in time.
func loadImage(elements Elements, images webimage.Loader, path string, size image.Point) (*image.Gray, error) {
	type loaded struct {
		picture *image.Gray
		err     error
	}
	result, ok := PrepareElement(elements, "prepare image", path, size, func() loaded {
		picture, err := images(path, size)
		return loaded{picture: picture, err: err}
	})
	if !ok {
		return nil, errNotReady
	}
	return result.picture, result.err
}

// drawFormula lays out and draws a formula through elements, with the element faces, or its
// text alternative if it was not ready in time or could not be laid out. The formula is drawn
// on a white canvas of its own, a margin around its box for the ink that overhangs it, then
// darkens dst under it: the worker never touches dst, which the page may have moved on from.
func drawFormula(dst *image.Gray, faces *Faces, elements Elements, placed PlacedImage) {
	type drawn struct {
		canvas *image.Gray
		err    error
	}
	margin := placed.MathSize
	result, ok := PrepareElement(elements, "lay out formula", placed.Path, placed.Rect.Size(), func() (result drawn) {
		faces.elements.with(func(elementFaces *Faces) {
			formula, _, err := elementFaces.formula(placed.Path, placed.MathSize, image.Pt(math.MaxInt32, math.MaxInt32))
			if err != nil {
				result.err = err
				return
			}
			canvas := image.NewGray(placed.Rect.Inset(-margin))
			draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
			formula.Draw(canvas, image.Pt(placed.Rect.Min.X, placed.Rect.Min.Y+formula.Ascent))
			result.canvas = canvas
		})
		return result
	})
	if !ok {
		result.err = errNotReady
	}
	if result.err != nil {
		log.Printf("layout: formula: %v", result.err)
		drawAlt(dst, faces, placed)
		return
	}
	darken(dst, result.canvas)
}

// darken draws src over dst where src is darker: ink drawn on white, over what dst has there.
func darken(dst, src *image.Gray) {
	area := src.Bounds().Intersect(dst.Bounds())
	for y := area.Min.Y; y < area.Max.Y; y++ {
		dstRow := dst.Pix[dst.PixOffset(area.Min.X, y):dst.PixOffset(area.Max.X, y)]
		srcRow := src.Pix[src.PixOffset(area.Min.X, y):src.PixOffset(area.Max.X, y)]
		for index, gray := range srcRow {
			dstRow[index] = min(dstRow[index], gray)
		}
	}
}

// drawAlt draws the text alternative of an image in its box, in lines broken between words,
// in the format and gray of the text around it; what does not fit in the box is cut off.
func drawAlt(dst *image.Gray, faces *Faces, placed PlacedImage) {
	format := placed.altFormat
	if placed.Alt == "" || format.size <= 0 {
		return
	}
	area, ok := dst.SubImage(placed.Rect).(*image.Gray)
	if !ok || area.Bounds().Empty() {
		return
	}
	font, err := faces.FamilyFont(format.family, format.style, format.size)
	if err != nil {
		return
	}
	lineHeight := max(font.Ascent()+font.Descent(), 1)
	top := placed.Rect.Min.Y
	for _, line := range wrapWords(placed.Alt, placed.Rect.Dx(), func(text string) int { return faces.width(format, text) }) {
		if top >= placed.Rect.Max.Y {
			return
		}
		_ = faces.draw(area, format, image.Pt(placed.Rect.Min.X, top), line, color.Gray{Y: placed.altGray})
		top += lineHeight
	}
}

// wrapWords breaks text into lines no wider than width, as measured by widthOf, between its
// words: a word wider than width has a line of its own.
func wrapWords(text string, width int, widthOf func(string) int) []string {
	var lines []string
	line := ""
	for word := range strings.FieldsSeq(text) {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if line != "" && widthOf(candidate) > width {
			lines = append(lines, line)
			candidate = word
		}
		line = candidate
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// altFormatOf returns the format an inline image's text alternative is drawn in: the text's
// family, style and size, in horizontal lines with no letter spacing.
func altFormatOf(format textFormat) textFormat {
	return textFormat{family: format.family, style: format.style, size: format.size}
}
