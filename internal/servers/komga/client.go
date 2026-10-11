package komga

import (
	"bytes"
	"context"
	"net/http"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/httpclient"
)

// Client calls Komga's REST API with an API key.
type Client struct {
	config     servers.Config
	httpClient *http.Client
}

// NewClient returns a client for the server of config, which sends its requests through
// httpClient (httpclient.New on the reader, a test server's client in tests).
func NewClient(config servers.Config, httpClient *http.Client) *Client {
	return &Client{config: config, httpClient: httpClient}
}

// resultPage is a page of results, as Komga returns lists. The app always asks for all the
// results at once, or for the first ones only.
type resultPage[Item any] struct {
	Content []Item `json:"content"`
}

// getJSON sends a GET request and decodes its JSON response into value.
func (client *Client) getJSON(ctx context.Context, path string, value any) error {
	request, err := client.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return httpclient.GetJSON(client.httpClient, request, path, value)
}

// getBytes sends a GET request and returns the body of its response.
func (client *Client) getBytes(ctx context.Context, path string) ([]byte, error) {
	request, err := client.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return httpclient.GetBytes(client.httpClient, request, path)
}

// do sends a request with the API key, and a JSON body unless body is nil. It returns the
// response if its status is 2xx, else an *httpclient.StatusError.
func (client *Client) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	request, err := client.newRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	return httpclient.Do(client.httpClient, request, path)
}

// newRequest makes a request with the API key, and a JSON body unless body is nil.
func (client *Client) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, client.config.Server+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-API-Key", client.config.APIKey)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}
