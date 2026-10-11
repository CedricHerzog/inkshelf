package mealie

import (
	"context"
	"net/http"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/httpclient"
)

// Client calls Mealie's REST API with an API token.
type Client struct {
	config     servers.Config
	httpClient *http.Client
}

// NewClient returns a client for the server of config, which sends its requests through
// httpClient (httpclient.New on the reader, a test server's client in tests).
func NewClient(config servers.Config, httpClient *http.Client) *Client {
	return &Client{config: config, httpClient: httpClient}
}

// resultPage is a page of results, as Mealie returns lists. The client always asks for all
// the results at once.
type resultPage[Item any] struct {
	Items []Item `json:"items"`
}

// getJSON sends a GET request and decodes its JSON response into value.
func (client *Client) getJSON(ctx context.Context, path string, value any) error {
	request, err := client.newRequest(ctx, path)
	if err != nil {
		return err
	}
	return httpclient.GetJSON(client.httpClient, request, path, value)
}

// getBytes sends a GET request and returns the body of its response.
func (client *Client) getBytes(ctx context.Context, path string) ([]byte, error) {
	request, err := client.newRequest(ctx, path)
	if err != nil {
		return nil, err
	}
	return httpclient.GetBytes(client.httpClient, request, path)
}

// newRequest makes a GET request with the API token.
func (client *Client) newRequest(ctx context.Context, path string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.config.Server+path, http.NoBody)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+client.config.APIKey)
	return request, nil
}
