package pocketbook

import (
	"image"
	"image/color"
)

// white is the color of a blank canvas, and of the panel before anything is drawn. The app's own
// grays are in its widgets package.
var white = color.Gray{Y: 0xff}

// ClearScreen fills the canvas with white.
func ClearScreen() { Fill(Screen(), white) }

// Fill fills rect of the canvas with gray. Like every drawing function, it only changes the
// canvas: Refresh shows the result.
func Fill(rect image.Rectangle, gray color.Gray) {
	if current == nil {
		return
	}
	canvas := current.canvas
	rect = rect.Intersect(canvas.Rect)
	if rect.Empty() {
		return
	}
	// Fill the first row pixel by pixel, then copy it into the other rows: copy is much faster
	// than a loop.
	first := canvas.Pix[canvas.PixOffset(rect.Min.X, rect.Min.Y):][:rect.Dx()]
	for column := range first {
		first[column] = gray.Y
	}
	for offset := range rect.Dy() - 1 {
		row := rect.Min.Y + 1 + offset
		copy(canvas.Pix[canvas.PixOffset(rect.Min.X, row):][:rect.Dx()], first)
	}
}

// DrawImage copies img into the canvas with its top-left corner at origin, clipped to the
// screen. It copies whole rows, which is all a page turn costs once its page is decoded.
func DrawImage(img *image.Gray, origin image.Point) {
	if current == nil {
		return
	}
	canvas := current.canvas
	dst := img.Rect.Sub(img.Rect.Min).Add(origin).Intersect(canvas.Rect)
	for offset := range dst.Dy() {
		row := dst.Min.Y + offset
		src := img.Pix[img.PixOffset(dst.Min.X-origin.X+img.Rect.Min.X, row-origin.Y+img.Rect.Min.Y):][:dst.Dx()]
		copy(canvas.Pix[canvas.PixOffset(dst.Min.X, row):], src)
	}
}

// Canvas returns the canvas the app draws into, for drawing that the functions here don't do
// (text, for one). It is nil when the app is not running.
func Canvas() *image.Gray {
	if current == nil {
		return nil
	}
	return current.canvas
}

// ScreenSize returns the screen's size in pixels (1404x1872 on an InkPad 3).
func ScreenSize() image.Point {
	if current == nil {
		return image.Point{}
	}
	return current.canvas.Rect.Size()
}

// Screen returns the screen's rectangle.
func Screen() image.Rectangle { return image.Rectangle{Max: ScreenSize()} }
