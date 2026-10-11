package httpclient

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A status other than 2xx is a StatusError that names the request by its path.
func TestStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "no such cover", http.StatusNotFound)
	}))
	defer server.Close()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/cover", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	_, err = GetBytes(server.Client(), request, "/cover")
	var statusError *StatusError
	if !errors.As(err, &statusError) || statusError.Code != http.StatusNotFound {
		t.Fatalf("error %v, want a StatusError 404", err)
	}
	if want := "GET /cover: 404 Not Found no such cover"; err.Error() != want {
		t.Errorf("error %q, want %q", err, want)
	}
}

// A 2xx response's JSON is decoded.
func TestGetJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if _, err := writer.Write([]byte(`{"name":"Tome 1"}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/book", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	var book struct{ Name string }
	if err := GetJSON(server.Client(), request, "/book", &book); err != nil || book.Name != "Tome 1" {
		t.Fatalf("GetJSON = %+v, %v", book, err)
	}
}
