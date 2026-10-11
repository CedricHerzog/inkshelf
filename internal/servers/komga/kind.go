package komga

import (
	_ "embed" // the logo

	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// logo is Komga's logo (icons/komga.svg, made by make icons).
//
//go:embed logo.png
var logo []byte

// Kind is what the app shows of a Komga server.
var Kind = servers.Kind{Name: "Komga", Logo: logo, Words: servers.BookWords}
