package fixedlayout

import (
	"errors"
	"image"
	"image/draw"
	"math"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
	"github.com/CedricHerzog/inkshelf/internal/web/svgimage"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
	"github.com/CedricHerzog/inkshelf/internal/web/webimage"
)

// DefaultSize is the size of a page that gives none, in CSS pixels: a 6" reader's, the page
// that package epub reads reflowable chapters for.
var DefaultSize = image.Pt(600, 800)

// lineSpacing is the height of a line of a region's text, in percent of its font size: CSS's
// line-height: normal, about.
const lineSpacing = 120

// Page is a page of a fixed-layout book: its initial containing block, and the regions drawn
// in it. Lengths are in CSS pixels.
type Page struct {
	Size image.Point
	// Regions are drawn in their order: the page's own content first, then each positioned
	// element of its XHTML.
	Regions []*Region
	// EmPixels is the font size of the regions' documents, in CSS pixels: what their lengths in
	// hundredths of an em are relative to.
	EmPixels int
}

// Region is an area of a page: a positioned element, or the page itself.
type Region struct {
	Rect image.Rectangle // in the page
	// Color fills Rect (transparent for none), then Gradient (nil for none), then Background,
	// the path of an image stretched over it ("" for none). Backgrounds are drawn under every
	// region's content.
	Color      css.Color
	Gradient   *css.Gradient
	Background string
	// Content is laid out in Rect from its top, nil for none. A lone image is fitted in Rect,
	// centred.
	Content *document.Document
}

// Files are what a page's paths name: the files of its book, as internal/ebook/epub's Book reads them.
type Files interface {
	Resource(path string) ([]byte, error)
	ImageSize(path string) (image.Point, error)
}

// Render draws page on white, scaled to fit in size keeping its aspect ratio: the backgrounds
// of its regions, then their content, clipped to the page. An image that cannot be drawn is
// left out.
func (page *Page) Render(size image.Point, files Files) (*image.Gray, error) {
	if page.Size.X <= 0 || page.Size.Y <= 0 || size.X <= 0 || size.Y <= 0 {
		return nil, errors.New("fixedlayout: empty page")
	}
	scale := page.scale(size)
	canvas := image.NewGray(image.Rectangle{Max: page.FittedSize(size)})
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	faces := layout.NewFaces(fonts.Serif)
	defer faces.Close()
	for _, region := range page.Regions {
		area := scaledRect(region.Rect, scale)
		fillColor(canvas, area, region.Color)
		if region.Gradient != nil {
			fillGradient(canvas, area, region.Gradient)
		}
		if region.Background != "" {
			_ = drawImage(canvas, region.Background, area, files, faces, false) // left out if it fails
		}
	}
	for _, region := range page.Regions {
		if region.Content == nil {
			continue
		}
		area := scaledRect(region.Rect, scale)
		if area.Empty() {
			continue
		}
		if blocks := region.Content.Blocks; len(blocks) == 1 && blocks[0].Kind == document.Image {
			_ = drawImage(canvas, blocks[0].Image, area, files, faces, true) // left out if it fails
			continue
		}
		// The text's first page, laid out in area: what overflows its bottom is left out.
		settings := layout.Settings{
			Width: area.Max.X, Height: area.Max.Y, Margins: image.Rectangle{Min: area.Min},
			FontSize: max(int(math.Round(float64(page.EmPixels)*scale)), 1), LineSpacing: lineSpacing,
		}
		pages, err := layout.Paginate(region.Content, settings, faces, files.ImageSize)
		if err != nil {
			return nil, err
		}
		if err := pages[0].Draw(canvas, faces, webimage.NewLoader(files.Resource, faces.SVGText())); err != nil {
			return nil, err
		}
	}
	return canvas, nil
}

// FittedSize returns the size of the page fitted in size, keeping its aspect ratio: what
// Render draws.
func (page *Page) FittedSize(size image.Point) image.Point {
	return scaled(page.Size, page.scale(size))
}

// scale returns how much the page is enlarged to fit in size.
func (page *Page) scale(size image.Point) float64 {
	return min(float64(size.X)/float64(page.Size.X), float64(size.Y)/float64(page.Size.Y))
}

// scaled returns size scaled, rounded.
func scaled(size image.Point, scale float64) image.Point {
	return image.Pt(int(math.Round(float64(size.X)*scale)), int(math.Round(float64(size.Y)*scale)))
}

// scaledRect returns rect scaled, rounded.
func scaledRect(rect image.Rectangle, scale float64) image.Rectangle {
	return image.Rectangle{Min: scaled(rect.Min, scale), Max: scaled(rect.Max, scale)}
}

// drawImage draws the image at imagePath (a data: URI too) in area of canvas: fitted and
// centred in it if fitted, else stretched over it, its transparent parts showing what is under
// it.
func drawImage(canvas *image.Gray, imagePath string, area image.Rectangle, files Files, faces *layout.Faces, fitted bool) error {
	if area.Empty() {
		return nil
	}
	var data []byte
	var err error
	if uri.IsData(imagePath) {
		data, err = uri.DecodeData(imagePath)
	} else {
		data, err = files.Resource(imagePath)
	}
	if err != nil {
		return err
	}
	var picture *image.Gray
	switch {
	case !fitted && !svgimage.IsSVG(data) && drawTransparent(canvas, data, area):
		return nil // composited over what is under it
	case svgimage.IsSVG(data):
		if picture, err = rasterize(data, imagePath, area.Size(), files, faces, fitted); err != nil {
			return err
		}
	default:
		decoded, err := pageimage.DecodeGray(data)
		if err != nil {
			return err
		}
		picture = pageimage.Fit(decoded, area.Dx(), area.Dy())
		if !fitted {
			picture = stretch(picture, area.Size())
		}
	}
	paste(canvas, picture, area.Min.Add(area.Size().Sub(picture.Bounds().Size()).Div(2)))
	return nil
}

// paste copies src into dst with its top-left corner at topLeft, clipped to dst: row by row,
// as image/draw has no fast path for gray images.
func paste(dst, src *image.Gray, topLeft image.Point) {
	bounds := src.Bounds()
	target := bounds.Sub(bounds.Min).Add(topLeft).Intersect(dst.Bounds())
	for row := target.Min.Y; row < target.Max.Y; row++ {
		from := src.PixOffset(bounds.Min.X+target.Min.X-topLeft.X, bounds.Min.Y+row-topLeft.Y)
		copy(dst.Pix[dst.PixOffset(target.Min.X, row):dst.PixOffset(target.Max.X, row)], src.Pix[from:])
	}
}

// rasterize draws an SVG in a box of size: at its own aspect ratio fitted in it if fitted,
// else at the box's (the SVG's preserveAspectRatio then places it). Its text is drawn with
// faces.
func rasterize(data []byte, svgPath string, size image.Point, files Files, faces *layout.Faces, fitted bool) (*image.Gray, error) {
	if fitted {
		natural, err := svgimage.Size(data)
		if err != nil {
			return nil, err
		}
		scale := min(float64(size.X)/float64(natural.X), float64(size.Y)/float64(natural.Y))
		size = scaled(natural, scale)
		size = image.Pt(max(size.X, 1), max(size.Y, 1))
	}
	return svgimage.Rasterize(data, size, svgimage.Options{
		Resolve: func(href string) ([]byte, error) {
			return files.Resource(uri.Resolve(svgPath, href))
		},
		Text: faces.SVGText(),
	})
}

// stretch returns src scaled to size, its aspect ratio not kept: each pixel is the source
// pixel under its centre. src is fitted in size already, so that this only stretches it.
func stretch(src *image.Gray, size image.Point) *image.Gray {
	bounds := src.Bounds()
	dst := image.NewGray(image.Rectangle{Max: size})
	for row := range size.Y {
		sourceRow := bounds.Min.Y + (2*row+1)*bounds.Dy()/(2*size.Y)
		for column := range size.X {
			sourceColumn := bounds.Min.X + (2*column+1)*bounds.Dx()/(2*size.X)
			dst.Pix[row*dst.Stride+column] = src.Pix[src.PixOffset(sourceColumn, sourceRow)]
		}
	}
	return dst
}
