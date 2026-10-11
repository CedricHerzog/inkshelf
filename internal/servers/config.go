package servers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is a server of config.json, copied to the reader over USB.
type Config struct {
	Name string `json:"name"` // shown at start-up; also the folder of the server's books
	// Type is the kind of server: "komga" (also when empty), "mealie" or "calibreweb".
	Type   string `json:"type,omitempty"`
	Server string `json:"server"` // e.g. http://nas:25600
	// APIKey is created in Komga under Account settings > API keys, in Mealie under Profile >
	// API Tokens.
	APIKey string `json:"apiKey,omitempty"`
	// Username and Password log in to servers without API keys (Calibre-Web).
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// LoadConfig reads the servers of dir/config.json: a list of them, or a single one.
func LoadConfig(dir string) ([]Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return nil, err
	}
	var configs []Config
	// A single server may be written as an object instead of a list of one.
	if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && trimmed[0] == '{' {
		configs = make([]Config, 1)
		err = json.Unmarshal(trimmed, &configs[0])
	} else {
		err = json.Unmarshal(data, &configs)
	}
	if err != nil {
		return nil, fmt.Errorf("config.json: %w", err)
	}
	if len(configs) == 0 {
		return nil, errors.New("config.json: no server")
	}
	seenNames := map[string]bool{} // lowercased, as FAT folder names ignore case
	for index := range configs {
		config := &configs[index]
		config.Name = strings.TrimSpace(config.Name)
		config.Type = strings.ToLower(strings.TrimSpace(config.Type))
		config.Server = strings.TrimRight(strings.TrimSpace(config.Server), "/")
		if config.Name == "" || config.Server == "" || (config.APIKey == "" && config.Username == "") {
			return nil, fmt.Errorf("config.json: server %d: name, server and apiKey (or username and password) are required", index+1)
		}
		// The name is a folder on the reader's FAT storage.
		if config.Name == "." || config.Name == ".." || strings.ContainsAny(config.Name, `/\:*?"<>|`) {
			return nil, fmt.Errorf("config.json: server %d: name %q cannot be a folder name", index+1, config.Name)
		}
		if seenNames[strings.ToLower(config.Name)] {
			return nil, fmt.Errorf("config.json: server %d: name %q is used twice", index+1, config.Name)
		}
		seenNames[strings.ToLower(config.Name)] = true
	}
	return configs, nil
}
