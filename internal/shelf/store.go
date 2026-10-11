package shelf

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"

	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
)

// The files of a downloaded book, in its folder.
const (
	bookFile     = "book.json" // the Book, written last
	pg4Extension = ".pg4"      // the pages, as pageimage prepares them: 0001.pg4, 0002.pg4…
	pngExtension = ".png"      // the pages of books downloaded by older versions
)

// Store keeps each book in Dir/<id>/: its pages and book.json. As book.json is written last,
// a folder without it is a partial download, which Download resumes.
type Store struct {
	Dir string
	// Logf, if set, receives the time each download spent in each stage.
	Logf func(format string, args ...any)
}

// logf logs through Logf, if set.
func (store Store) logf(format string, args ...any) {
	if store.Logf != nil {
		store.Logf(format, args...)
	}
}

// bookDir returns the folder of book id.
func (store Store) bookDir(id string) string { return filepath.Join(store.Dir, filepath.Base(id)) }

// pagePath returns the file of page number (counted from 1) of book id, with extension
// pg4Extension, or pngExtension for older books.
func (store Store) pagePath(id string, number int, extension string) string {
	return filepath.Join(store.bookDir(id), fmt.Sprintf("%04d%s", number, extension))
}

// pageOnDisk reports whether page number of book id is already on disk: from an interrupted
// download, or from an older version.
func (store Store) pageOnDisk(id string, number int) bool {
	for _, extension := range []string{pg4Extension, pngExtension} {
		if _, err := os.Stat(store.pagePath(id, number, extension)); err == nil {
			return true
		}
	}
	return false
}

// Book returns a downloaded book, unless it is to be removed.
func (store Store) Book(id string) (Book, bool) {
	book, ok := store.BookEvenIfRemoved(id)
	return book, ok && !book.PendingRemoval
}

// BookEvenIfRemoved returns a downloaded book, even one to be removed. It reports false for a
// book not downloaded, partly downloaded, or whose book.json cannot be read.
func (store Store) BookEvenIfRemoved(id string) (Book, bool) {
	data, err := os.ReadFile(filepath.Join(store.bookDir(id), bookFile))
	if err != nil {
		return Book{}, false
	}
	var book Book
	if err := json.Unmarshal(data, &book); err != nil {
		return Book{}, false
	}
	return book, true
}

// Books returns the downloaded books, by series then by number, except those to be removed.
func (store Store) Books() []Book {
	return slices.DeleteFunc(store.AllBooks(), func(book Book) bool { return book.PendingRemoval })
}

// AllBooks returns the downloaded books, by series then by number, those to be removed
// included.
func (store Store) AllBooks() []Book {
	// A folder that cannot be read (not created yet) holds no books.
	entries, _ := os.ReadDir(store.Dir)
	var books []Book
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if book, ok := store.BookEvenIfRemoved(entry.Name()); ok {
			books = append(books, book)
		}
	}
	slices.SortFunc(books, compareBooks)
	return books
}

// Save writes book.json.
func (store Store) Save(book Book) error {
	data, err := json.Marshal(book)
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(store.bookDir(book.ID), bookFile), data)
}

// Delete removes a book, downloaded or partial.
func (store Store) Delete(id string) error { return os.RemoveAll(store.bookDir(id)) }

// Size returns the bytes a book takes on the reader, as far as they can be counted.
func (store Store) Size(id string) int64 {
	var size int64
	// A folder or a file that cannot be read counts as empty: the size is only shown.
	entries, _ := os.ReadDir(store.bookDir(id))
	for _, entry := range entries {
		if info, err := entry.Info(); err == nil {
			size += info.Size()
		}
	}
	return size
}

// LoadPage reads a page written by Download, or by an older version.
func (store Store) LoadPage(id string, number int) (*image.Gray, error) {
	data, err := os.ReadFile(store.pagePath(id, number, pg4Extension))
	if err == nil {
		return pageimage.Unpack(data)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	data, err = os.ReadFile(store.pagePath(id, number, pngExtension))
	if err != nil {
		return nil, err
	}
	return pageimage.DecodeGray(data)
}
