package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook/script"
)

// The reader is remote controlled when remoteMarker exists in the app's directory, holding a
// token of at least minTokenLength characters: a computer on the same network can then run
// scripts on the reader, see its panel and read the log, over HTTP on remotePort
// (docs/reference/remote-control.md). Every request carries the token.
const (
	remoteMarker   = "remote"
	remotePort     = "8740"
	minTokenLength = 16
)

// While remote controlled, the reader is kept awake: a script sends no real input, and the
// reader would otherwise fall asleep and drop Wi-Fi. Every awakeInterval it asks for
// awakeInterval and a margin more.
const (
	awakeInterval = time.Minute
	awakeMargin   = 30 * time.Second
)

// Limits on what a request may send or get, and how long its headers may take to come.
const (
	headerTimeout = 10 * time.Second
	maxScriptSize = 1 << 20
	maxLogChunk   = 1 << 20
)

// remoteToken returns the token of the marker file, or "" when the reader is not remote
// controlled.
func remoteToken() string {
	contents, err := os.ReadFile(filepath.Join(appDir, remoteMarker))
	if err != nil {
		return ""
	}
	token := strings.TrimSpace(string(contents))
	if len(token) < minTokenLength {
		log.Printf("remote control off: the token in %s has fewer than %d characters", remoteMarker, minTokenLength)
		return ""
	}
	return token
}

// startRemoteControl serves remote control requests for remote, connects to Wi-Fi and keeps
// the reader awake, for as long as the app runs. work is the app's BackgroundWork.
func startRemoteControl(token string, remote *pocketbook.RemoteControl, work script.BackgroundWork) {
	server := &remoteServer{
		token:      token,
		reader:     remote,
		work:       work,
		logPath:    logPath,
		profileDir: profileDir,
	}
	listener, err := new(net.ListenConfig).Listen(context.Background(), "tcp", ":"+remotePort)
	if err != nil {
		log.Printf("remote control off: %v", err)
		return
	}
	// No write timeout: a script replies when it is over, which may take minutes.
	httpServer := &http.Server{Handler: server.handler(), ReadHeaderTimeout: headerTimeout}
	go func() {
		err := httpServer.Serve(listener)
		log.Printf("remote control stopped: %v", err)
	}()
	go func() {
		// The app's own connections then find Wi-Fi on, and leave it on when they end.
		if _, err := pocketbook.KeepWiFiOn(context.Background()); err != nil {
			log.Printf("remote control: %v", err)
		}
		log.Printf("remote control on %s", strings.Join(remoteURLs(), ", "))
	}()
	go func() {
		for {
			pocketbook.PreventSleepFor(awakeInterval + awakeMargin)
			time.Sleep(awakeInterval)
		}
	}()
}

// remoteURLs returns the addresses the server can be reached at, for the log.
func remoteURLs() []string {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return []string{err.Error()}
	}
	var urls []string
	for _, address := range addresses {
		if network, ok := address.(*net.IPNet); ok && !network.IP.IsLoopback() && network.IP.To4() != nil {
			urls = append(urls, "http://"+net.JoinHostPort(network.IP.String(), remotePort))
		}
	}
	return urls
}

// remoteReader is the reader under remote control: a pocketbook.RemoteControl.
type remoteReader interface {
	script.Target
	Ready() bool
}

// remoteServer answers remote control requests.
type remoteServer struct {
	token      string
	reader     remoteReader
	work       script.BackgroundWork
	logPath    string
	profileDir string

	// scripts lets one script run at a time: two would mix their input.
	scripts sync.Mutex
	// mu guards shots.
	mu sync.Mutex
	// shots are the PNGs of the last script's shots, by name.
	shots map[string][]byte
}

// handler returns the server's routes, behind the token check.
func (server *remoteServer) handler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET /panel.png", server.panel)
	routes.HandleFunc("POST /script", server.runScript)
	routes.HandleFunc("GET /shots/{name}", server.shot)
	routes.HandleFunc("GET /log", server.log)
	routes.HandleFunc("GET /profile/", server.profileFiles)
	routes.HandleFunc("GET /profile/{name}", server.profileFile)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		given, bearer := strings.CutPrefix(request.Header.Get("Authorization"), "Bearer ")
		if !bearer || subtle.ConstantTimeCompare([]byte(given), []byte(server.token)) != 1 {
			http.Error(writer, "wrong or missing token", http.StatusUnauthorized)
			return
		}
		routes.ServeHTTP(writer, request)
	})
}

// panel sends what the panel shows, as a PNG.
func (server *remoteServer) panel(writer http.ResponseWriter, request *http.Request) {
	if !server.ready(writer) {
		return
	}
	encoded, err := encodePNG(server.reader.Panel())
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writePNG(writer, encoded)
}

// scriptResult is the reply to a script: the shots taken, in order, and the error that stopped
// it, if any.
type scriptResult struct {
	Shots []string `json:"shots"`
	Error string   `json:"error,omitempty"`
}

// runScript runs the script in the request's body and replies with a scriptResult. Its shots
// replace the last script's.
func (server *remoteServer) runScript(writer http.ResponseWriter, request *http.Request) {
	if !server.ready(writer) {
		return
	}
	source, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, maxScriptSize))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	steps, err := script.Parse(string(source))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	server.scripts.Lock()
	defer server.scripts.Unlock()
	server.mu.Lock()
	server.shots = map[string][]byte{}
	server.mu.Unlock()
	result := scriptResult{Shots: []string{}}
	err = script.Run(server.reader, server.work, steps, func(name string, panel *image.Gray) error {
		encoded, err := encodePNG(panel)
		if err != nil {
			return err
		}
		server.mu.Lock()
		server.shots[name] = encoded
		server.mu.Unlock()
		result.Shots = append(result.Shots, name)
		log.Printf("script: shot %s", name)
		return nil
	})
	if err != nil {
		result.Error = err.Error()
		log.Printf("script: %v", err)
	}
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(result); err != nil {
		log.Printf("script: reply: %v", err)
	}
}

// shot sends a shot of the last script.
func (server *remoteServer) shot(writer http.ResponseWriter, request *http.Request) {
	name, _ := strings.CutSuffix(request.PathValue("name"), ".png")
	server.mu.Lock()
	encoded, ok := server.shots[name]
	server.mu.Unlock()
	if !ok {
		http.NotFound(writer, request)
		return
	}
	writePNG(writer, encoded)
}

// log sends the log from the byte at the offset parameter (default 0), up to maxLogChunk
// bytes. X-Log-Offset says where the bytes sent start, X-Log-Size how long the log is: ask
// again from offset plus the bytes received to follow it. If the log is shorter than the
// offset, it was truncated, and the bytes sent start over from 0.
func (server *remoteServer) log(writer http.ResponseWriter, request *http.Request) {
	offset := int64(0)
	if text := request.URL.Query().Get("offset"); text != "" {
		var err error
		if offset, err = strconv.ParseInt(text, 10, 64); err != nil || offset < 0 {
			http.Error(writer, "bad offset "+text, http.StatusBadRequest)
			return
		}
	}
	file, err := os.Open(server.logPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusNotFound)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if offset > info.Size() {
		offset = 0
	}
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.Header().Set("X-Log-Offset", strconv.FormatInt(offset, 10))
	writer.Header().Set("X-Log-Size", strconv.FormatInt(info.Size(), 10))
	if _, err := io.Copy(writer, io.NewSectionReader(file, offset, min(info.Size()-offset, maxLogChunk))); err != nil {
		log.Printf("remote control: sending the log: %v", err)
	}
}

// profileFiles lists the files of the profile directory, one name a line.
func (server *remoteServer) profileFiles(writer http.ResponseWriter, request *http.Request) {
	entries, err := os.ReadDir(server.profileDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			fmt.Fprintln(writer, entry.Name())
		}
	}
}

// profileFile sends a file of the profile directory. It is opened through an os.Root, so that
// no name can reach a file outside the directory.
func (server *remoteServer) profileFile(writer http.ResponseWriter, request *http.Request) {
	root, err := os.OpenRoot(server.profileDir)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusNotFound)
		return
	}
	defer root.Close()
	file, err := root.Open(request.PathValue("name"))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusNotFound)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.Error(writer, "not a file", http.StatusNotFound)
		return
	}
	http.ServeContent(writer, request, info.Name(), info.ModTime(), file)
}

// ready reports whether the reader is open, and replies with an error if not.
func (server *remoteServer) ready(writer http.ResponseWriter) bool {
	if server.reader.Ready() {
		return true
	}
	http.Error(writer, "the app is still starting", http.StatusServiceUnavailable)
	return false
}

// encodePNG returns img as a PNG file.
func encodePNG(img image.Image) ([]byte, error) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

// writePNG sends a PNG file.
func writePNG(writer http.ResponseWriter, encoded []byte) {
	writer.Header().Set("Content-Type", "image/png")
	if _, err := writer.Write(encoded); err != nil {
		log.Printf("remote control: sending a PNG: %v", err)
	}
}
