package main

import (
	"fmt"
	"image"
	"image/draw"
	"io"
	"path/filepath"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
	"github.com/CedricHerzog/inkshelf/internal/web/webimage"
)

// A test of the suite checks one feature on a few pages; a real book checks them all together,
// on every page. readWholeBooks lays out every chapter of real books and draws every page, as
// reading them through would, and reports what a reader would see wrong: a character drawn as
// the missing-glyph box, text that no page shows, and pages that turn the wrong way.

// readWholeBooks reads each book of paths whole and writes what it found to report, a book that
// cannot be read included. The books stay where they are: nothing of theirs is copied or kept.
func readWholeBooks(paths []string, report io.Writer) {
	for _, path := range paths {
		if err := readWholeBook(path, report); err != nil {
			fmt.Fprintf(report, "%s: %v\n", filepath.Base(path), err)
		}
	}
}

// readWholeBook reads the book at path whole, and reports on it in a paragraph.
func readWholeBook(path string, report io.Writer) error {
	book, err := epub.Open(path)
	if err != nil {
		return err
	}
	defer book.Close()
	index, err := epub.BuildIndex(book)
	if err != nil {
		return err
	}
	// Footnotes stay in the text, so that every rune of a chapter is on a page.
	settings := readerSettings()
	settings.HideFootnotes = false
	faces := layout.NewFaces(fonts.Serif)
	defer faces.Close()
	images := webimage.NewLoader(book.Resource, faces.SVGText())

	pageCount, leftOut := 0, 0
	missing := map[rune]int{} // by character, the pages it is missing on
	var drawErrors []string
	for chapter := range book.Chapters() {
		doc, err := book.Chapter(chapter)
		if err != nil {
			drawErrors = append(drawErrors, fmt.Sprintf("chapter %d: %v", chapter, err))
			continue
		}
		pages, err := layout.Paginate(doc, settings, faces, book.ImageSize)
		if err != nil {
			drawErrors = append(drawErrors, fmt.Sprintf("chapter %d: %v", chapter, err))
			continue
		}
		leftOut += doc.Chars - covered(pages, doc.Chars)
		for _, page := range pages {
			pageCount++
			for _, character := range faces.MissingCharacters(page) {
				missing[character]++
			}
			canvas := whiteCanvas(settings)
			if err := page.Draw(canvas, faces, images, layout.Elements{}); err != nil {
				drawErrors = append(drawErrors, fmt.Sprintf("chapter %d: %v", chapter, err))
			}
		}
	}

	direction := "left to right"
	if index.RightToLeft() {
		direction = "right to left"
	}
	spine := book.PageProgression
	if spine == "" {
		spine = "not given"
	}
	fmt.Fprintf(report, "%s: %d chapters, %d pages, %d runes; pages turn %s (spine: %s, language %q)\n",
		filepath.Base(path), book.Chapters(), pageCount, index.Total, direction, spine, index.Language)
	fmt.Fprintf(report, "  characters drawn as a missing glyph: %d", len(missing))
	for character, pages := range missing {
		fmt.Fprintf(report, " %q (U+%04X, %d pages)", character, character, pages)
	}
	fmt.Fprintf(report, "\n  runes on no page: %d\n", leftOut)
	for _, drawError := range drawErrors {
		fmt.Fprintf(report, "  error: %s\n", drawError)
	}
	return nil
}

// covered returns how many of a chapter's chars runes its pages show: the union of their
// ranges, pages being in order.
func covered(pages []layout.Page, chars int) int {
	shown, reached := 0, 0
	for _, page := range pages {
		start, end := max(page.Start, reached), min(page.End, chars)
		if end > start {
			shown += end - start
		}
		reached = max(reached, end)
	}
	return shown
}

// whiteCanvas returns a white canvas of settings' page.
func whiteCanvas(settings layout.Settings) *image.Gray {
	canvas := image.NewGray(image.Rect(0, 0, settings.Width, settings.Height))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	return canvas
}
