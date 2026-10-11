// Package fakemealie is a Mealie server with fixed recipes and generated photos, for running
// the app off the reader (the emulator) without a real server. Its content is always the same.
package fakemealie

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// The sizes of the photos, about Mealie's.
var (
	originalSize  = image.Pt(1200, 800)
	thumbnailSize = image.Pt(600, 400)
)

// jpegQuality is the quality of the photos. Mealie sends WebP, which Go cannot encode; the
// app's decoders take either.
const jpegQuality = 85

// photoFontSize is the size of the name on a full-size photo, in pixels.
const photoFontSize = 90

// server is the state of a fake Mealie.
type server struct {
	mu     sync.Mutex        // guards photos
	photos map[string][]byte // encoded photos, by recipe id and file name
}

// summary is a recipe as Mealie lists them: without its ingredients and steps.
type summary struct {
	ID             string     `json:"id"`
	Slug           string     `json:"slug"`
	Name           string     `json:"name"`
	Image          *string    `json:"image"`
	RecipeCategory []category `json:"recipeCategory"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// NewHandler returns the handler of a fake Mealie. Any API token is accepted, but one is
// required.
func NewHandler() http.Handler {
	fake := &server{photos: map[string][]byte{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/recipes", listRecipes)
	mux.HandleFunc("GET /api/recipes/{slug}", getRecipe)
	mux.HandleFunc("GET /api/media/recipes/{id}/images/{file}", fake.photo)
	return requireToken(mux)
}

// New starts a fake Mealie, as NewHandler serves it.
func New() *httptest.Server { return httptest.NewServer(NewHandler()) }

// requireToken answers 401 to the requests without a bearer token, as Mealie does.
func requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		isMedia := strings.HasPrefix(request.URL.Path, "/api/media/") // public on Mealie
		if !isMedia && !strings.HasPrefix(request.Header.Get("Authorization"), "Bearer ") {
			http.Error(writer, `{"detail":"Not authenticated"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// listRecipes sends all the recipes, by name, in one page.
func listRecipes(writer http.ResponseWriter, request *http.Request) {
	items := make([]summary, len(catalog))
	for index, item := range catalog {
		items[index] = summary{
			ID: item.ID, Slug: item.Slug, Name: item.Name, Image: item.Image,
			RecipeCategory: item.RecipeCategory, UpdatedAt: item.UpdatedAt,
		}
	}
	writeJSON(writer, map[string]any{"page": 1, "per_page": len(items), "total": len(items), "total_pages": 1, "items": items})
}

// getRecipe sends a whole recipe.
func getRecipe(writer http.ResponseWriter, request *http.Request) {
	for _, item := range catalog {
		if item.Slug == request.PathValue("slug") {
			writeJSON(writer, item)
			return
		}
	}
	http.NotFound(writer, request)
}

// photo sends a recipe's photo, original.webp or min-original.webp, drawn the first time.
func (fake *server) photo(writer http.ResponseWriter, request *http.Request) {
	id, file := request.PathValue("id"), request.PathValue("file")
	size := map[string]image.Point{"original.webp": originalSize, "min-original.webp": thumbnailSize}[file]
	var name string
	for _, item := range catalog {
		if item.ID == id && item.Image != nil {
			name = item.Name
		}
	}
	if name == "" || size == (image.Point{}) {
		http.NotFound(writer, request)
		return
	}
	fake.mu.Lock()
	data, ok := fake.photos[id+"/"+file]
	if !ok {
		var err error
		if data, err = drawPhoto(name, size); err != nil {
			fake.mu.Unlock()
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		fake.photos[id+"/"+file] = data
	}
	fake.mu.Unlock()
	writer.Header().Set("Content-Type", "image/jpeg")
	// An error here means that the client has gone: there is no one left to tell.
	_, _ = writer.Write(data)
}

// drawPhoto draws a stand-in photo of a recipe: its name on a diagonal gradient, as a JPEG.
func drawPhoto(name string, size image.Point) ([]byte, error) {
	canvas := image.NewRGBA(image.Rectangle{Max: size})
	for row := range size.Y {
		for column := range size.X {
			shade := uint8(90 + 120*(column+row)/(size.X+size.Y))
			canvas.SetRGBA(column, row, color.RGBA{R: shade, G: shade - 30, B: shade - 60, A: 255})
		}
	}
	face, err := newFace(photoFontSize * float64(size.X) / float64(originalSize.X))
	if err != nil {
		return nil, err
	}
	drawer := font.Drawer{Dst: canvas, Src: image.White, Face: face}
	width := drawer.MeasureString(name)
	drawer.Dot = fixed.Point26_6{X: (fixed.I(size.X) - width) / 2, Y: fixed.I(size.Y / 2)}
	drawer.DrawString(name)
	var encoded bytes.Buffer
	err = jpeg.Encode(&encoded, canvas, &jpeg.Options{Quality: jpegQuality})
	return encoded.Bytes(), err
}

// newFace returns Go Bold at size pixels.
func newFace(size float64) (font.Face, error) {
	parsed, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}
	// At 72 DPI, a point is a pixel.
	return opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

// writeJSON sends value as JSON.
func writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	// An error here means that the client has gone: there is no one left to tell.
	_ = json.NewEncoder(writer).Encode(value)
}
