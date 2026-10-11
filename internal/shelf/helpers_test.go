package shelf

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"slices"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/servers/servertest"
)

// encodePNG returns img encoded as a PNG.
func encodePNG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

// newTestServer returns a server with one book, b1, of 3 pages: a PNG, a WebP
// (testdata/page.webp) and a PNG, all of the same picture, 200x100, its left half white and
// its right half black; and the cover of series s1. Book alpha-01 has 10 pages. Book c1 is a
// comic book archive of the same 3 pages.
func newTestServer(t testing.TB) *servertest.Fake {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for column := range 200 {
		for row := range 100 {
			shade := color.Gray{Y: 255}
			if column >= 100 { // opaque black: a transparent pixel would be white
				shade.Y = 0
			}
			picture.Set(column, row, shade)
		}
	}
	pictureData := encodePNG(t, picture)
	webpData, err := os.ReadFile("testdata/page.webp")
	if err != nil {
		t.Fatal(err)
	}
	return &servertest.Fake{
		Files: map[string][]byte{"c1": zipFiles(t, map[string][]byte{
			"c1/p1.png": pictureData, "c1/p2.webp": webpData, "c1/p10.png": pictureData, "c1/info.xml": nil,
		})},
		PageImages: map[string][][]byte{
			"b1":       {pictureData, webpData, pictureData},
			"alpha-01": slices.Repeat([][]byte{pictureData}, 10),
		},
		Thumbnails: map[string][]byte{"s1": encodePNG(t, gradient(300, 450))},
	}
}

// gradient returns a gray image whose pixels all differ from their neighbours.
func gradient(width, height int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, width, height))
	for index := range img.Pix {
		img.Pix[index] = uint8(index)
	}
	return img
}

// saveBooks saves books in store as if they had been downloaded.
func saveBooks(t *testing.T, store Store, books ...Book) {
	t.Helper()
	for _, book := range books {
		if err := os.MkdirAll(store.bookDir(book.ID), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := store.Save(book); err != nil {
			t.Fatal(err)
		}
	}
}

// zipFiles returns a zip of files, by name.
func zipFiles(t testing.TB, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
