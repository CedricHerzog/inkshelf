// Package registry opens the Server of each kind of server. A new kind is a package under
// internal/servers and a case in Opener and in KindOf.
package registry

import (
	"fmt"
	"net/http"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/calibreweb"
	"github.com/CedricHerzog/inkshelf/internal/servers/komga"
	"github.com/CedricHerzog/inkshelf/internal/servers/mealie"
)

// KindOf returns the servers.KindOf of every kind of server: its name, logo and words.
func KindOf(config servers.Config) servers.Kind {
	switch config.Type {
	case "", "komga":
		return komga.Kind
	case "mealie":
		return mealie.Kind
	case "calibreweb", "calibre-web":
		return calibreweb.Kind
	}
	return servers.Kind{Name: config.Type, Words: servers.BookWords}
}

// Opener returns the servers.Opener of every kind of server, which sends requests through
// httpClient.
func Opener(httpClient *http.Client) servers.Opener {
	return func(config servers.Config) (servers.Server, error) {
		switch config.Type {
		case "", "komga":
			return komga.NewServer(config, httpClient), nil
		case "mealie":
			return mealie.NewServer(config, httpClient), nil
		case "calibreweb", "calibre-web":
			return calibreweb.NewServer(config, httpClient), nil
		}
		return nil, fmt.Errorf("server %s: unknown type %q", config.Name, config.Type)
	}
}
