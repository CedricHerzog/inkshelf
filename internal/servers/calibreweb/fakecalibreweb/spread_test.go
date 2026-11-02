package fakecalibreweb

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
)

// The spread book's middle chapters are an image spread of the index.
func TestSpreadEPUB(t *testing.T) {
	data, err := makeSpreadEPUB(book{ID: 53, Title: "Zoé et le phare", Author: "Anne Martin"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "book.epub")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	opened, err := epub.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	index, err := epub.BuildIndex(opened)
	if err != nil {
		t.Fatal(err)
	}
	if want := []epub.Spread{{First: 1, Left: 1, Right: 2}}; !slices.Equal(index.Spreads, want) {
		t.Errorf("spreads %v, want %v", index.Spreads, want)
	}
}
