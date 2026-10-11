package state

import "github.com/CedricHerzog/inkshelf/internal/servers"

// Loading is what the loading and server screens show. Guarded by the mutex.
type Loading struct {
	Status  string           // the loading screen's status line
	Err     error            // what stopped loading; the app returns it when it closes
	Servers []servers.Config // to choose from, when config.json has several
}

// Server is the server in use. Guarded by the mutex.
type Server struct {
	Name   string         // shown in the series list's header, when config.json has several
	Client servers.Server // nil until a server is chosen
	Online bool           // the series list came from the server, not only from the downloads
	Kind   servers.Kind   // its logo, name and words; the zero Kind until it is known
	// LocalProgress is set when the server keeps no reading progress (its InProgress is
	// unsupported, as Mealie's): the reader's progress is then the only one.
	LocalProgress bool
}
