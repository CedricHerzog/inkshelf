package epub

import (
	"context"
	"errors"
	"fmt"
	"image"

	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/timing"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/svgimage"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
	"github.com/CedricHerzog/inkshelf/internal/web/webimage"
)

// A roll book (rendition:layout roll, EPUB 3.4) is a strip of images to scroll down through,
// as webtoons are: the images of its chapters one under the other, each as wide as the screen,
// with no gap. On a paged screen the strip is cut into pages a screen high, which RenderPage
// draws: a picture taller than the screen goes on over several pages, and a page may show the end
// of one picture and the start of the next.

// maxRollPages caps the pages of a roll: a strip that would make more is not drawn as a roll.
const maxRollPages = 10000

// stripImage is an image of a roll's strip: its path, where its top is, and its height, in
// pixels of a strip as wide as the screen.
type stripImage struct {
	path        string
	top, height int
}

// rollStrip is the strip of a roll laid out for a screen width, or why it cannot be.
type rollStrip struct {
	images []stripImage
	height int
	err    error
}

// errNotImages is why a roll book whose chapters are not all pictures is not drawn as a roll.
var errNotImages = errors.New("epub: a chapter of the roll is not only images")

// Roll reports whether the book is a roll of images (rendition:layout roll) that RenderPage
// draws as pages: a roll whose chapters hold anything but images is read as a reflowable book.
func (book *Book) Roll() bool {
	if book.Rendition.Layout != Roll || book.Chapters() == 0 {
		return false
	}
	return book.strip(renderedPageSize.X).err == nil
}

// DrawnAsPages reports whether the book is shown as pages that RenderPage draws, PageCount of
// them: a fixed-layout book or a roll.
func (book *Book) DrawnAsPages() bool { return book.FixedLayout() || book.Roll() }

// PageCount returns how many pages RenderPage draws for a screen of size: a fixed-layout
// book's chapters, or the screens a roll's strip fills.
func (book *Book) PageCount(size image.Point) int {
	if !book.Roll() || size.X <= 0 || size.Y <= 0 {
		return book.Chapters()
	}
	strip := book.strip(size.X)
	return min(max((strip.height+size.Y-1)/size.Y, 1), maxRollPages)
}

// strip returns the roll's strip laid out for a screen width wide, once per width: pages are
// drawn by several workers.
func (book *Book) strip(width int) rollStrip {
	book.stripsLock.Lock()
	defer book.stripsLock.Unlock()
	if strip, ok := book.strips[width]; ok {
		return strip
	}
	if book.strips == nil {
		book.strips = map[int]rollStrip{}
	}
	strip := book.layOutStrip(width)
	book.strips[width] = strip
	return strip
}

// layOutStrip lays the roll's images out one under the other, each fitted to width.
func (book *Book) layOutStrip(width int) rollStrip {
	span := timing.Start(context.Background(), "lay out roll")
	defer span.End()
	var strip rollStrip
	for index := range book.Chapters() {
		chapter, err := book.chapterWithoutFonts(index)
		if err != nil {
			return rollStrip{err: err}
		}
		paths, ok := chapterImages(chapter)
		if !ok {
			return rollStrip{err: errNotImages}
		}
		for _, imagePath := range paths {
			size, err := book.ImageSize(imagePath)
			if err != nil || size.X <= 0 || size.Y <= 0 {
				continue // an image that cannot be read is left out, as on a page
			}
			height := max(int((int64(size.Y)*int64(width)+int64(size.X)/2)/int64(size.X)), 1)
			if strip.height+height > maxRollPages*renderedPageSize.Y {
				return rollStrip{err: fmt.Errorf("epub: a roll over %d pages", maxRollPages)}
			}
			strip.images = append(strip.images, stripImage{path: imagePath, top: strip.height, height: height})
			strip.height += height
		}
	}
	return strip
}

// chapterImages returns the paths of a chapter's images, in order, and false if it has
// anything else to show: text, a rule, a table. A chapter that shows nothing is fine.
func chapterImages(chapter *document.Document) ([]string, bool) {
	var paths []string
	for _, block := range chapter.Blocks {
		switch block.Kind {
		case document.Image:
			paths = append(paths, block.Image)
		case document.Rule:
			return nil, false
		default:
			for _, run := range block.Runs {
				switch {
				case run.Image != "":
					paths = append(paths, run.Image)
				case run.Text != "" && !onlySpaces(run.Text):
					return nil, false
				}
			}
		}
	}
	return paths, true
}

// onlySpaces reports whether text is only whitespace.
func onlySpaces(text string) bool {
	for _, character := range text {
		if character != ' ' && character != '\n' && character != '\t' && character != ' ' {
			return false
		}
	}
	return true
}

// renderRollPage draws page index of the roll for a screen of size: the part of the strip a
// screen high from index screens down, white past its end.
func (book *Book) renderRollPage(index int, size image.Point) (*image.Gray, error) {
	if index < 0 || index >= book.PageCount(size) {
		return nil, fmt.Errorf("page %d of %d", index, book.PageCount(size))
	}
	strip := book.strip(size.X)
	canvas := image.NewGray(image.Rectangle{Max: size})
	for offset := range canvas.Pix {
		canvas.Pix[offset] = 0xff
	}
	top := index * size.Y
	for _, placed := range strip.images {
		if placed.top+placed.height <= top || placed.top >= top+size.Y {
			continue
		}
		picture, err := book.stripPicture(placed.path, image.Pt(size.X, placed.height))
		if err != nil {
			continue // left out, as an image that fails on a page
		}
		pasteGray(canvas, picture, image.Pt(0, placed.top-top))
	}
	return canvas, nil
}

// stripPicture returns an image of the book prepared at size: a picture, or an SVG drawn.
func (book *Book) stripPicture(imagePath string, size image.Point) (*image.Gray, error) {
	data, err := book.Resource(imagePath)
	if uri.IsData(imagePath) {
		data, err = uri.DecodeData(imagePath)
	}
	if err != nil {
		return nil, err
	}
	if svgimage.IsSVG(data) {
		return webimage.Prepare(data, size, svgimage.Options{Resolve: func(href string) ([]byte, error) {
			return book.Resource(uri.Resolve(imagePath, href))
		}})
	}
	decoded, err := pageimage.DecodeGray(data)
	if err != nil {
		return nil, err
	}
	return pageimage.Fit(decoded, size.X, size.Y), nil
}

// pasteGray copies src into dst with its top left at topLeft, clipped to dst.
func pasteGray(dst, src *image.Gray, topLeft image.Point) {
	bounds := src.Bounds()
	target := bounds.Sub(bounds.Min).Add(topLeft).Intersect(dst.Bounds())
	for row := target.Min.Y; row < target.Max.Y; row++ {
		from := src.PixOffset(bounds.Min.X+target.Min.X-topLeft.X, bounds.Min.Y+row-topLeft.Y)
		copy(dst.Pix[dst.PixOffset(target.Min.X, row):dst.PixOffset(target.Max.X, row)], src.Pix[from:])
	}
}
