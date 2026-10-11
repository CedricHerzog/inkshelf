package state

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// TestFiltersGolden reads a filters.json as the app saves it on the reader, and saves it
// again. Its format must not change: the readers' files are not rewritten when the app is
// updated.
func TestFiltersGolden(t *testing.T) {
	saved := LoadPreferences("testdata")
	if saved.Library != "lib-manga" || saved.Status != "reading" || saved.Books != BooksDownloaded || saved.TextSize != 44 {
		t.Errorf("read %+v", saved)
	}
	if len(saved.Libraries) != 2 || saved.Libraries[1] != (servers.Library{ID: "lib-manga", Name: "Manga"}) {
		t.Errorf("libraries: %+v", saved.Libraries)
	}
	if saved.Directions["series-1"] != servers.RightToLeft || saved.Directions["series-2"] != servers.LeftToRight {
		t.Errorf("directions: %+v", saved.Directions)
	}

	dir := t.TempDir()
	if err := saved.Save(dir); err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(filepath.Join(dir, "filters.json"))
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "filters.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, bytes.TrimSpace(fixture)) {
		t.Errorf("saved again as\n%s\nwant\n%s", encoded, bytes.TrimSpace(fixture))
	}
}
