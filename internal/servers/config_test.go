package servers

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		data    string   // the content of config.json
		want    []Config // the servers read, if no error
		wantErr string   // part of the error expected, if any
	}{
		{
			name: "list",
			data: `[{"name":" Manga ","server":"http://host:1/ ","apiKey":"a"},{"name":"BD","type":" Komga","server":"http://host:2","apiKey":"b"}]`,
			want: []Config{
				{Name: "Manga", Server: "http://host:1", APIKey: "a"},
				{Name: "BD", Type: "komga", Server: "http://host:2", APIKey: "b"},
			},
		},
		{
			name: "single",
			data: ` {"name":"Manga","server":"http://host:1","apiKey":"a"}`,
			want: []Config{{Name: "Manga", Server: "http://host:1", APIKey: "a"}},
		},
		{
			name:    "no name",
			data:    `[{"server":"http://host:1","apiKey":"a"}]`,
			wantErr: "server 1: name, server and apiKey (or username and password) are required",
		},
		{
			name: "username instead of a key",
			data: `[{"name":"Books","type":"calibreweb","server":"http://host:3","username":"me","password":"secret"}]`,
			want: []Config{{Name: "Books", Type: "calibreweb", Server: "http://host:3", Username: "me", Password: "secret"}},
		},
		{
			name:    "no key",
			data:    `[{"name":"A","server":"http://host:1","apiKey":"a"},{"name":"B","server":"http://host:2"}]`,
			wantErr: "server 2:",
		},
		{
			name:    "name used twice",
			data:    `[{"name":"A","server":"http://host:1","apiKey":"a"},{"name":"a","server":"http://host:2","apiKey":"b"}]`,
			wantErr: "used twice",
		},
		{
			name:    "slash in the name",
			data:    `[{"name":"a/b","server":"http://host:1","apiKey":"a"}]`,
			wantErr: "cannot be a folder name",
		},
		{
			name:    "dots as the name",
			data:    `[{"name":"..","server":"http://host:1","apiKey":"a"}]`,
			wantErr: "cannot be a folder name",
		},
		{name: "empty list", data: `[]`, wantErr: "no server"},
		{name: "bad JSON", data: `[{`, wantErr: "config.json:"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(test.data), 0o644); err != nil {
				t.Fatal(err)
			}
			configs, err := LoadConfig(dir)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("error %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(configs, test.want) {
				t.Errorf("got %+v, want %+v", configs, test.want)
			}
		})
	}
}

func TestLoadConfigMissing(t *testing.T) {
	if _, err := LoadConfig(t.TempDir()); err == nil {
		t.Error("no error without a config.json")
	}
}
