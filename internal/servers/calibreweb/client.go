package calibreweb

import (
	"context"
	"io"
	"net/http"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/httpclient"
)

// Client reads Calibre-Web's OPDS catalog with a username and a password.
type Client struct {
	config     servers.Config
	httpClient *http.Client
	// fileClient downloads books: httpClient's transport without its timeout, which would
	// cut a large file's download. The request's context stops it.
	fileClient *http.Client
}

// NewClient returns a client for the server of config, which sends its requests through
// httpClient (httpclient.New on the reader, a test server's client in tests).
func NewClient(config servers.Config, httpClient *http.Client) *Client {
	fileClient := *httpClient
	fileClient.Timeout = 0
	return &Client{config: config, httpClient: httpClient, fileClient: &fileClient}
}

// getBytes sends a GET request and returns the body of its response.
func (client *Client) getBytes(ctx context.Context, path string) ([]byte, error) {
	request, err := client.newRequest(ctx, path)
	if err != nil {
		return nil, err
	}
	return httpclient.GetBytes(client.httpClient, request, path)
}

// File starts the download of a book's file at path (an acquisition link), and returns its
// body and size, -1 if unknown. The caller closes the body.
func (client *Client) File(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	request, err := client.newRequest(ctx, path)
	if err != nil {
		return nil, 0, err
	}
	response, err := httpclient.Do(client.fileClient, request, path)
	if err != nil {
		return nil, 0, err
	}
	return response.Body, response.ContentLength, nil
}

// newRequest makes a GET request with the username and password.
func (client *Client) newRequest(ctx context.Context, path string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.config.Server+path, http.NoBody)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(client.config.Username, client.config.Password)
	return request, nil
}
