package epub

import (
	"image"
	"strings"
	"sync"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/container"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
)

// ErrDRM is the error of a book whose content is encrypted (fonts obfuscated only are fine).
var ErrDRM = container.ErrDRM

// The values of rendition:layout.
const (
	Reflowable   = container.Reflowable
	PrePaginated = container.PrePaginated
)

// Book is an open EPUB: its container, and the documents built from its chapters. Its methods
// may be called from several goroutines at once.
type Book struct {
	*container.Book
	// calls are the links of the book's documents that have an id of their own, which may be
	// the calls of notes. They are read once (callsOnce), and kept by the target they lead to.
	// Both are "path#fragment".
	callsOnce sync.Once
	calls     map[string][]string
}

// Open opens the EPUB at filePath.
func Open(filePath string) (*Book, error) {
	opened, err := container.Open(filePath)
	if err != nil {
		return nil, err
	}
	return &Book{Book: opened}, nil
}

// Resource returns a file of the book, by its path in the zip; or the image of a
// pre-paginated spine item that its document shows, its page drawn as a PNG.
func (book *Book) Resource(filePath string) ([]byte, error) {
	if itemPath, ok := strings.CutPrefix(filePath, renderedPagePrefix); ok {
		return book.renderedPage(itemPath)
	}
	return book.ReadFile(filePath)
}

// ImageSize returns the size of an image of the book, from its header only, as the
// container's ImageSize does; or the size of the image of a pre-paginated spine item.
func (book *Book) ImageSize(filePath string) (image.Point, error) {
	if itemPath, ok := strings.CutPrefix(filePath, renderedPagePrefix); ok {
		return book.renderedPageSizeOf(itemPath)
	}
	return book.Book.ImageSize(filePath)
}

// resolve returns the path in the zip of href, relative to the file at base: without its
// fragment, percent-decoded, ".." at the root staying there.
func resolve(base, href string) string { return uri.Resolve(base, href) }
