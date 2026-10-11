package komga

import (
	"context"

	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// Libraries returns the libraries the API key can see: root folders of series, e.g. Manga,
// Comics, BD.
func (client *Client) Libraries(ctx context.Context) ([]servers.Library, error) {
	var libraries []servers.Library
	err := client.getJSON(ctx, "/api/v1/libraries", &libraries)
	return libraries, err
}
