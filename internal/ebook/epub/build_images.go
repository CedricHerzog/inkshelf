package epub

import (
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
)

// Lengths in hundredths of an em.
const (
	// imageMargin is the space above and below an image that is a block of its own.
	imageMargin = 25
	// smallImageHeight is the height up to which an image in a line of text stays in it.
	smallImageHeight = 200
)

// addImage adds the image at source (the src of an <img>, the href of an SVG <image>…) of the
// element img: the first image of its manifest fallback chain that the book can show, else
// source itself; a data: URI as it is. A small image in a line of text goes in it; another
// is a block of its own.
func (parser *builder) addImage(source string, img *element) {
	if source == "" {
		return
	}
	imagePath := source
	if !uri.IsData(source) {
		imagePath = resolve(parser.chapterPath, source)
		if fallback, ok := parser.book.SupportedImage(imagePath); ok {
			imagePath = fallback
		}
	}
	if parser.floatImage(imagePath, img) {
		return
	}
	width, height, sized := parser.imageSize(imagePath, img)
	if height > 0 && height <= smallImageHeight*img.style.fontSize/100 {
		run := img.run
		if parser.spaceNeeded && parser.pendingRunes > 0 {
			run.Text = " "
			parser.appendRun(run)
		}
		parser.spaceNeeded = false
		run.Text, run.Image = "", imagePath
		if sized {
			run.ImageHeight = height * 100 / parser.blockElement().block.fontSize
		}
		parser.appendRun(run)
		return
	}
	widthPercent := 0
	if sized {
		widthPercent = min(max(width*100/textWidth, 1), 100)
	}
	parser.addImageBlock(imagePath, widthPercent)
}

// imageSize returns the size of an image in hundredths of an em, 0 if unknown: from its
// element's CSS (or its width and height attributes, for an <img>) and its own size in CSS
// pixels. sized is set if CSS or the attributes give it.
func (parser *builder) imageSize(imagePath string, img *element) (width, height int, sized bool) {
	width, height = img.style.width, img.style.height
	if img.name == "img" {
		if width == 0 {
			width, _ = lengthOf(img.attributes["width"]+"px", 100, textWidth)
		}
		if height == 0 {
			height, _ = lengthOf(img.attributes["height"]+"px", 100, 0)
		}
	}
	sized = width > 0 || height > 0 || img.style.maxWidth > 0
	if width == 0 || height == 0 {
		natural, err := parser.book.ImageSize(imagePath)
		switch {
		case err != nil || natural.X <= 0 || natural.Y <= 0:
		case width == 0 && height == 0:
			width, height = natural.X*100/pixelsPerEm, natural.Y*100/pixelsPerEm
		case width == 0:
			width = height * natural.X / natural.Y
		default:
			height = width * natural.Y / natural.X
		}
	}
	if maxWidth := img.style.maxWidth; maxWidth > 0 && width > maxWidth {
		width, height = maxWidth, height*maxWidth/width
	}
	return width, height, sized
}

// addImageBlock adds an image as a block, widthPercent wide (0 for its own width).
func (parser *builder) addImageBlock(imagePath string, widthPercent int) {
	parser.flush()
	blockElement := parser.blockElement()
	props := blockElement.block
	block := document.Block{
		Kind: document.Image, Image: imagePath, ImageWidth: widthPercent,
		Align: props.align, Left: props.left, Right: props.right,
		MarginTop: max(parser.pending.marginTop, imageMargin), MarginBottom: imageMargin,
		PageBreak: parser.pending.pageBreak, BorderTop: parser.pending.borderTop, Footnote: noteIDOf(blockElement),
	}
	parser.floatBlock(&block, blockElement)
	parser.addBlock(block)
	parser.pending, parser.continued = pending{}, false
}
