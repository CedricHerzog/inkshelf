package reading

import (
	"context"
	"errors"
	"image"
	"image/draw"
	"log"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/timing"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
	"github.com/CedricHerzog/inkshelf/internal/web/webimage"
)

// A reflowable book may hold a picture across two pages: two chapters that are each one image,
// pinned to the left and the right page (epub.Index.Spreads). They read as one page, the
// spread, shown in landscape: its position is the first chapter's start, and a turn moves past
// both chapters.

// spreadOf returns the spread chapter is in, and whether it is in one.
func (text *TextBook) spreadOf(chapter int) (epub.Spread, bool) {
	for _, spread := range text.Index.Spreads {
		if chapter == spread.Left || chapter == spread.Right {
			return spread, true
		}
	}
	return epub.Spread{}, false
}

// secondChapter returns the chapter of spread read second.
func secondChapter(spread epub.Spread) int { return spread.Left + spread.Right - spread.First }

// AtSpread reports whether the page shown is a spread.
func (text *TextBook) AtSpread() bool {
	spread, ok := text.spreadOf(text.at.Chapter)
	return ok && spread.First == text.at.Chapter
}

// errNoImage is the error of a spread chapter whose document is not one image.
var errNoImage = errors.New("a spread's page holds no image")

// drawSpread draws spread on a white canvas of the screen turned to landscape: each chapter's
// image fitted in its half, set against the gutter between them.
func (renderer *renderer) drawSpread(spread epub.Spread) (*image.Gray, error) {
	renderer.mu.Lock()
	defer renderer.mu.Unlock()
	if renderer.book == nil {
		return nil, errClosed
	}
	span := timing.Start(context.Background(), "render spread")
	landscape := image.Pt(renderer.settings.Height, renderer.settings.Width)
	half := image.Pt(landscape.X/2, landscape.Y)
	canvas := image.NewGray(image.Rectangle{Max: landscape})
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	images := webimage.NewLoader(renderer.book.Resource, renderer.faces.SVGText())
	elements := renderer.settings.Elements
	elements.Book = renderer.bookID
	elements = elements.Started()
	for _, chapter := range []int{spread.Left, spread.Right} {
		doc, err := renderer.book.Chapter(chapter)
		if err != nil {
			return nil, err
		}
		if len(doc.Blocks) == 0 || doc.Blocks[0].Image == "" {
			return nil, errNoImage
		}
		imagePath := doc.Blocks[0].Image
		type loaded struct {
			picture *image.Gray
			err     error
		}
		result, ok := layout.PrepareElement(elements, "prepare image", imagePath, half, func() loaded {
			picture, err := images(imagePath, half)
			return loaded{picture: picture, err: err}
		})
		if !ok || result.err != nil {
			log.Printf("spread of chapter %d: image %.80s left out: %v", spread.First, imagePath, result.err)
			continue // the other half shows
		}
		size := result.picture.Bounds().Size()
		topLeft := image.Pt(half.X, (half.Y-size.Y)/2)
		if chapter == spread.Left {
			topLeft.X -= size.X
		}
		draw.Draw(canvas, image.Rectangle{Min: topLeft, Max: topLeft.Add(size)}, result.picture, result.picture.Bounds().Min, draw.Src)
	}
	log.Printf("render spread of chapters %d and %d: %v", spread.Left, spread.Right, span.End())
	return canvas, nil
}
