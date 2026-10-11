package svgimage

import (
	"encoding/xml"
	"errors"
	"image"
	"image/draw"

	xdraw "golang.org/x/image/draw"

	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage" // decodes the linked images, JPEG XL too
	"github.com/CedricHerzog/inkshelf/internal/web/css"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
)

// Resolver returns the data of an image an SVG links to by href, as written in the SVG:
// relative to the SVG's file. data: URIs never reach it.
type Resolver func(href string) ([]byte, error)

// Options are what Rasterize draws an SVG with, besides its data.
type Options struct {
	// Resolve reads the raster images it links to; nil leaves them out.
	Resolve Resolver
	// Text draws its text; nil leaves it out.
	Text TextDrawer
	// CSS is author CSS that applies under its own stylesheets.
	CSS string
}

// Rasterize draws an SVG image on white, size pixels large, its viewBox scaled into them as
// its preserveAspectRatio says (by default, fitted and centred). Its styles are computed first
// (style.go), its pattern fills tiled (pattern.go), its text taken out and drawn last, over
// its shapes, through options.Text. The
// raster images it links to are read through options.Resolve; one that cannot be read or
// decoded is left out.
func Rasterize(data []byte, size image.Point, options Options) (*image.Gray, error) {
	if size.X <= 0 || size.Y <= 0 {
		return nil, errors.New("svgimage: empty size")
	}
	root, err := readRoot(newDecoder(data))
	if err != nil {
		return nil, err
	}
	viewBox := root.viewBox
	if viewBox.empty() {
		width, height := root.size()
		viewBox = bounds{width: width, height: height}
	}
	userToPixels := root.aspect.place(viewBox, bounds{width: float64(size.X), height: float64(size.Y)})
	width, height := root.size()
	styled, err := restyle(data, options.CSS, css.Media{Type: "screen", Width: width, Height: height})
	if err != nil {
		return nil, err
	}

	canvas := image.NewRGBA(image.Rectangle{Max: size})
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	decoder := newDecoder(styled.markup)
	if _, err := readRoot(decoder); err != nil {
		return nil, err
	}
	for _, linked := range readImages(decoder) {
		_ = drawLinked(canvas, linked, userToPixels, options.Resolve) // left out if it fails
	}

	if err := drawShapes(canvas, styled, userToPixels.matrix()); err != nil {
		return nil, err
	}

	gray := grayOf(canvas)
	if options.Text != nil {
		drawTexts(gray, styled.texts, userToPixels.matrix(), options.Text)
	}
	return gray, nil
}

// grayOf returns an opaque image in gray, weighted as image/color's GrayModel does.
func grayOf(canvas *image.RGBA) *image.Gray {
	gray := image.NewGray(canvas.Bounds())
	for index := range gray.Pix {
		pixel := canvas.Pix[4*index : 4*index+3 : 4*index+3]
		red, green, blue := uint32(pixel[0]), uint32(pixel[1]), uint32(pixel[2])
		gray.Pix[index] = uint8((19595*red + 38470*green + 7471*blue + 1<<15) >> 16)
	}
	return gray
}

// linkedImage is an <image> of an SVG: a raster image, by href, in a box of user units.
type linkedImage struct {
	href   string
	area   bounds // a width or height of 0 is the image's own
	aspect aspectRatio
}

// referencedOnly are the elements whose content is drawn only where something refers to it,
// if at all: their images are left out.
var referencedOnly = map[string]bool{
	"defs": true, "symbol": true, "pattern": true, "clipPath": true, "mask": true, "marker": true,
}

// readImages reads the <image>s of an SVG from decoder, past its root element.
func readImages(decoder *xml.Decoder) []linkedImage {
	var images []linkedImage
	referencedDepth := 0 // how deep inside a referencedOnly element the decoder is
	for {
		token, err := decoder.Token()
		if err != nil {
			return images
		}
		switch token := token.(type) {
		case xml.StartElement:
			if referencedDepth > 0 || referencedOnly[token.Name.Local] {
				referencedDepth++
			} else if token.Name.Local == "image" {
				images = append(images, readImage(token.Attr))
			}
		case xml.EndElement:
			referencedDepth = max(referencedDepth-1, 0)
		}
	}
}

// readImage returns the <image> of attributes.
func readImage(attributes []xml.Attr) linkedImage {
	linked := linkedImage{aspect: parseAspectRatio("")}
	for _, attribute := range attributes {
		switch attribute.Name.Local {
		case "href": // xlink:href, or SVG 2's href
			linked.href = attribute.Value
		case "x":
			linked.area.x, _ = parseLength(attribute.Value)
		case "y":
			linked.area.y, _ = parseLength(attribute.Value)
		case "width":
			linked.area.width = positiveLength(attribute.Value)
		case "height":
			linked.area.height = positiveLength(attribute.Value)
		case "preserveAspectRatio":
			linked.aspect = parseAspectRatio(attribute.Value)
		}
	}
	return linked
}

// drawLinked draws a linked image on canvas, its box mapped to pixels by userToPixels.
func drawLinked(canvas *image.RGBA, linked linkedImage, userToPixels transform, resolve Resolver) error {
	var data []byte
	var err error
	switch {
	case uri.IsData(linked.href):
		data, err = uri.DecodeData(linked.href)
	case resolve == nil || linked.href == "":
		err = errors.New("svgimage: no image to read")
	default:
		data, err = resolve(linked.href)
	}
	if err != nil {
		return err
	}
	picture, err := pageimage.Decode(data)
	if err != nil {
		return err
	}
	pictureSize := picture.Bounds().Size()
	own := bounds{width: float64(pictureSize.X), height: float64(pictureSize.Y)}
	area := linked.area
	if area.width == 0 {
		area.width = own.width
	}
	if area.height == 0 {
		area.height = own.height
	}
	viewport := userToPixels.apply(area)
	destination := linked.aspect.place(own, viewport).apply(own).rectangle()
	// Scale clips to its destination's bounds, which a slice overflows.
	clipped := canvas.SubImage(viewport.rectangle()).(*image.RGBA)
	xdraw.ApproxBiLinear.Scale(clipped, destination, picture, picture.Bounds(), xdraw.Over, nil)
	return nil
}
