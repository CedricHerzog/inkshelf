package widgets

import (
	"context"
	"image"
	"image/color"

	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// The grays the app draws with. An e-ink panel shows 16 grays, but text and lines read best in
// these four, far enough apart to stay distinct once the panel has ghosted a little.
var (
	Black     = color.Gray{Y: 0x00}
	DarkGray  = color.Gray{Y: 0x55}
	LightGray = color.Gray{Y: 0xaa}
	White     = color.Gray{Y: 0xff}
)

// Show puts what the screen drew on the panel: in negative first if dark, then a flashing
// refresh of the whole panel if full, else a refresh of each of areas with waveform. The app
// draws black on white; in dark mode its screens show dark. The whole canvas is inverted, not
// only areas: the canvas is also the task switcher's thumbnail of the app.
func Show(dark, full bool, waveform pocketbook.Waveform, areas ...image.Rectangle) {
	if dark {
		span := timing.Start(context.Background(), "invert screen")
		pocketbook.Invert(pocketbook.Screen())
		span.End()
	}
	if full {
		pocketbook.FlashRefresh()
		return
	}
	for _, area := range areas {
		pocketbook.Refresh(area, waveform)
	}
}

// Picture draws img with its top-left corner at origin so that it keeps its own colors: in dark
// mode it is drawn in negative, and Show, which inverts the whole screen, gives it back, while
// what is drawn over it (a READ badge, a menu) shows dark like the rest. Use it only on screens
// that Show darkens in dark mode, as every screen but the reader does.
func Picture(img *image.Gray, origin image.Point) {
	pocketbook.DrawImage(img, origin)
	if pocketbook.DarkMode() {
		pocketbook.Invert(img.Rect.Sub(img.Rect.Min).Add(origin))
	}
}

// Text draws label in font and gray, with its top-left corner at topLeft: its baseline is the
// font's ascent below that corner.
func Text(font Font, gray color.Gray, topLeft image.Point, label string) {
	if canvas := pocketbook.Canvas(); canvas != nil {
		font.DrawTo(canvas, topLeft, label, gray)
	}
}

// Button draws a framed label centred in rect, light gray when the button cannot be used.
func Button(font Font, rect image.Rectangle, label string, enabled bool) {
	labelColor := Black
	if !enabled {
		labelColor = LightGray
	}
	Frame(rect, Px(3), labelColor)
	Text(font, labelColor, image.Pt(rect.Min.X+(rect.Dx()-font.Width(label))/2, font.CenterTop(rect)), label)
}

// Frame draws a border thickness pixels thick inside rect.
func Frame(rect image.Rectangle, thickness int, gray color.Gray) {
	pocketbook.Fill(image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+thickness), gray)
	pocketbook.Fill(image.Rect(rect.Min.X, rect.Max.Y-thickness, rect.Max.X, rect.Max.Y), gray)
	pocketbook.Fill(image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+thickness, rect.Max.Y), gray)
	pocketbook.Fill(image.Rect(rect.Max.X-thickness, rect.Min.Y, rect.Max.X, rect.Max.Y), gray)
}

// BackButton draws the Back button at the left of a header, the lists' or the reader
// toolbar's, and returns it. The whole left part of the header goes back, not only the
// button: it is the corner a thumb reaches for.
func BackButton(fonts Fonts, targets *Targets, label string) image.Rectangle {
	margin, bar := Px(ListMargin), Px(BarHeight)
	backButton := image.Rect(margin, (bar-Px(ButtonHeight))/2, margin+Px(240), (bar+Px(ButtonHeight))/2)
	Button(fonts.Bold, backButton, label, true)
	targets.Add(Target{Rect: image.Rect(0, 0, backButton.Max.X+margin, bar), Command: Back})
	return backButton
}

// ExitButton draws the Exit button at the bottom of the screens shown before a server is
// chosen, which have no header.
func ExitButton(fonts Fonts, targets *Targets) {
	screenRect := pocketbook.Screen()
	center, bottom := screenRect.Dx()/2, screenRect.Dy()
	exitButton := image.Rect(center-Px(150), bottom-Px(250), center+Px(150), bottom-Px(150))
	Button(fonts.Bold, exitButton, "Exit", true)
	targets.Tap(exitButton, pocketbook.Exit)
}

// Segments draws labels in equal cells across rect, the selected one inverted, and makes a
// tap on a cell call choose with its index. The lists' filters and the contents' tabs are
// segments.
func Segments(fonts Fonts, targets *Targets, rect image.Rectangle, labels []string, selected int, choose func(index int)) {
	gap := Px(12)
	cellWidth := (rect.Dx() - (len(labels)-1)*gap) / len(labels)
	for index, label := range labels {
		left := rect.Min.X + index*(cellWidth+gap)
		cell := image.Rect(left, rect.Min.Y, left+cellWidth, rect.Max.Y)
		labelColor := Black
		if index == selected {
			pocketbook.Fill(cell, Black)
			labelColor = White
		} else {
			Frame(cell, Px(3), Black)
		}
		label = Ellipsize(fonts.Body, Clean(label), cell.Dx()-Px(24))
		Text(fonts.Body, labelColor, image.Pt(cell.Min.X+(cell.Dx()-fonts.Body.Width(label))/2, fonts.Body.CenterTop(cell)), label)
		targets.Tap(cell.Inset(-gap/2), func() { choose(index) })
	}
}

// SegmentsRule draws the gray line under the segments of band.
func SegmentsRule(band image.Rectangle) {
	margin := Px(ListMargin)
	pocketbook.Fill(image.Rect(margin, band.Max.Y-Px(1), band.Max.X-margin, band.Max.Y+Px(1)), LightGray)
}
