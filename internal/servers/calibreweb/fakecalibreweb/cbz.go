package fakecalibreweb

import (
	"fmt"
	"html"
	"image"
	"image/color"
	"image/draw"
)

// makeCBZ returns the CBZ of a book: its pages as JPEGs, with a ComicInfo.xml and macOS
// metadata to skip, named so that only a natural sort orders them.
func makeCBZ(item book) ([]byte, error) {
	files := map[string][]byte{
		"ComicInfo.xml":         []byte("<ComicInfo><Title>" + html.EscapeString(item.Title) + "</Title></ComicInfo>"),
		"__MACOSX/._page 1.jpg": []byte("metadata"),
	}
	for number := range item.Pages {
		img := image.NewRGBA(image.Rectangle{Max: comicPageSize})
		draw.Draw(img, img.Bounds(), image.White, image.Point{}, draw.Src)
		// Two panels framed in black, the page number in the second.
		for _, panel := range []image.Rectangle{image.Rect(60, 60, 1140, 860), image.Rect(60, 940, 1140, 1740)} {
			draw.Draw(img, panel, image.Black, image.Point{}, draw.Src)
			draw.Draw(img, panel.Inset(12), image.NewUniform(color.Gray{Y: uint8(200 - number*15%120)}), image.Point{}, draw.Src)
		}
		lines := []string{item.Title, fmt.Sprintf("Page %d", number+1)}
		if err := drawLines(img, image.Rect(60, 940, 1140, 1740), 80, lines); err != nil {
			return nil, err
		}
		data, err := encodeJPEG(img)
		if err != nil {
			return nil, err
		}
		files[fmt.Sprintf("%s/page %d.jpg", item.Title, number+1)] = data
	}
	return zipFiles(nil, files)
}
