package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"log"
	"net/http"

	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
)

//go:embed index.html
var indexPage []byte

// waveformNames name the waveforms for the web page.
var waveformNames = map[pocketbook.Waveform]string{
	pocketbook.DU: "DU", pocketbook.GC16: "GC16", pocketbook.GC4: "GC4", pocketbook.A2: "A2",
	pocketbook.GL16: "GL16", pocketbook.DU4: "DU4",
}

// server is the web page's backend: it shows the panel and feeds the emulator input.
type server struct {
	emulator *pocketbook.Emulator
}

// newServer returns the web page's handler for emulator.
func newServer(emulator *pocketbook.Emulator) http.Handler {
	backend := &server{emulator: emulator}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := writer.Write(indexPage); err != nil {
			log.Printf("web page: %v", err)
		}
	})
	mux.HandleFunc("GET /panel.png", backend.panel)
	mux.HandleFunc("GET /refreshes", backend.refreshes)
	mux.HandleFunc("POST /pointer", backend.pointer)
	mux.HandleFunc("POST /key", backend.key)
	mux.HandleFunc("POST /control", backend.control)
	return mux
}

// panel sends what the panel shows, as a PNG image.
func (backend *server) panel(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "image/png")
	writer.Header().Set("Cache-Control", "no-store")
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(writer, backend.emulator.Panel()); err != nil {
		log.Printf("panel.png: %v", err)
	}
}

// refreshes streams the panel's refreshes as server-sent events.
func (backend *server) refreshes(writer http.ResponseWriter, request *http.Request) {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		http.Error(writer, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-store")
	refreshes, cancel := backend.emulator.Subscribe()
	defer cancel()
	fmt.Fprint(writer, ": connected\n\n")
	flusher.Flush()
	for {
		select {
		case <-request.Context().Done():
			return
		case refresh := <-refreshes:
			data, err := json.Marshal(map[string]any{
				"x":        refresh.Rect.Min.X,
				"y":        refresh.Rect.Min.Y,
				"width":    refresh.Rect.Dx(),
				"height":   refresh.Rect.Dy(),
				"waveform": waveformNames[refresh.Waveform],
				"full":     refresh.Full,
			})
			if err != nil {
				log.Printf("refreshes: %v", err)
				continue
			}
			fmt.Fprintf(writer, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// decode decodes the request's JSON body into value. If it can't, it answers with an error
// and returns false.
func decode(writer http.ResponseWriter, request *http.Request, value any) bool {
	if err := json.NewDecoder(request.Body).Decode(value); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

// pointer passes a touch on to the emulator.
func (backend *server) pointer(writer http.ResponseWriter, request *http.Request) {
	var touch struct {
		State string `json:"state"`
		X, Y  int
	}
	if !decode(writer, request, &touch) {
		return
	}
	point := image.Pt(touch.X, touch.Y)
	switch touch.State {
	case "down":
		backend.emulator.PointerDown(point)
	case "move":
		backend.emulator.PointerMove(point)
	case "up":
		backend.emulator.PointerUp(point)
	default:
		http.Error(writer, "unknown state "+touch.State, http.StatusBadRequest)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// key passes a button press or release on to the emulator.
func (backend *server) key(writer http.ResponseWriter, request *http.Request) {
	var press struct {
		Key   string `json:"key"`
		State string `json:"state"`
	}
	if !decode(writer, request, &press) {
		return
	}
	key, ok := keyNames[press.Key]
	switch {
	case !ok:
		http.Error(writer, "unknown key "+press.Key, http.StatusBadRequest)
		return
	case press.State == "press":
		backend.emulator.PressKey(key)
	case press.State == "release":
		backend.emulator.ReleaseKey(key)
	default:
		http.Error(writer, "unknown state "+press.State, http.StatusBadRequest)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// control passes the page's other controls on to the emulator: Home, battery, network, exit.
func (backend *server) control(writer http.ResponseWriter, request *http.Request) {
	var command struct {
		Action   string `json:"action"`
		Percent  int    `json:"percent"`
		Charging bool   `json:"charging"`
	}
	if !decode(writer, request, &command) {
		return
	}
	switch command.Action {
	case "home":
		backend.emulator.SetForeground(false)
	case "resume":
		backend.emulator.SetForeground(true)
	case "battery":
		backend.emulator.SetBattery(min(max(command.Percent, 0), 100), command.Charging)
	case "net-up":
		setNetwork(backend.emulator, true)
	case "net-down":
		setNetwork(backend.emulator, false)
	case "exit":
		backend.emulator.RequestExit()
	default:
		http.Error(writer, "unknown action "+command.Action, http.StatusBadRequest)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}
