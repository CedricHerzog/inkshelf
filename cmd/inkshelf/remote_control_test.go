package main

import (
	"encoding/json"
	"errors"
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

func (reader *fakeReader) Ready() bool                { return !reader.notOpen }
func (reader *fakeReader) Size() image.Point          { return image.Pt(1404, 1872) }
func (reader *fakeReader) PointerDown(image.Point)    {}
func (reader *fakeReader) PointerMove(image.Point)    {}
func (reader *fakeReader) PointerUp(image.Point)      {}
func (reader *fakeReader) ReleaseKey(pocketbook.Key)  {}
func (reader *fakeReader) LastRefresh() time.Time     { return time.Time{} }
func (reader *fakeReader) Pending() bool              { return false }
func (reader *fakeReader) RequestExit()               {}
func (reader *fakeReader) Battery() (int, bool, bool) { return 42, true, true }
func (reader *fakeReader) Panel() *image.Gray         { return image.NewGray(image.Rect(0, 0, 4, 4)) }
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
		appPath:    filepath.Join(dir, "inkshelf.app"),
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
	httpServer, server := newTestServer(t, &fakeReader{notOpen: true})
	server.started = time.Date(2026, time.October, 16, 9, 0, 0, 0, time.UTC)
	status, headers, body := send(t, http.MethodGet, httpServer.URL+"/status", "")
	if status != http.StatusOK || strings.TrimSpace(body) != `{"ready":false,"logSize":0}` || headers.Get(startedHeader) != "2026-10-16T09:00:00Z" {
		t.Errorf("status: %d, %q, started %q", status, body, headers.Get(startedHeader))
	}
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

// armHeader returns the start of an ELF executable for machine, 32-bit little-endian.
func armHeader(machine byte) string {
	header := make([]byte, elfHeaderSize)
	copy(header, elfMagic)
	header[4], header[5], header[elfMachineOffset] = elfClass32, elfLittleEndian, machine
	return string(header)
}

func TestRemoteControlReplacesTheApp(t *testing.T) {
	httpServer, server := newTestServer(t, &fakeReader{})
	restarts := 0
	server.restart = func() { restarts++ }
	if err := os.WriteFile(server.appPath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	const x86 = 3
	for _, refused := range []string{"", "#!/bin/sh\n", armHeader(x86)} {
		if status, _, _ := send(t, http.MethodPut, httpServer.URL+"/app", refused); status != http.StatusBadRequest {
			t.Errorf("upload of %q: status %d, want 400", refused[:min(len(refused), 8)], status)
		}
	}
	if contents, _ := os.ReadFile(server.appPath); string(contents) != "old" || restarts != 0 {
		t.Fatalf("after refused uploads: app %q, %d restarts; want the old app kept", contents, restarts)
	}

	program := armHeader(elfMachineARM) + "code"
	if status, _, body := send(t, http.MethodPut, httpServer.URL+"/app", program); status != http.StatusOK {
		t.Fatalf("upload: status %d, body %q", status, body)
	}
	contents, err := os.ReadFile(server.appPath)
	if err != nil || string(contents) != program || restarts != 1 {
		t.Errorf("after the upload: app of %d bytes, %d restarts, %v; want the new app and a restart", len(contents), restarts, err)
	}
	if info, err := os.Stat(server.appPath); err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("the new app is not executable: %v", info.Mode())
	}
}

func TestRemoteControlStats(t *testing.T) {
	httpServer, _ := newTestServer(t, &fakeReader{})
	status, _, body := send(t, http.MethodGet, httpServer.URL+"/stats", "")
	var sample remoteStats
	if err := json.Unmarshal([]byte(body), &sample); status != http.StatusOK || err != nil {
		t.Fatalf("status %d, body %q", status, body)
	}
	if sample.Goroutines == 0 || sample.HeapAlloc == 0 || sample.Battery != 42 || !sample.Charging {
		t.Errorf("stats = %+v, want goroutines, a heap and the battery", sample)
	}
}

func TestRemoteControlStartsAProfile(t *testing.T) {
	httpServer, server := newTestServer(t, &fakeReader{})
	var limits []time.Duration
	server.capture = func(dir string, limit time.Duration) (func(), error) {
		if dir != server.profileDir {
			t.Errorf("capture into %s, want %s", dir, server.profileDir)
		}
		limits = append(limits, limit)
		if len(limits) == 3 {
			return nil, errors.New("cpu profiling already in use")
		}
		return func() {}, nil
	}
	for _, test := range []struct {
		query  string
		status int
	}{
		{"", http.StatusOK},
		{"?for=30s", http.StatusOK},
		{"?for=soon", http.StatusBadRequest},
		{"?for=1h", http.StatusBadRequest},
		{"?for=10s", http.StatusConflict}, // the third capture fails
	} {
		if status, _, body := send(t, http.MethodPost, httpServer.URL+"/profile"+test.query, ""); status != test.status {
			t.Errorf("POST /profile%s: status %d (%q), want %d", test.query, status, body, test.status)
		}
	}
	if want := []time.Duration{defaultCaptureTime, 30 * time.Second, 10 * time.Second}; !slices.Equal(limits, want) {
		t.Errorf("captures for %v, want %v", limits, want)
	}
}
