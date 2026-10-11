package container

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"io"

	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
	"github.com/CedricHerzog/inkshelf/internal/web/webimage"
)

// HasFile reports whether the zip holds a file at filePath.
func (book *Book) HasFile(filePath string) bool { return book.files[filePath] != nil }

// ReadFile returns a file of the book, by its path in the zip.
func (book *Book) ReadFile(filePath string) ([]byte, error) {
	file, ok := book.files[filePath]
	if !ok {
		return nil, fmt.Errorf("%s: not in the book", filePath)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}
	return data, nil
}

// ImageSize returns the size of an image of the book, from its header only: reading whole
// images to lay out a chapter of illustrations is slow.
func (book *Book) ImageSize(filePath string) (image.Point, error) {
	if uri.IsData(filePath) { // an image embedded in the chapter
		data, err := uri.DecodeData(filePath)
		if err != nil {
			return image.Point{}, err
		}
		return webimage.Size(data)
	}
	file, ok := book.files[filePath]
	if !ok {
		return image.Point{}, fmt.Errorf("%s: not in the book", filePath)
	}
	reader, err := file.Open()
	if err != nil {
		return image.Point{}, fmt.Errorf("%s: %w", filePath, err)
	}
	defer reader.Close()
	config, err := pageimage.DecodeConfig(reader)
	if errors.Is(err, image.ErrFormat) { // perhaps an SVG, whose size is in its markup
		if data, readErr := book.ReadFile(filePath); readErr == nil {
			if size, sizeErr := webimage.Size(data); sizeErr == nil {
				return size, nil
			}
		}
	}
	if err != nil {
		return image.Point{}, fmt.Errorf("%s: %w", filePath, err)
	}
	return image.Pt(config.Width, config.Height), nil
}

// Cover returns the cover image the package document names, if any.
func (book *Book) Cover() ([]byte, error) {
	if book.coverPath == "" {
		return nil, errors.New("the book names no cover")
	}
	return book.ReadFile(book.coverPath)
}

// CoverPath returns the path in the zip of the cover image the package document names, "" if
// none: the images an SVG cover links to are relative to it.
func (book *Book) CoverPath() string { return book.coverPath }

// readXML decodes the XML file at filePath into value.
func (book *Book) readXML(filePath string, value any) error {
	data, err := book.ReadFile(filePath)
	if err != nil {
		return err
	}
	decoder := NewDecoder(data)
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("%s: %w", filePath, err)
	}
	return nil
}

// NewDecoder returns a lenient XML decoder for data: HTML's entities, mismatched end tags
// closed as they come, and the charsets that x/text knows. HTML's void elements are not
// auto-closed: EPUBs are XML, and write <meta></meta> as often as <br/>.
func NewDecoder(data []byte) *xml.Decoder {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity
	decoder.CharsetReader = charsetReader
	return decoder
}
