package comicarchive

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/naturalorder"
)

// ErrRAR is the error of a RAR archive (a CBR), which the standard library cannot read.
var ErrRAR = errors.New("RAR archives (CBR) are not supported")

// rarMagic starts every RAR archive, of version 4 and 5.
var rarMagic = []byte("Rar!\x1a\x07")

// pageExtensions are the extensions of the images the app decodes.
var pageExtensions = []string{".jpg", ".jpeg", ".jxl", ".png", ".webp"}

// Archive is an open comic book archive. Page may be called from several goroutines at once.
type Archive struct {
	zip   *zip.ReadCloser
	pages []*zip.File // the page images, in reading order
}

// Open opens the archive at filePath and lists its pages.
func Open(filePath string) (*Archive, error) {
	if err := checkNotRAR(filePath); err != nil {
		return nil, err
	}
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}
	opened := &Archive{zip: reader}
	for _, file := range reader.File {
		if isPage(file) {
			opened.pages = append(opened.pages, file)
		}
	}
	slices.SortStableFunc(opened.pages, func(left, right *zip.File) int {
		return naturalorder.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	})
	return opened, nil
}

// checkNotRAR returns ErrRAR if the file at filePath is a RAR archive.
func checkNotRAR(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	start := make([]byte, len(rarMagic))
	if _, err := io.ReadFull(file, start); err == nil && bytes.Equal(start, rarMagic) {
		return fmt.Errorf("%s: %w", filePath, ErrRAR)
	}
	return nil
}

// isPage reports whether file is a page image: an image that is not macOS metadata nor hidden.
func isPage(file *zip.File) bool {
	if file.FileInfo().IsDir() || strings.HasPrefix(file.Name, "__MACOSX/") {
		return false
	}
	base := path.Base(file.Name)
	return !strings.HasPrefix(base, ".") && slices.Contains(pageExtensions, strings.ToLower(path.Ext(base)))
}

// PageCount returns the number of pages.
func (opened *Archive) PageCount() int { return len(opened.pages) }

// Page returns the image of page number, counted from 1.
func (opened *Archive) Page(number int) ([]byte, error) {
	if number < 1 || number > len(opened.pages) {
		return nil, fmt.Errorf("page %d of %d: out of range", number, len(opened.pages))
	}
	file := opened.pages[number-1]
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file.Name, err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file.Name, err)
	}
	return data, nil
}

// Close closes the archive.
func (opened *Archive) Close() error { return opened.zip.Close() }
