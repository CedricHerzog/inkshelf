// Package fakekomga is a Komga server with generated series, books and pages, for running the
// app off the reader (the emulator) without a real server. Its content is always the same.
package fakekomga

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
)

// server is the state of a fake Komga.
type server struct {
	mu       sync.Mutex
	progress map[string]progress // by book id
	images   map[string][]byte   // encoded pages and covers, by imageKey or coverKey
	// pageFace and coverFace draw the text of pages and covers. A face is not safe for
	// concurrent use: mu guards them too.
	pageFace  font.Face
	coverFace font.Face
}

// progress is how far a book has been read, as Komga sends and receives it.
type progress struct {
	Page      int       `json:"page"`
	Completed bool      `json:"completed"`
	at        time.Time // when it was set; zero for the starting progress
}

// The sizes of the text on pages and covers, in pixels.
const (
	pageFontSize  = 90
	coverFontSize = 44
)

// NewHandler returns the handler of a fake Komga. Any API key is accepted.
func NewHandler() http.Handler {
	fake := &server{
		progress:  map[string]progress{},
		images:    map[string][]byte{},
		pageFace:  mustFace(pageFontSize),
		coverFace: mustFace(coverFontSize),
	}
	for _, item := range catalog {
		for book, page := range item.progress {
			fake.progress[bookID(item.id, book)] = progress{Page: page, Completed: page >= item.pages}
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/libraries", listLibraries)
	mux.HandleFunc("GET /api/v1/series", fake.listSeries)
	mux.HandleFunc("GET /api/v1/series/{series}/books", fake.listBooks)
	mux.HandleFunc("GET /api/v1/books", fake.inProgress)
	mux.HandleFunc("GET /api/v1/books/{book}/pages", fake.listPages)
	mux.HandleFunc("GET /api/v1/books/{book}/pages/{number}", fake.page)
	mux.HandleFunc("PATCH /api/v1/books/{book}/read-progress", fake.setProgress)
	mux.HandleFunc("DELETE /api/v1/books/{book}/read-progress", fake.clearProgress)
	mux.HandleFunc("GET /api/v1/series/{series}/thumbnail", fake.seriesThumbnail)
	mux.HandleFunc("GET /api/v1/books/{book}/thumbnail", fake.bookThumbnail)
	return mux
}

// New starts a fake Komga, as NewHandler serves it.
func New() *httptest.Server { return httptest.NewServer(NewHandler()) }

// mustFace returns Go Bold at size pixels. It panics if the font cannot be loaded, which
// would be a bug: the font is compiled in.
func mustFace(size float64) font.Face {
	parsed, err := opentype.Parse(gobold.TTF)
	if err != nil {
		panic(err)
	}
	// At 72 DPI, a point is a pixel.
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	return face
}

// writeJSON sends value as JSON.
func writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	// An error here means that the client has gone: there is no one left to tell.
	_ = json.NewEncoder(writer).Encode(value)
}

// writeImage sends an encoded image of format "jpeg" or "png".
func writeImage(writer http.ResponseWriter, format string, data []byte) {
	writer.Header().Set("Content-Type", "image/"+format)
	// An error here means that the client has gone: there is no one left to tell.
	_, _ = writer.Write(data)
}
