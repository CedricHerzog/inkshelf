package mealie

import (
	_ "embed" // the logo

	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// logo is Mealie's logo (icons/mealie.svg, made by make icons).
//
//go:embed logo.png
var logo []byte

// Kind is what the app shows of a Mealie server: its categories are the series, its recipes
// the books.
var Kind = servers.Kind{
	Name:  "Mealie",
	Logo:  logo,
	Words: servers.Words{Series: "category", SeriesPlural: "categories", Book: "recipe", BookPlural: "recipes"},
}
