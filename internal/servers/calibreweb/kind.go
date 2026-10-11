package calibreweb

import (
	_ "embed" // the logo

	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// logo is the logo of Calibre-Web servers (icons/calibreweb.svg, made by make icons).
//
//go:embed logo.png
var logo []byte

// Kind is what the app shows of a Calibre-Web server.
var Kind = servers.Kind{Name: "Calibre-Web", Logo: logo, Words: servers.BookWords}
