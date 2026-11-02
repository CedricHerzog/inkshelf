package pocketbook

import "image"

// Orientation is which way the app holds the screen. The panel is mounted sideways: its
// framebuffer is natively landscape (1872x1404 on an InkPad 3), and the portrait screen the
// firmware and every other app use is turned a quarter to fit it.
type Orientation int

// The orientations.
const (
	// Portrait is the screen as the firmware shares it, 1404x1872, turned into the framebuffer.
	Portrait Orientation = iota
	// Landscape is a canvas of the app's own, 1872x1404, copied to the framebuffer as it is: a
	// two-page spread.
	Landscape
)

// SetOrientation makes the app draw in orientation from now on: ScreenSize, Screen and Canvas
// return that orientation's canvas, touches arrive in its coordinates, and the next refresh
// flashes the whole panel, which still shows the other orientation. Setting the current
// orientation does nothing. Call it on the main thread only, before drawing.
func SetOrientation(orientation Orientation) {
	if current == nil || orientation == current.orientation {
		return
	}
	current.orientation = orientation
	current.canvas = current.portrait
	if orientation == Landscape {
		if current.landscape == nil {
			size := current.portrait.Rect.Size()
			current.landscape = image.NewGray(image.Rect(0, 0, size.Y, size.X))
		}
		current.canvas = current.landscape
	}
	current.flashNext = true
}

// ScreenOrientation returns the orientation the app draws in.
func ScreenOrientation() Orientation {
	if current == nil {
		return Portrait
	}
	return current.orientation
}

// oriented returns the size of the screen in the current orientation, given its portrait size.
func oriented(portraitSize image.Point) image.Point {
	if ScreenOrientation() == Landscape {
		return image.Pt(portraitSize.Y, portraitSize.X)
	}
	return portraitSize
}

// isLandscape reports whether a screen of size is landscape: wider than tall.
func isLandscape(size image.Point) bool { return size.X > size.Y }

// followingPanel returns panel, a copy of what the panel shows, if it has canvas's size; else a
// white one of that size, after the orientation changed. The flash refresh that follows a
// change fills it.
func followingPanel(panel, canvas *image.Gray) *image.Gray {
	if panel.Rect == canvas.Rect {
		return panel
	}
	followed := image.NewGray(canvas.Rect)
	for index := range followed.Pix {
		followed.Pix[index] = white.Y
	}
	return followed
}
