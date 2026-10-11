package shelf

import (
	"fmt"
	"image"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
)

// TestPrepareSVGCover checks that an SVG cover is drawn on the screen's size, where
// pageimage.Prepare alone cannot decode it.
func TestPrepareSVGCover(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 60 90"><rect x="10" y="10" width="40" height="70" fill="black"/></svg>`)
	packed, err := prepareCover(svg, image.Pt(300, 400), nil)
	if err != nil {
		t.Fatal(err)
	}
	cover, err := pageimage.Unpack(packed)
	if err != nil {
		t.Fatal(err)
	}
	if size := cover.Bounds().Size(); size != image.Pt(300, 400) {
		t.Errorf("cover %v, want 300x400", size)
	}
	if center := cover.GrayAt(150, 200).Y; center > 64 {
		t.Errorf("the centre of the cover is %d, want the black rectangle", center)
	}
}

// TestPrepareSVGCoverLinked checks that an SVG cover draws the image it links to, relative to
// the cover's path, as SVG cover wrappers do.
func TestPrepareSVGCoverLinked(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 60 90"><image xlink:href="../images/cover.png" width="60" height="90"/></svg>`)
	black := encodePNG(t, image.NewGray(image.Rect(0, 0, 60, 90)))
	linked := func(href string) ([]byte, error) {
		if linkedPath := uri.Resolve("OEBPS/text/cover.svg", href); linkedPath != "OEBPS/images/cover.png" {
			return nil, fmt.Errorf("%s: not in the book", linkedPath)
		}
		return black, nil
	}
	packed, err := prepareCover(svg, image.Pt(300, 400), linked)
	if err != nil {
		t.Fatal(err)
	}
	cover, err := pageimage.Unpack(packed)
	if err != nil {
		t.Fatal(err)
	}
	if center := cover.GrayAt(150, 200).Y; center > 64 {
		t.Errorf("the centre of the cover is %d, want the linked black image", center)
	}
}
