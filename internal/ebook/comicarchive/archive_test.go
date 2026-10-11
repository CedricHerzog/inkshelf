package comicarchive

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// writeZip writes a zip of files (name → content) in the order given, and returns its path.
func writeZip(tb testing.TB, names []string, contents map[string]string) string {
	tb.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, name := range names {
		entry, err := writer.Create(name)
		if err != nil {
			tb.Fatal(err)
		}
		if strings.HasSuffix(name, "/") { // a directory
			continue
		}
		if _, err := entry.Write([]byte(contents[name])); err != nil {
			tb.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		tb.Fatal(err)
	}
	filePath := filepath.Join(tb.TempDir(), "book.cbz")
	if err := os.WriteFile(filePath, buffer.Bytes(), 0o644); err != nil {
		tb.Fatal(err)
	}
	return filePath
}

func TestPages(t *testing.T) {
	names := []string{
		"Vol 1/page 10.jpg", "Vol 1/page 2.PNG", "Vol 1/", "Vol 1/ComicInfo.xml",
		"__MACOSX/Vol 1/._page 2.PNG", "Vol 1/.hidden.jpg", "Vol 1/page 1.webp",
	}
	contents := map[string]string{}
	for _, name := range names {
		contents[name] = "image of " + name
	}
	opened, err := Open(writeZip(t, names, contents))
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	want := []string{"Vol 1/page 1.webp", "Vol 1/page 2.PNG", "Vol 1/page 10.jpg"}
	if opened.PageCount() != len(want) {
		t.Fatalf("PageCount = %d, want %d", opened.PageCount(), len(want))
	}
	// Read concurrently, as the download's workers do.
	var group sync.WaitGroup
	for index, name := range want {
		group.Go(func() {
			data, err := opened.Page(index + 1)
			if err != nil {
				t.Error(err)
				return
			}
			if string(data) != contents[name] {
				t.Errorf("page %d = %q, want %q", index+1, data, contents[name])
			}
		})
	}
	group.Wait()
	if _, err := opened.Page(len(want) + 1); err == nil {
		t.Error("no error for a page past the last")
	}
}

func TestRAR(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "book.cbr")
	if err := os.WriteFile(filePath, []byte("Rar!\x1a\x07\x01\x00rest"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(filePath); !errors.Is(err, ErrRAR) {
		t.Errorf("Open = %v, want ErrRAR", err)
	}
}
