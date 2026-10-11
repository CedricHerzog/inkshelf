package shelf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
	"github.com/CedricHerzog/inkshelf/internal/web/svgimage"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
)

// The files of a downloaded Reflowable book, in its folder.
const (
	epubFile  = "book.epub"
	indexFile = "index.json" // its epub.Index
	coverFile = "cover.pg4"  // its cover, prepared for the screen
)

// CharsPerPage is the length of a Reflowable book's virtual page, in runes: Book.Pages and
// Book.Page count these, as the real pages depend on the font size.
const CharsPerPage = 1500

// downloadReflowable fetches book's EPUB, reads it through to index it, prepares its cover,
// then saves book.json with its virtual page count. A fixed-layout EPUB becomes page images.
func (store Store) downloadReflowable(ctx context.Context, server servers.Server, book Book, width, height int, reports *reporter) error {
	epubPath := store.EPUBPath(book.ID)
	if err := fetchFile(ctx, server, book.ID, epubPath, reports); err != nil {
		return err
	}
	reports.report(DownloadProgress{Preparing: true})
	opened, err := epub.Open(epubPath)
	if err != nil {
		// Fetching it again won't help a broken or protected book: don't keep it.
		_ = os.Remove(epubPath) // best effort: the book's error is the one worth returning
		return err
	}
	defer opened.Close()
	if opened.FixedLayout() {
		return store.downloadFixedLayout(ctx, opened, book, image.Pt(width, height), reports)
	}
	index, err := epub.BuildIndex(opened)
	if err != nil {
		_ = os.Remove(epubPath)
		return err
	}
	if err := store.saveIndex(book.ID, index); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	store.saveCover(ctx, server, opened, book.ID, image.Pt(width, height))
	book.Pages = max((index.Total+CharsPerPage-1)/CharsPerPage, 1)
	store.logf("download %s: EPUB of %d chapters, %d runes", book.ID, len(index.Chapters), index.Total)
	return store.Save(book)
}

// saveCover prepares the cover of book bookID for the screen: the server's, else the one the
// EPUB names. A book without one is fine: the cover view then says so.
func (store Store) saveCover(ctx context.Context, server servers.Server, opened *epub.Book, bookID string, size image.Point) {
	data, err := server.FirstPage(ctx, bookID)
	var linked svgimage.Resolver
	if err != nil {
		if data, err = opened.Cover(); err != nil {
			store.logf("cover of %s: %v", bookID, err)
			return
		}
		linked = func(href string) ([]byte, error) {
			return opened.Resource(uri.Resolve(opened.CoverPath(), href))
		}
	}
	packed, err := prepareCover(data, size, linked)
	if err == nil {
		err = writeFileAtomic(filepath.Join(store.bookDir(bookID), coverFile), packed)
	}
	if err != nil {
		store.logf("cover of %s: %v", bookID, err)
	}
}

// prepareCover prepares a cover image for the screen, size large, as a .pg4. An SVG is drawn
// fitted in the screen, with the images it links to read through linked (nil: left out), its
// text in the reader's serif.
func prepareCover(data []byte, size image.Point, linked svgimage.Resolver) ([]byte, error) {
	if !svgimage.IsSVG(data) {
		return pageimage.Prepare(data, size.X, size.Y, nil)
	}
	faces := layout.NewFaces(fonts.Serif)
	defer faces.Close()
	drawn, err := svgimage.Rasterize(data, size, svgimage.Options{Resolve: linked, Text: faces.SVGText()})
	if err != nil {
		return nil, err
	}
	return pageimage.PrepareDecoded(drawn, size.X, size.Y), nil
}

// EPUBPath returns the file of a Reflowable book.
func (store Store) EPUBPath(id string) string { return filepath.Join(store.bookDir(id), epubFile) }

// saveIndex writes the index of book id.
func (store Store) saveIndex(id string, index epub.Index) error {
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(store.bookDir(id), indexFile), data)
}

// LoadIndex returns the index of Reflowable book id, from opened (its EPUB). An index of
// another version of the parser is built again, as its offsets would not match.
func (store Store) LoadIndex(id string, opened *epub.Book) (epub.Index, error) {
	var index epub.Index
	data, err := os.ReadFile(filepath.Join(store.bookDir(id), indexFile))
	if err == nil {
		err = json.Unmarshal(data, &index)
	}
	if err == nil && index.Version == epub.IndexVersion {
		return index, nil
	}
	if index, err = epub.BuildIndex(opened); err != nil {
		return epub.Index{}, err
	}
	if err := store.saveIndex(id, index); err != nil {
		store.logf("saving the index of %s: %v", id, err) // it is built again next time
	}
	return index, nil
}

// LoadFirstPage returns the first page of a downloaded book, for the cover view: page 1 of a
// book of pages, the cover of a Reflowable one.
func (store Store) LoadFirstPage(book Book) (*image.Gray, error) {
	if book.Format != servers.Reflowable {
		return store.LoadPage(book.ID, 1)
	}
	data, err := os.ReadFile(filepath.Join(store.bookDir(book.ID), coverFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s has no cover", book.Title)
	}
	if err != nil {
		return nil, err
	}
	return pageimage.Unpack(data)
}
