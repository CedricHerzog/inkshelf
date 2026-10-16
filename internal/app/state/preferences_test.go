package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDarkPagesSaved pins how filters.json keeps the dark or light pages chosen per series,
// which the golden file of older versions does not have.
func TestDarkPagesSaved(t *testing.T) {
	dir := t.TempDir()
	saved := Preferences{DarkPages: map[string]bool{"series-1": true, "series-2": false}}
	if err := saved.Save(dir); err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(filepath.Join(dir, "filters.json"))
	if err != nil {
		t.Fatal(err)
	}
	if want := `"darkPages":{"series-1":true,"series-2":false}`; !strings.Contains(string(encoded), want) {
		t.Errorf("saved as %s, want %s in it", encoded, want)
	}
	loaded := LoadPreferences(dir)
	if len(loaded.DarkPages) != 2 || !loaded.DarkPages["series-1"] || loaded.DarkPages["series-2"] {
		t.Errorf("read back %v", loaded.DarkPages)
	}
}
