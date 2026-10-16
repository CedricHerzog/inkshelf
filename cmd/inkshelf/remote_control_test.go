package main

import (
	"encoding/json"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
)

const testToken = "0123456789abcdef"

// fakeReader is a reader under remote control that records the input it gets, and whose panel
// is gray.
type fakeReader struct {
	mu      sync.Mutex
	notOpen bool
	keys    []pocketbook.Key
}

func (reader *fakeReader) Ready() bool               { return !reader.notOpen }
func (reader *fakeReader) Size() image.Point         { return image.Pt(1404, 1872) }
func (reader *fakeReader) PointerDown(image.Point)   {}
func (reader *fakeReader) PointerMove(image.Point)   {}
func (reader *fakeReader) PointerUp(image.Point)     {}
func (reader *fakeReader) ReleaseKey(pocketbook.Key) {}
func (reader *fakeReader) LastRefresh() time.Time    { return time.Time{} }
func (reader *fakeReader) Pending() bool             { return false }
func (reader *fakeReader) RequestExit()              {}
func (reader *fakeReader) Panel() *image.Gray        { return image.NewGray(image.Rect(0, 0, 4, 4)) }
func (reader *fakeReader) PressKey(key pocketbook.Key) {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	reader.keys = append(reader.keys, key)
}

// newTestServer returns a remote control server for reader, its log and profiles in a
// temporary directory.
func newTestServer(t *testing.T, reader *fakeReader) (*httptest.Server, *remoteServer) {
	dir := t.TempDir()
	server := &remoteServer{
		token:      testToken,
		reader:     reader,
		work:       func() ([]string, time.Time) { return nil, time.Time{} },
		logPath:    filepath.Join(dir, "inkshelf.log"),
		profileDir: filepath.Join(dir, "profile"),
	}
	httpServer := httptest.NewServer(server.handler())
	t.Cleanup(httpServer.Close)
	return httpServer, server
}

// send sends a request with the token and returns the response's status, headers and body.
func send(t *testing.T, method, url, body string) (int, http.Header, string) {
	t.Helper()
	request, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+testToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	received, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, response.Header, string(received)
}

func TestRemoteControlNeedsTheToken(t *testing.T) {
	httpServer, _ := newTestServer(t, &fakeReader{})
	for _, header := range []string{"", "Bearer wrong", testToken} {
		request, err := http.NewRequest(http.MethodGet, httpServer.URL+"/panel.png", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		if header != "" {
			request.Header.Set("Authorization", header)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Errorf("Authorization %q: status %d, want 401", header, response.StatusCode)
		}
	}
	if status, headers, _ := send(t, http.MethodGet, httpServer.URL+"/panel.png", ""); status != http.StatusOK ||
		headers.Get("Content-Type") != "image/png" {
		t.Errorf("with the token: status %d, type %q, want a PNG", status, headers.Get("Content-Type"))
	}
}

func TestRemoteControlScript(t *testing.T) {
	reader := &fakeReader{}
	httpServer, _ := newTestServer(t, reader)

	status, _, body := send(t, http.MethodPost, httpServer.URL+"/script", "key next\nshot first\nkey prev\nshot second\n")
	var result scriptResult
	if err := json.Unmarshal([]byte(body), &result); status != http.StatusOK || err != nil {
		t.Fatalf("status %d, body %q", status, body)
	}
	if result.Error != "" || !slices.Equal(result.Shots, []string{"first", "second"}) {
		t.Errorf("result = %+v, want shots first and second", result)
	}
	if want := []pocketbook.Key{pocketbook.KeyNext, pocketbook.KeyPrev}; !slices.Equal(reader.keys, want) {
		t.Errorf("keys = %v, want %v", reader.keys, want)
	}
	if status, _, _ := send(t, http.MethodGet, httpServer.URL+"/shots/second.png", ""); status != http.StatusOK {
		t.Errorf("shot second: status %d", status)
	}

	// A command of the emulator only fails the script, which then has no shots.
	_, _, body = send(t, http.MethodPost, httpServer.URL+"/script", "shot again\nhome\n")
	result = scriptResult{}
	if err := json.Unmarshal([]byte(body), &result); err != nil || !strings.Contains(result.Error, "emulator") ||
		len(result.Shots) != 0 {
		t.Errorf("result = %+v, want an error about the emulator", result)
	}
	if status, _, _ := send(t, http.MethodGet, httpServer.URL+"/shots/second.png", ""); status != http.StatusNotFound {
		t.Errorf("the last script's shot: status %d, want 404", status)
	}

	if status, _, _ := send(t, http.MethodPost, httpServer.URL+"/script", "jump 1 2"); status != http.StatusBadRequest {
		t.Errorf("a bad script: status %d, want 400", status)
	}
}

func TestRemoteControlNotReady(t *testing.T) {
	httpServer, _ := newTestServer(t, &fakeReader{notOpen: true})
	if status, _, _ := send(t, http.MethodPost, httpServer.URL+"/script", "key next"); status != http.StatusServiceUnavailable {
		t.Errorf("status %d, want 503", status)
	}
}

func TestRemoteControlLog(t *testing.T) {
	httpServer, server := newTestServer(t, &fakeReader{})
	if err := os.WriteFile(server.logPath, []byte("first line\nsecond line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		offset     string
		wantOffset string
		want       string
	}{
		{"", "0", "first line\nsecond line\n"},
		{"11", "11", "second line\n"},
		{"23", "23", ""},
		{"500", "0", "first line\nsecond line\n"}, // truncated since: start over
	} {
		status, headers, body := send(t, http.MethodGet, httpServer.URL+"/log?offset="+test.offset, "")
		if status != http.StatusOK || body != test.want || headers.Get("X-Log-Offset") != test.wantOffset ||
			headers.Get("X-Log-Size") != "23" {
			t.Errorf("offset %q: status %d, offset %s, size %s, body %q", test.offset, status,
				headers.Get("X-Log-Offset"), headers.Get("X-Log-Size"), body)
		}
	}
	if status, _, _ := send(t, http.MethodGet, httpServer.URL+"/log?offset=-1", ""); status != http.StatusBadRequest {
		t.Errorf("negative offset: status %d, want 400", status)
	}
}

func TestRemoteControlProfile(t *testing.T) {
	httpServer, server := newTestServer(t, &fakeReader{})
	if status, _, body := send(t, http.MethodGet, httpServer.URL+"/profile/", ""); status != http.StatusOK || body != "" {
		t.Errorf("no profile yet: status %d, body %q", status, body)
	}
	if err := os.MkdirAll(server.profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(server.profileDir, "cpu.pprof"), []byte("profile"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(server.logPath, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, body := send(t, http.MethodGet, httpServer.URL+"/profile/", ""); body != "cpu.pprof\n" {
		t.Errorf("list = %q, want cpu.pprof", body)
	}
	if status, _, body := send(t, http.MethodGet, httpServer.URL+"/profile/cpu.pprof", ""); status != http.StatusOK || body != "profile" {
		t.Errorf("cpu.pprof: status %d, body %q", status, body)
	}
	for _, name := range []string{"..%2Finkshelf.log", "%2E%2E"} {
		if status, _, body := send(t, http.MethodGet, httpServer.URL+"/profile/"+name, ""); status == http.StatusOK {
			t.Errorf("%s: status 200, body %q, want it refused", name, body)
		}
	}
}
