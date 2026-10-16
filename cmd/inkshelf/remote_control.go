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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook/script"
	"github.com/CedricHerzog/inkshelf/internal/timing"
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

// A profile started by request lasts defaultCaptureTime unless the request says, and at most
// maxCaptureTime: a trace grows by megabytes a minute.
const (
	defaultCaptureTime = 90 * time.Second
	maxCaptureTime     = 10 * time.Minute
)

// Limits on what a request may send or get, and how long its headers may take to come.
const (
	headerTimeout = 10 * time.Second
	maxScriptSize = 1 << 20
	maxLogChunk   = 1 << 20
	maxAppSize    = 64 << 20
)

// An uploaded app must be a 32-bit little-endian ARM ELF executable, as make build makes it:
// the four bytes of elfMagic, then elfClass32 and elfLittleEndian, and elfMachineARM at
// elfMachineOffset.
const (
	elfMagic         = "\x7fELF"
	elfClass32       = 1
	elfLittleEndian  = 1
	elfMachineOffset = 18
	elfMachineARM    = 40
	elfHeaderSize    = 52
)

// startedHeader is the reply header that says when the app started.
const startedHeader = "X-Started"

// restarting is set when an uploaded app replaced this one: main execs it once the app has
// closed.
var restarting atomic.Bool

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
		appPath:    appPath,
		started:    time.Now(),
		capture:    timing.Capture,
		restart: func() {
			restarting.Store(true)
			pocketbook.Exit()
		},
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
	Battery() (percent int, charging, ok bool)
}

// remoteServer answers remote control requests.
type remoteServer struct {
	token      string
	reader     remoteReader
	work       script.BackgroundWork
	logPath    string
	profileDir string
	// appPath is the app's program, which an upload replaces; restart then closes the app and
	// starts the new one.
	appPath string
	restart func()
	// started is when this app started, sent with every reply as startedHeader: a new value
	// tells that the app restarted.
	started time.Time
	// capture starts profiling, as timing.Capture does.
	capture func(dir string, limit time.Duration) (stop func(), err error)

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
	routes.HandleFunc("GET /status", server.status)
	routes.HandleFunc("GET /panel.png", server.panel)
	routes.HandleFunc("POST /script", server.runScript)
	routes.HandleFunc("GET /shots/{name}", server.shot)
	routes.HandleFunc("GET /log", server.log)
	routes.HandleFunc("GET /stats", server.stats)
	routes.HandleFunc("POST /profile", server.startProfile)
	routes.HandleFunc("GET /profile/", server.profileFiles)
	routes.HandleFunc("GET /profile/{name}", server.profileFile)
	routes.HandleFunc("PUT /app", server.replaceApp)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		given, bearer := strings.CutPrefix(request.Header.Get("Authorization"), "Bearer ")
		if !bearer || subtle.ConstantTimeCompare([]byte(given), []byte(server.token)) != 1 {
			http.Error(writer, "wrong or missing token", http.StatusUnauthorized)
			return
		}
		writer.Header().Set(startedHeader, server.started.Format(time.RFC3339Nano))
		routes.ServeHTTP(writer, request)
	})
}

// remoteStatus is the reply to a status request.
type remoteStatus struct {
	// Ready is whether the app has opened the reader.
	Ready bool `json:"ready"`
	// LogSize is the log's length in bytes: where what happens next will be logged.
	LogSize int64 `json:"logSize"`
}

// status says whether the app has opened the reader and how long the log is. Its
// startedHeader, as every reply's, tells whether the app restarted.
func (server *remoteServer) status(writer http.ResponseWriter, request *http.Request) {
	reply := remoteStatus{Ready: server.reader.Ready()}
	if info, err := os.Stat(server.logPath); err == nil {
		reply.LogSize = info.Size()
	}
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(reply); err != nil {
		log.Printf("remote control: status: %v", err)
	}
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

// remoteStats is a sample of what the app costs, for watching its CPU, memory and battery over
// time.
type remoteStats struct {
	At time.Time `json:"at"`
	// The Go heap: in use, obtained from the system, allocated since start; and the garbage
	// collections, with their total pause.
	HeapAlloc    uint64 `json:"heapAlloc"`
	HeapSys      uint64 `json:"heapSys"`
	TotalAlloc   uint64 `json:"totalAlloc"`
	NumGC        uint32 `json:"numGC"`
	PauseTotalNs uint64 `json:"pauseTotalNs"`
	Goroutines   int    `json:"goroutines"`
	// UserTicks and SystemTicks are the CPU time the process used since it started, in clock
	// ticks (100 a second on the reader), from /proc/self/stat.
	UserTicks   int64  `json:"userTicks"`
	SystemTicks int64  `json:"systemTicks"`
	LoadAverage string `json:"loadAverage"`
	Battery     int    `json:"battery"`
	Charging    bool   `json:"charging"`
}

// stats sends a remoteStats. Reading the memory stats stops the program for a moment: sample
// it every few seconds at most.
func (server *remoteServer) stats(writer http.ResponseWriter, request *http.Request) {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	sample := remoteStats{
		At:           time.Now(),
		HeapAlloc:    memory.HeapAlloc,
		HeapSys:      memory.HeapSys,
		TotalAlloc:   memory.TotalAlloc,
		NumGC:        memory.NumGC,
		PauseTotalNs: memory.PauseTotalNs,
		Goroutines:   runtime.NumGoroutine(),
	}
	sample.UserTicks, sample.SystemTicks = cpuTicks()
	if loadAverage, err := os.ReadFile("/proc/loadavg"); err == nil {
		sample.LoadAverage = strings.TrimSpace(string(loadAverage))
	}
	sample.Battery, sample.Charging, _ = server.reader.Battery()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(sample); err != nil {
		log.Printf("remote control: stats: %v", err)
	}
}

// The fields of /proc/self/stat after the command's name, which ends with ")": the state is
// field 3 of the whole line, utime field 14 and stime field 15.
const (
	statUserTimeField   = 14 - 3
	statSystemTimeField = 15 - 3
)

// cpuTicks returns the CPU time this process used, in clock ticks, or zeros where there is no
// /proc (off Linux).
func cpuTicks() (user, system int64) {
	contents, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, 0
	}
	_, afterName, ok := strings.Cut(string(contents), ") ")
	fields := strings.Fields(afterName)
	if !ok || len(fields) <= statSystemTimeField {
		return 0, 0
	}
	user, _ = strconv.ParseInt(fields[statUserTimeField], 10, 64)
	system, _ = strconv.ParseInt(fields[statSystemTimeField], 10, 64)
	return user, system
}

// startProfile starts profiling into the profile directory for the duration of the for
// parameter ("90s"; defaultCaptureTime if missing). GET /profile/ lists the files once it is
// over. One capture runs at a time: the Go runtime has one CPU profile.
func (server *remoteServer) startProfile(writer http.ResponseWriter, request *http.Request) {
	limit := defaultCaptureTime
	if text := request.URL.Query().Get("for"); text != "" {
		var err error
		if limit, err = time.ParseDuration(text); err != nil || limit <= 0 || limit > maxCaptureTime {
			http.Error(writer, fmt.Sprintf("bad duration %q: want up to %v", text, maxCaptureTime), http.StatusBadRequest)
			return
		}
	}
	if _, err := server.capture(server.profileDir, limit); err != nil {
		http.Error(writer, err.Error(), http.StatusConflict)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(map[string]string{"profiling": limit.String()}); err != nil {
		log.Printf("remote control: profile: %v", err)
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

// replaceApp replaces the app's program with the request's body, then restarts the app. The
// program is written next to it, checked and synced first, then renamed over it: the running
// app keeps its own copy open, and a failed upload leaves the old program in place.
func (server *remoteServer) replaceApp(writer http.ResponseWriter, request *http.Request) {
	program, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, maxAppSize))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := checkARMExecutable(program); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := writeFileSynced(server.appPath+".new", program); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.Rename(server.appPath+".new", server.appPath); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("remote control: new app (%d bytes), restarting", len(program))
	fmt.Fprintln(writer, "restarting")
	// The reply must leave before the app closes.
	if err := http.NewResponseController(writer).Flush(); err != nil {
		log.Printf("remote control: %v", err)
	}
	server.restart()
}

// checkARMExecutable reports an error unless program looks like what make build makes.
func checkARMExecutable(program []byte) error {
	if len(program) < elfHeaderSize || string(program[:len(elfMagic)]) != elfMagic {
		return errors.New("not an ELF executable")
	}
	machine := int(program[elfMachineOffset]) | int(program[elfMachineOffset+1])<<8
	if program[4] != elfClass32 || program[5] != elfLittleEndian || machine != elfMachineARM {
		return errors.New("not a 32-bit ARM executable: run make build")
	}
	return nil
}

// writeFileSynced writes contents to path and syncs it to the storage, which the reader may
// lose power before writing back.
func writeFileSynced(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// execNewApp replaces this process with the app at appPath, once the app has closed after an
// upload. The process keeps its id and group, by which the firmware knows the app's task.
func execNewApp() error {
	log.Printf("remote control: starting %s", appPath)
	return syscall.Exec(appPath, os.Args, os.Environ()) //nolint:gosec // appPath is the app's own, fixed path
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
