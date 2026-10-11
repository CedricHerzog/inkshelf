package servers

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestConfigGolden reads a config.json as users write it on the reader. Its format must not
// change: the readers' files are not rewritten when the app is updated.
func TestConfigGolden(t *testing.T) {
	dir := filepath.Join("testdata", "golden")
	configs, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []Config{
		{Name: "Komga", Type: "komga", Server: "http://nas:25600", APIKey: "0123456789abcdef"},
		{Name: "Cuisine", Type: "mealie", Server: "https://mealie.example.org", APIKey: "mealie-token"},
		{Name: "Livres", Type: "calibreweb", Server: "http://calibre.local:8083", Username: "reader", Password: "secret"},
	}
	if len(configs) != len(want) {
		t.Fatalf("got %d servers, want %d", len(configs), len(want))
	}
	for index := range want {
		if configs[index] != want[index] {
			t.Errorf("server %d: got %+v, want %+v", index, configs[index], want[index])
		}
	}

	fixture, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(configs)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, bytes.TrimSpace(fixture)) {
		t.Errorf("encoded again as\n%s\nwant\n%s", encoded, bytes.TrimSpace(fixture))
	}
}
