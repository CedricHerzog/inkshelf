package epub

import (
	"context"
	"image"
	"image/draw"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/container"
	"github.com/CedricHerzog/inkshelf/internal/timing"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
)

// Spread is two consecutive chapters shown side by side, as the two pages of an open book: by
// their chapter indexes (counted from 0), the one read first, and the left and right pages.
// Left is First when pages turn from left to right.
type Spread struct {
	First int `json:"first"`
	Left  int `json:"left"`
	Right int `json:"right"`
}

// Spreads returns the spreads of a fixed-layout book, in reading order, as EPUB 3.3 pairs
// pages: in print, the first page of a book is alone, on the side opposite its binding (the
// right in a book read from left to right), and the pages after it alternate. A page pinned to
// a side by page-spread-left or -right takes that side, and the next ones alternate from it.
// A page stands alone when it is centred (page-spread-center), meant for portrait
// (rendition:orientation), not pre-paginated, or in a part of the book that has no spreads
// (rendition:spread none): it takes a whole spread, and the next page starts a new one, on the
// side reading starts from. Two pages pair when the first is on that side and the second on
// the other.
func (book *Book) Spreads() []Spread {
	firstSide, startSide := container.PageSpreadRight, container.PageSpreadLeft
	if book.RightToLeft() {
		firstSide, startSide = container.PageSpreadLeft, container.PageSpreadRight
	}
	sides := make([]string, book.Chapters()) // "" for a page that stands alone
	next := firstSide
	for index := range sides {
		rendition := book.RenditionOf(book.ChapterItem(index))
		if rendition.Layout != container.PrePaginated || rendition.Spread == container.SpreadNone ||
			rendition.Orientation == container.OrientationPortrait || rendition.PageSpread == container.PageSpreadCenter {
			next = startSide
			continue
		}
		side := next
		if rendition.PageSpread == container.PageSpreadLeft || rendition.PageSpread == container.PageSpreadRight {
			side = rendition.PageSpread
		}
		sides[index] = side
		next = otherSide(side)
	}
	return pairSides(sides, startSide)
}

// RenderSpread draws the two pages of spread on a white canvas of size (a screen in landscape),
// each fitted in its half and set against the gutter between them, as an open book shows them.
// The two pages share one page's budget for their elements: they show as one.
func (book *Book) RenderSpread(spread Spread, size image.Point, elements layout.Elements) (*image.Gray, error) {
	defer timing.Start(context.Background(), "render spread").End()
	half := image.Pt(size.X/2, size.Y)
	canvas := image.NewGray(image.Rectangle{Max: size})
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	elements = elements.Started()
	for _, chapter := range []int{spread.Left, spread.Right} {
		drawn, err := book.RenderPage(chapter, half, elements)
		if err != nil {
			return nil, err
		}
		pageSize := drawn.Bounds().Size()
		topLeft := image.Pt(half.X, (half.Y-pageSize.Y)/2)
		if chapter == spread.Left {
			topLeft.X -= pageSize.X
		}
		draw.Draw(canvas, image.Rectangle{Min: topLeft, Max: topLeft.Add(pageSize)}, drawn, drawn.Bounds().Min, draw.Src)
	}
	return canvas, nil
}

// pairSides returns the spreads of consecutive chapters whose sides (left, right, or "" for
// none) are startSide, the side reading starts from, then the other.
func pairSides(sides []string, startSide string) []Spread {
	var spreads []Spread
	for index := 0; index+1 < len(sides); index++ {
		if sides[index] != startSide || sides[index+1] != otherSide(startSide) {
			continue
		}
		spread := Spread{First: index, Left: index, Right: index + 1}
		if startSide == container.PageSpreadRight {
			spread.Left, spread.Right = index+1, index
		}
		spreads = append(spreads, spread)
		index++ // the second page is taken
	}
	return spreads
}

// otherSide returns the side of a spread opposite side.
func otherSide(side string) string {
	if side == container.PageSpreadLeft {
		return container.PageSpreadRight
	}
	return container.PageSpreadLeft
}
