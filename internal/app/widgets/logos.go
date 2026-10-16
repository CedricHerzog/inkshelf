package widgets

import (
	"image"
	"log"

	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// AppName is the app's name, shown until it knows the kind of server it talks to.
const AppName = "Inkshelf"

// KindTitle returns the name of a kind of server, or the app's while it is unknown.
func KindTitle(kind servers.Kind) string {
	if kind.Name != "" {
		return kind.Name
	}
	return AppName
}

// Words returns how a kind of server calls its series and books, a recipe server's being
// categories and recipes. Every label that names them goes through it.
func Words(kind servers.Kind) servers.Words {
	if kind.Words == (servers.Words{}) {
		return servers.BookWords
	}
	return kind.Words
}

// logoKey is the logo of a kind of server, fitted to a size.
type logoKey struct {
	name string
	size image.Point
}

// Logos are the logos drawn so far, fitted to the size they were drawn at, or nil for those
// that could not be decoded. A logo is a PNG, and fitting it takes a while: it is done once.
// Only the main thread, which draws, uses them.
type Logos map[logoKey]*image.Gray

// Draw draws the logo of kind centred in box, and reports whether the kind has one.
func (logos *Logos) Draw(kind servers.Kind, box image.Rectangle) bool {
	if kind.Logo == nil || box.Empty() {
		return false
	}
	key := logoKey{kind.Name, box.Size()}
	logo, ok := (*logos)[key]
	if !ok {
		var err error
		if logo, err = fitLogo(kind.Logo, box.Size()); err != nil {
			log.Printf("logo of %s: %v", kind.Name, err)
		}
		if *logos == nil {
			*logos = Logos{}
		}
		(*logos)[key] = logo
	}
	if logo == nil {
		return false
	}
	// Not a Picture: a logo is a one-color icon drawn like text, so it shows white in dark mode,
	// as the firmware's icons do.
	pocketbook.DrawImage(logo, box.Min.Add(box.Size().Sub(logo.Bounds().Size()).Div(2)))
	return true
}

// fitLogo decodes a logo, fitted within size and in the grays of the panel.
func fitLogo(data []byte, size image.Point) (*image.Gray, error) {
	packed, err := pageimage.Prepare(data, size.X, size.Y, nil)
	if err != nil {
		return nil, err
	}
	return pageimage.Unpack(packed)
}
