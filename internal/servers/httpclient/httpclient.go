// Package httpclient holds what the clients of every kind of server share: the HTTP client the
// app sends its requests through, and the handling of their responses. Each server builds its
// own requests, with its own way of logging in (an API key, a token, a username and password),
// and hands them here to be sent.
package httpclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/timing"
)

const (
	// maxIdleConnections is how many connections to a server New's client keeps open: one for
	// each page fetched at once and each cover loaded, with room to spare.
	maxIdleConnections = 8
	// requestTimeout is the longest a request may take, the transfer of its body included.
	requestTimeout = time.Minute
	// maxErrorMessage is how many bytes of an error response go into a StatusError.
	maxErrorMessage = 200
)

// New returns the HTTP client for the servers. It keeps a connection open for each page
// fetched at once and each cover loaded, where Go's default keeps 2 and closes the others
// after each request.
func New() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = maxIdleConnections
	return &http.Client{Timeout: requestTimeout, Transport: transport}
}

// StatusError is the error of a request that a server answered with a status other than 2xx.
// Find it with errors.As to tell, for example, a missing cover (404) or a wrong password (401)
// from a network error.
type StatusError struct {
	Method  string
	Path    string // the path asked for, after the server's address
	Code    int    // the HTTP status code, e.g. 404
	Status  string // the HTTP status, e.g. "404 Not Found"
	Message string // the start of the response's body
}

func (err *StatusError) Error() string {
	return fmt.Sprintf("%s %s: %s %s", err.Method, err.Path, err.Status, err.Message)
}

// Do sends request through httpClient. It returns the response if its status is 2xx, else a
// *StatusError naming the request by path, the part of its URL after the server's address. The
// caller closes the response's body.
func Do(httpClient *http.Client, request *http.Request, path string) (*http.Response, error) {
	span := timing.Start(request.Context(), "http request")
	response, err := httpClient.Do(request) //nolint:gosec // G704: requests go to the servers of config.json
	span.End()
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		defer response.Body.Close()
		// The message only helps to understand the error: one that cannot be read stays empty.
		message, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorMessage))
		return nil, &StatusError{
			Method:  request.Method,
			Path:    path,
			Code:    response.StatusCode,
			Status:  response.Status,
			Message: strings.TrimSpace(string(message)),
		}
	}
	return response, nil
}

// GetJSON sends request, as Do, and decodes its JSON response into value.
func GetJSON(httpClient *http.Client, request *http.Request, path string, value any) error {
	response, err := Do(httpClient, request, path)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	defer timing.Start(request.Context(), "http body").End()
	if err := json.NewDecoder(response.Body).Decode(value); err != nil {
		return fmt.Errorf("%s %s: %w", request.Method, path, err)
	}
	return nil
}

// GetBytes sends request, as Do, and returns the body of its response.
func GetBytes(httpClient *http.Client, request *http.Request, path string) ([]byte, error) {
	response, err := Do(httpClient, request, path)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	defer timing.Start(request.Context(), "http body").End()
	return io.ReadAll(response.Body)
}
