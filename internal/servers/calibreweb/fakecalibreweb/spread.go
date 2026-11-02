package fakecalibreweb

import (
	"image"
	"image/color"
	"image/draw"
)

// spreadPictureSize is the picture across two pages of the spread book, both halves together.
var spreadPictureSize = image.Pt(1600, 1200)

// spreadChapters are the chapters of the spread book: text, a picture across two pages as two
// chapters of one image each, pinned to the left and the right page, then text again.
var spreadChapters = []showcaseChapter{
	{file: "before.xhtml", title: "Avant", language: "fr", body: `<h1>Avant</h1>
<p>Zoé monte l'escalier du phare. La page suivante montre la mer, d'un bord à l'autre du livre ouvert.</p>`},
	{file: "left.xhtml", title: "La mer, à gauche", language: "fr", properties: "page-spread-left", body: `<div><img src="left.jpg" alt="La mer, à gauche"/></div>`},
	{file: "right.xhtml", title: "La mer, à droite", language: "fr", properties: "page-spread-right", body: `<div><img src="right.jpg" alt="La mer, à droite"/></div>`},
	{file: "after.xhtml", title: "Après", language: "fr", body: `<h1>Après</h1>
<p>Le soir tombe, et la lumière du phare commence à tourner.</p>`},
}

// makeSpreadEPUB returns the EPUB of the spread book: a reflowable book whose two middle
// chapters are the halves of one picture, the sea under a band of sky, with its title across
// the middle.
func makeSpreadEPUB(item book) ([]byte, error) {
	picture := image.NewRGBA(image.Rectangle{Max: spreadPictureSize})
	for row := range spreadPictureSize.Y {
		shade := uint8(230 - row*150/spreadPictureSize.Y)
		draw.Draw(picture, image.Rect(0, row, spreadPictureSize.X, row+1), image.NewUniform(color.Gray{Y: shade}), image.Point{}, draw.Src)
	}
	band := image.Rect(0, 500, spreadPictureSize.X, 700)
	draw.Draw(picture, band, image.White, image.Point{}, draw.Src)
	if err := drawLines(picture, band, 90, []string{"La mer, d'un bord à l'autre"}); err != nil {
		return nil, err
	}
	half := spreadPictureSize.X / 2
	left, err := encodeJPEG(picture.SubImage(image.Rect(0, 0, half, spreadPictureSize.Y)))
	if err != nil {
		return nil, err
	}
	right, err := encodeJPEG(picture.SubImage(image.Rect(half, 0, spreadPictureSize.X, spreadPictureSize.Y)))
	if err != nil {
		return nil, err
	}
	return chaptersEPUB(item, spreadChapters, showcaseCSS, "fr", map[string][]byte{"left.jpg": left, "right.jpg": right})
}
