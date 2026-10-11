package layout

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/CedricHerzog/inkshelf/internal/web/mathml"
	"github.com/CedricHerzog/inkshelf/internal/web/webimage"
)

// The grays layout draws with: formulas in black, rules in dark gray.
var (
	black    = color.Gray{Y: 0x00}
	darkGray = color.Gray{Y: 0x55}
)

// Draw draws page into dst, which is white, from the bottom layer up: the border boxes'
// backgrounds and borders, underlines and strike-throughs, rules in dark gray, images through
// images (one that cannot be loaded is left out), and the words in their gray.
func (page Page) Draw(dst *image.Gray, faces *Faces, images webimage.Loader) error {
	drawBorderBoxes(dst, page.Boxes)
	for _, stroke := range page.Strokes {
		fill(dst, stroke.Rectangle, stroke.Gray)
	}
	for _, rule := range page.Rules {
		draw.Draw(dst, rule, image.NewUniform(darkGray), image.Point{}, draw.Src)
	}
	for _, placed := range page.Images {
		if mathml.Is(placed.Path) {
			if formula, _, err := faces.formula(placed.Path, placed.MathSize, image.Pt(math.MaxInt32, math.MaxInt32)); err == nil {
				formula.Draw(dst, image.Pt(placed.Rect.Min.X, placed.Rect.Min.Y+formula.Ascent))
			}
			continue
		}
		picture, err := images(placed.Path, placed.Rect.Size())
		if err != nil {
			continue
		}
		// The image was fitted in its area and may be a pixel smaller: centre it.
		offset := placed.Rect.Size().Sub(picture.Bounds().Size()).Div(2)
		draw.Draw(dst, picture.Bounds().Sub(picture.Bounds().Min).Add(placed.Rect.Min.Add(offset)), picture, picture.Bounds().Min, draw.Src)
	}
	for _, line := range page.Lines {
		for _, word := range line.Words {
			if err := faces.draw(dst, word.format(), image.Pt(word.X, word.Top), word.Text, color.Gray{Y: word.Gray}); err != nil {
				return err
			}
		}
	}
	return nil
}
