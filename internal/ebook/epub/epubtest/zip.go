package epubtest

import (
	"archive/zip"
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Zip returns an EPUB of files (path → content): the mimetype first and stored, as EPUB
// requires, then files compressed, sorted by path so that the same files give the same zip.
func Zip(tb testing.TB, files map[string]string) []byte {
	tb.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := entry.Write([]byte("application/epub+zip")); err != nil {
		tb.Fatal(err)
	}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		entry, err := writer.Create(name)
		if err != nil {
			tb.Fatal(err)
		}
		if _, err := entry.Write([]byte(files[name])); err != nil {
			tb.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		tb.Fatal(err)
	}
	return buffer.Bytes()
}

// Write writes an EPUB of files, as Zip makes it, into a folder of its own that the test
// removes, and returns its path.
func Write(tb testing.TB, files map[string]string) string {
	tb.Helper()
	filePath := filepath.Join(tb.TempDir(), "book.epub")
	if err := os.WriteFile(filePath, Zip(tb, files), 0o644); err != nil {
		tb.Fatal(err)
	}
	return filePath
}
