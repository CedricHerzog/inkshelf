package shelf

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// CoverKind tells a series' cover from a book's. It is also the name of their folder.
type CoverKind string

// The kinds of covers.
const (
	SeriesCover CoverKind = "series"
	BookCover   CoverKind = "books"
)

// CoverFiles keeps the covers of series and books, prepared for the panel like pages, in
// Dir/series/<id>.pg4 and Dir/books/<id>.pg4. A cover file's modification time is that of
// its series or book on the server, so a series or book modified since shows it is stale.
type CoverFiles struct {
	Dir string
}

// path returns the file of a cover.
func (covers CoverFiles) path(kind CoverKind, id string) string {
	return filepath.Join(covers.Dir, string(kind), filepath.Base(id)+pg4Extension)
}

// Load reads a cover saved by Fetch.
func (covers CoverFiles) Load(kind CoverKind, id string) (*image.Gray, error) {
	data, err := os.ReadFile(covers.path(kind, id))
	if err != nil {
		return nil, err
	}
	return pageimage.Unpack(data)
}

// Stale reports whether a cover is missing, or older than modified: when its series or book
// last changed on the server (the zero time if unknown).
func (covers CoverFiles) Stale(kind CoverKind, id string, modified time.Time) bool {
	info, err := os.Stat(covers.path(kind, id))
	return err != nil || info.ModTime().Before(modified)
}

// Fetch downloads a cover from server and saves it prepared for a width×height box, with
// modified as its modification time unless that is zero.
func (covers CoverFiles) Fetch(ctx context.Context, server servers.Server, kind CoverKind, id string, width, height int, modified time.Time) error {
	fetch := server.BookThumbnail
	if kind == SeriesCover {
		fetch = server.SeriesThumbnail
	}
	data, err := fetch(ctx, id)
	if err != nil {
		return err
	}
	packed, err := pageimage.Prepare(data, width, height, nil)
	if err != nil {
		return err
	}
	path := covers.path(kind, id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := writeFileAtomic(path, packed); err != nil {
		return err
	}
	if !modified.IsZero() {
		return os.Chtimes(path, modified, modified)
	}
	return nil
}
