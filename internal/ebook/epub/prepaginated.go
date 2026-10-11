package epub

import (
	"bytes"
	"cmp"
	"fmt"
	"image"
	"image/png"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/container"
	"github.com/CedricHerzog/inkshelf/internal/ebook/fixedlayout"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
)

// A pre-paginated (fixed-layout) chapter is a page drawn whole, not text that flows. Its
// elements are read into regions of a fixedlayout.Page: the flow of the page, plus a region for
// each element with position: absolute or fixed and for each background. A book whose chapters
// are all pre-paginated is drawn page by page with RenderPage. In a book that mixes both
// layouts, the document of a pre-paginated chapter is a single image whose path starts with
// renderedPagePrefix, and Resource draws that image.

// renderedPagePrefix starts the path of the image that the document of a pre-paginated spine
// item shows, followed by the item's path. Resource draws the page and ImageSize gives its
// size.
const renderedPagePrefix = "rendered-page:"

// renderedPageSize is the size that a pre-paginated chapter of a mixed book is drawn at: the
// screen of a 300 ppi reader. The text reader fits the image in its page.
var renderedPageSize = image.Pt(1404, 1872)

// positionedElement is an element with position: absolute or fixed while it is open. Its
// content goes into region, and outer keeps the document of the content around it, which the
// builder goes back to when the element closes.
type positionedElement struct {
	region *fixedlayout.Region
	outer  document.Document
}

// FixedLayout reports whether every chapter of the book is pre-paginated: a book of pages,
// which RenderPage draws.
func (book *Book) FixedLayout() bool {
	for index := range book.Chapters() {
		if book.ChapterLayout(index) != PrePaginated {
			return false
		}
	}
	return true
}

// RenderPage draws chapter index (counted from 0) as a fixed-layout page fitted in size.
func (book *Book) RenderPage(index int, size image.Point) (*image.Gray, error) {
	if index < 0 || index >= book.Chapters() {
		return nil, fmt.Errorf("chapter %d of %d", index, book.Chapters())
	}
	page, err := book.fixedPage(book.ChapterItem(index))
	if err != nil {
		return nil, err
	}
	return page.Render(size, book)
}

// renderedPage returns the page of the spine item at itemPath, drawn at renderedPageSize, as
// a PNG: the image of its document.
func (book *Book) renderedPage(itemPath string) ([]byte, error) {
	page, err := book.fixedPageAt(itemPath)
	if err != nil {
		return nil, err
	}
	drawn, err := page.Render(renderedPageSize, book)
	if err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&encoded, drawn); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

// renderedPageSizeOf returns the size of renderedPage's image of the spine item at itemPath.
func (book *Book) renderedPageSizeOf(itemPath string) (image.Point, error) {
	page, err := book.fixedPageAt(itemPath)
	if err != nil {
		return image.Point{}, err
	}
	return page.FittedSize(renderedPageSize), nil
}

// fixedPageAt returns the page of the first spine item at itemPath.
func (book *Book) fixedPageAt(itemPath string) (*fixedlayout.Page, error) {
	for _, item := range book.Spine() {
		if item.Path == itemPath {
			return book.fixedPage(item)
		}
	}
	return nil, fmt.Errorf("%s: not in the spine", itemPath)
}

// fixedPage reads a spine item as a fixed-layout page: the regions of its XHTML, or its image.
// The page's size is its viewport's, else its first image's, else fixedlayout.DefaultSize.
func (book *Book) fixedPage(item container.SpineItem) (*fixedlayout.Page, error) {
	page := &fixedlayout.Page{EmPixels: pixelsPerEm}
	flow := &fixedlayout.Region{}
	page.Regions = append(page.Regions, flow)
	switch {
	case item.Content == "" || strings.HasPrefix(item.ContentType, "image/"):
		// Built as a reflowable item is: its image, or a paragraph saying it cannot be shown.
		item.Layout = Reflowable
		doc, err := book.documentOf(item, false)
		if err != nil {
			return nil, err
		}
		flow.Content = doc
	default:
		doc, err := book.buildDocument(item.Content, page, false)
		if err != nil {
			return nil, err
		}
		flow.Content = doc
		for _, region := range page.Regions {
			if region.Content != nil {
				region.Content.Fonts, region.Content.Language = doc.Fonts, doc.Language
			}
		}
	}
	if page.Size == (image.Point{}) {
		page.Size = fixedlayout.DefaultSize
		if imagePath := firstImage(page); imagePath != "" {
			if size, err := book.ImageSize(imagePath); err == nil && size.X > 0 && size.Y > 0 {
				page.Size = size
			}
		}
	}
	flow.Rect = image.Rectangle{Max: page.Size}
	return page, nil
}

// firstImage returns the path of the first image in a page's regions, "" if there is none.
func firstImage(page *fixedlayout.Page) string {
	for _, region := range page.Regions {
		if region.Content == nil {
			continue
		}
		for _, block := range region.Content.Blocks {
			if block.Kind == document.Image {
				return block.Image
			}
			for _, run := range block.Runs {
				if run.Image != "" {
					return run.Image
				}
			}
		}
	}
	return ""
}

// readViewport takes the page's size from a viewport <meta>, the first one that gives a size.
func (parser *builder) readViewport(meta *element) {
	if parser.page.Size != (image.Point{}) || !strings.EqualFold(meta.attributes["name"], "viewport") {
		return
	}
	if size, ok := fixedlayout.ParseViewport(meta.attributes["content"]); ok {
		parser.page.Size = size
	}
}

// pageSize returns the size of the page: its viewport's, else the default size while the
// chapter is being read.
func (parser *builder) pageSize() image.Point {
	return cmp.Or(parser.page.Size, fixedlayout.DefaultSize)
}

// placeOnPage follows the CSS that places an element of a pre-paginated chapter on its page. A
// background becomes a region drawn under the content. position: absolute or fixed takes the
// element out of the flow into a region of its own, which its content goes into.
func (parser *builder) placeOnPage(opened *element, declared map[string]string) {
	containing := image.Rectangle{Max: parser.pageSize()}
	for ancestor := opened.parent; ancestor != nil; ancestor = ancestor.parent {
		if ancestor.positioned != nil {
			containing = ancestor.positioned.region.Rect
			break
		}
	}
	position := strings.ToLower(declared["position"])
	positioned := position == "absolute" || position == "fixed"
	background := parser.backgroundOf(declared)
	color, gradient := backgroundFill(declared)
	if !positioned && opened.name != "html" && opened.name != "body" && (declared["width"] == "" || declared["height"] == "") {
		// The box of an element in the flow is not known: its colors would cover the page.
		color, gradient = css.Color{}, nil
	}
	if !positioned && background == "" && color.Alpha == 0 && gradient == nil {
		return
	}
	rect := parser.boxRect(opened, declared, containing)
	if opened.name == "html" || opened.name == "body" {
		rect = image.Rectangle{Max: parser.pageSize()} // their background is the page's
	}
	region := &fixedlayout.Region{Rect: rect, Color: color, Gradient: gradient, Background: background}
	parser.page.Regions = append(parser.page.Regions, region)
	if !positioned {
		return // only its background is placed: its content stays in the flow
	}
	if opened.block == nil {
		parser.openBlock(opened) // a positioned element is a block
	}
	// Its text starts at the region's edges: the margins of the elements around it stay out.
	opened.block.left, opened.block.right = 0, 0
	opened.positioned = &positionedElement{region: region, outer: parser.doc}
	parser.doc, parser.pending = document.Document{}, pending{}
}

// closePositioned ends a positioned element: its content becomes its region's, and the builder
// goes back to the content around it.
func (parser *builder) closePositioned(closed *element) {
	if content := parser.doc; len(content.Blocks) > 0 {
		closed.positioned.region.Content = &content
	}
	parser.doc, parser.pending, parser.continued = closed.positioned.outer, pending{}, false
	closed.positioned = nil
}

// backgroundFill returns the background color declared (transparent if none) and the
// linear-gradient of its background image (nil if none), from background-color and
// background-image, else from the background shorthand. currentColor is the color declared
// for the element, else black.
func backgroundFill(declared map[string]string) (css.Color, *css.Gradient) {
	current := css.Black
	if value, ok := declared["color"]; ok {
		current, _ = css.ParseColor(value, css.Black)
	}
	color, ok := css.ParseColor(declared["background-color"], current)
	if !ok {
		color, _ = css.BackgroundColor(declared["background"], current)
	}
	layers := cmp.Or(declared["background-image"], declared["background"])
	if layered(layers) || tiled(declared["background-size"]) {
		return color, nil // a pattern of gradients (a grid, stripes): drawn as one, it would hide the page
	}
	gradient, ok := css.ParseLinearGradient(layers, current)
	if !ok {
		return color, nil
	}
	return color, &gradient
}

// layered reports whether a background image value holds more than one layer: a comma out of
// parentheses.
func layered(value string) bool {
	depth := 0
	for _, character := range value {
		switch character {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

// tiled reports whether a background-size repeats the image in a box: any size but the
// box's own.
func tiled(size string) bool {
	switch strings.Join(strings.Fields(strings.ToLower(size)), " ") {
	case "", "auto", "auto auto", "100%", "100% 100%", "cover", "contain":
		return false
	}
	return true
}

// backgroundOf returns the path in the book of the background image declared, "" if none.
// Its URL is relative to the stylesheet that declares it, which the cascade forgets: it is
// looked for from the chapter, then from each of its stylesheets.
func (parser *builder) backgroundOf(declared map[string]string) string {
	address, ok := css.URL(cmp.Or(declared["background-image"], declared["background"]))
	if !ok || address == "" {
		return ""
	}
	bases := append([]string{parser.chapterPath}, slices.Sorted(maps.Keys(parser.sheetsAdded))...)
	for _, base := range bases {
		if imagePath := resolve(base, address); parser.book.HasFile(imagePath) {
			return imagePath
		}
	}
	return ""
}

// boxRect returns the box of an element in its containing block, in CSS pixels: from its
// left, top, right, bottom, width and height, the size of an <img> without them its image's.
func (parser *builder) boxRect(opened *element, declared map[string]string, containing image.Rectangle) image.Rectangle {
	fontPixels := float64(opened.style.fontSize*pixelsPerEm) / 100
	lengthOf := func(property string, hundred int) cssLength {
		value := declared[property]
		if value == "" && opened.name == "img" && (property == "width" || property == "height") {
			value = opened.attributes[property] + "px" // its attribute, a number of pixels
		}
		pixels, set := pixelsOf(value, float64(hundred), fontPixels)
		return cssLength{pixels, set}
	}
	width, height := lengthOf("width", containing.Dx()), lengthOf("height", containing.Dy())
	if opened.name == "img" && (!width.set || !height.set) {
		width, height = parser.imageBoxSize(opened.attributes["src"], width, height)
	}
	left, boxWidth := placeAxis(lengthOf("left", containing.Dx()), lengthOf("right", containing.Dx()), width, float64(containing.Dx()))
	top, boxHeight := placeAxis(lengthOf("top", containing.Dy()), lengthOf("bottom", containing.Dy()), height, float64(containing.Dy()))
	round := func(value float64) int { return int(math.Round(value)) }
	return image.Rect(round(left), round(top), round(left+boxWidth), round(top+boxHeight)).Add(containing.Min)
}

// imageBoxSize returns the size of the box of an <img> whose source is source and whose CSS
// gives width or height or neither: its image's size, or the other at its aspect ratio.
func (parser *builder) imageBoxSize(source string, width, height cssLength) (cssLength, cssLength) {
	imagePath := source
	if !uri.IsData(source) {
		imagePath = resolve(parser.chapterPath, source)
		if fallback, ok := parser.book.SupportedImage(imagePath); ok {
			imagePath = fallback
		}
	}
	natural, err := parser.book.ImageSize(imagePath)
	switch {
	case err != nil || natural.X <= 0 || natural.Y <= 0:
	case width.set:
		height = cssLength{width.pixels * float64(natural.Y) / float64(natural.X), true}
	case height.set:
		width = cssLength{height.pixels * float64(natural.X) / float64(natural.Y), true}
	default:
		width, height = cssLength{float64(natural.X), true}, cssLength{float64(natural.Y), true}
	}
	return width, height
}

// cssLength is a length in CSS pixels, if set.
type cssLength struct {
	pixels float64
	set    bool
}

// pixelsOf returns a CSS length in pixels: a percentage of hundred, em of fontPixels. It
// returns false if value is no length (auto, a number other than 0…).
func pixelsOf(value string, hundred, fontPixels float64) (float64, bool) {
	length, ok := css.ParseLength(value)
	if !ok {
		return 0, false
	}
	switch length.Unit {
	case "%":
		return length.Value * hundred / 100, true
	case "em":
		return length.Value * fontPixels, true
	case "rem":
		return length.Value * pixelsPerEm, true
	}
	return length.Pixels()
}

// placeAxis returns where a box starts and how long it is along one axis of its containing
// block, containing long: from its start and end offsets (left and right, or top and bottom)
// and its length, each set or not. Without a length, the box stretches between its offsets;
// with an end offset only, it ends there.
func placeAxis(start, end, length cssLength, containing float64) (float64, float64) {
	if !length.set {
		length.pixels = containing - start.pixels - end.pixels
	}
	if !start.set && end.set {
		start.pixels = containing - end.pixels - length.pixels
	}
	return start.pixels, max(length.pixels, 0)
}
