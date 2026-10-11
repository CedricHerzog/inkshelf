package widgets

import (
	"image"
	"image/color"
)

// Panel lays out centred lines of text and buttons down Area, from Top: the end of a book,
// the question whether to delete it, the hold menu of a book.
type Panel struct {
	Fonts   Fonts
	Targets *Targets
	Area    image.Rectangle
	Top     int // where the next line or button goes
}

// PanelButton is a button of a panel, and what a tap on it does. The panel sets the target's
// Rect when it draws the button.
type PanelButton struct {
	Label string
	OnTap Target
}

// Sizes of a panel's buttons: their width and height, and the distance from one to the next.
const (
	PanelButtonWidth  = 640
	PanelButtonHeight = 110
	PanelButtonStep   = 150
)

// Line draws message centred, wrapped to the area's width.
func (panel *Panel) Line(font Font, gray color.Gray, message string) {
	for _, line := range WrapWords(font, Clean(message), panel.Area.Dx()-2*Px(WideMargin)) {
		Text(font, gray, image.Pt(panel.Area.Min.X+(panel.Area.Dx()-font.Width(line))/2, panel.Top), line)
		panel.Top += font.LineHeight()
	}
}

// Gap leaves space between two groups of lines.
func (panel *Panel) Gap() { panel.Top += Px(100) }

// Button draws a button centred, and makes it a target.
func (panel *Panel) Button(item PanelButton) {
	buttonWidth := min(Px(PanelButtonWidth), panel.Area.Dx()-2*Px(40))
	left := panel.Area.Min.X + (panel.Area.Dx()-buttonWidth)/2
	rect := image.Rect(left, panel.Top, left+buttonWidth, panel.Top+Px(PanelButtonHeight))
	Button(panel.Fonts.Bold, rect, item.Label, true)
	item.OnTap.Rect = rect
	panel.Targets.Add(item.OnTap)
	panel.Top += Px(PanelButtonStep)
}
