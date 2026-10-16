package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubReader answers as a reader under remote control would, for a script that takes one shot
// and fails after it.
func stubReader(t *testing.T) *reader {
	routes := http.NewServeMux()
	routes.HandleFunc("POST /script", func(writer http.ResponseWriter, request *http.Request) {
		source, _ := io.ReadAll(request.Body)
		if string(source) != "shot one\nhome\n" {
			t.Errorf("script sent = %q", source)
		}
		fmt.Fprint(writer, `{"shots":["one"],"error":"line 2: home works only in the emulator"}`)
	})
	routes.HandleFunc("GET /shots/one.png", func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprint(writer, "png")
	})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer token" {
			http.Error(writer, "no token", http.StatusUnauthorized)
			return
		}
		routes.ServeHTTP(writer, request)
	}))
	t.Cleanup(server.Close)
	return &reader{Address: strings.TrimPrefix(server.URL, "http://"), Token: "token"}
}

func TestRunScriptSavesShotsAndReportsTheError(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("smoke.txt", []byte("shot one\nhome\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := stubReader(t).runScript("smoke.txt")
	if err == nil || !strings.Contains(err.Error(), "home works only in the emulator") {
		t.Errorf("runScript = %v, want the script's error", err)
	}
	saved, readErr := os.ReadFile(filepath.Join(shotsDir, "smoke", "one.png"))
	if readErr != nil || string(saved) != "png" {
		t.Errorf("shot one = %q, %v; want it saved", saved, readErr)
	}
}
