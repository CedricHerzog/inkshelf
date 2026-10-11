package fakekomga

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// pageSize is the size of the generated pages, about that of a scanned manga page.
var pageSize = image.Pt(1200, 1800)

// coverSize is the size of the thumbnails, as Komga makes them.
var coverSize = image.Pt(300, 450)

// jpegQuality is the quality of the JPEG pages and covers.
const jpegQuality = 85

// encodedPage returns page number of book of series item, encoded in format ("jpeg" or
// "png"). Each page is drawn and encoded once, then kept.
func (fake *server) encodedPage(item series, book, number int, format string) ([]byte, error) {
	key := fmt.Sprintf("%s/%d.%s", bookID(item.id, book), number, format)
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if data, ok := fake.images[key]; ok {
		return data, nil
	}
	img := fake.drawPageLocked(item, book, number)
	var buffer bytes.Buffer
	var err error
	if format == "jpeg" {
		err = jpeg.Encode(&buffer, img, &jpeg.Options{Quality: jpegQuality})
	} else {
		err = png.Encode(&buffer, img)
	}
	if err != nil {
		return nil, err
	}
	fake.images[key] = buffer.Bytes()
	return buffer.Bytes(), nil
}

// encodedCover returns the cover of book (0: of the series) of series item, as a JPEG. Each
// cover is drawn and encoded once, then kept.
func (fake *server) encodedCover(item series, book int) ([]byte, error) {
	key := fmt.Sprintf("%s/cover/%d", item.id, book)
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if data, ok := fake.images[key]; ok {
		return data, nil
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, fake.drawCoverLocked(item, book), &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	fake.images[key] = buffer.Bytes()
	return buffer.Bytes(), nil
}

// drawPageLocked draws a page that looks like manga: three panels, the page number big in the
// first, the series and book in the second, and a gray ramp in the last to show the dithering.
// The caller holds fake.mu, which guards the font face.
func (fake *server) drawPageLocked(item series, book, number int) image.Image {
	const (
		margin     = 60 // around the panels
		gutter     = 30 // between two panels
		border     = 6  // the width of a panel's black frame
		rampMargin = 20 // between the frame and the gray ramp
	)
	img := image.NewRGBA(image.Rectangle{Max: pageSize})
	draw.Draw(img, img.Rect, image.White, image.Point{}, draw.Src)
	inner := img.Rect.Inset(margin)
	thirds := (inner.Dy() - 2*gutter) / 3
	panels := []image.Rectangle{
		image.Rect(inner.Min.X, inner.Min.Y, inner.Max.X, inner.Min.Y+thirds),
		image.Rect(inner.Min.X, inner.Min.Y+thirds+gutter, inner.Max.X, inner.Min.Y+2*thirds+gutter),
		image.Rect(inner.Min.X, inner.Min.Y+2*(thirds+gutter), inner.Max.X, inner.Max.Y),
	}
	for _, panel := range panels {
		draw.Draw(img, panel, image.Black, image.Point{}, draw.Src)
		draw.Draw(img, panel.Inset(border), image.White, image.Point{}, draw.Src)
	}

	// The ramp goes from black on the left to white on the right, one gray per column.
	ramp := panels[2].Inset(border + rampMargin)
	for offset := range ramp.Dx() {
		column := ramp.Min.X + offset
		gray := color.Gray{Y: uint8(255 * offset / (ramp.Dx() - 1))}
		draw.Draw(img, image.Rect(column, ramp.Min.Y, column+1, ramp.Max.Y), image.NewUniform(gray), image.Point{}, draw.Src)
	}

	drawCentered(img, fake.pageFace, panels[0], strconv.Itoa(number))
	drawCentered(img, fake.pageFace, panels[1], fmt.Sprintf("%s %d", item.id, book))
	return img
}

// drawCoverLocked draws a cover: diagonal stripes, darker or lighter for each series, and a
// white band with the series' id and the book's number. The caller holds fake.mu, which
// guards the font face.
func (fake *server) drawCoverLocked(item series, book int) image.Image {
	const (
		darkestShade = 40 // the dark stripes of the first series
		shadeStep    = 50 // how much lighter the next series' stripes are
		shadeCount   = 5  // after shadeCount series, the shades start over
		stripeWidth  = 24 // in pixels, measured along the rows
		stripeLift   = 40 // how much lighter the light stripes are
		bandMargin   = 20 // between the band and the cover's sides
		bandTop      = 150
		bandBottom   = 300
	)
	img := image.NewRGBA(image.Rectangle{Max: coverSize})
	seriesIndex := slices.IndexFunc(catalog, func(other series) bool { return other.id == item.id })
	shade := darkestShade + shadeStep*(seriesIndex%shadeCount)
	for row := range coverSize.Y {
		for column := range coverSize.X {
			gray := uint8(shade)
			if (row+column)/stripeWidth%2 == 0 {
				gray += stripeLift
			}
			img.Set(column, row, color.Gray{Y: gray})
		}
	}
	band := image.Rect(bandMargin, bandTop, coverSize.X-bandMargin, bandBottom)
	draw.Draw(img, band, image.White, image.Point{}, draw.Src)
	label := strings.ToUpper(item.id)
	if book > 0 {
		label = fmt.Sprintf("%s %d", label, book)
	}
	drawCentered(img, fake.coverFace, band, label)
	return img
}

// drawCentered draws text in black with face in the middle of rect.
func drawCentered(img *image.RGBA, face font.Face, rect image.Rectangle, text string) {
	drawer := font.Drawer{Dst: img, Src: image.Black, Face: face}
	width := drawer.MeasureString(text).Ceil()
	metrics := face.Metrics()
	height := (metrics.Ascent + metrics.Descent).Ceil()
	center := rect.Min.Add(rect.Size().Div(2))
	drawer.Dot = fixed.P(center.X-width/2, center.Y-height/2+metrics.Ascent.Ceil())
	drawer.DrawString(text)
}
