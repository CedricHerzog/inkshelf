package container

import (
	"archive/zip"
	"cmp"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
)

// Book is the container of an open EPUB. Its methods may be called from several goroutines at
// once.
type Book struct {
	Title    string
	Language string   // the first dc:language, "" if none
	Creators []string // the dc:creators, in the order of the package document
	// Identifier is the book's unique identifier, which obfuscated fonts are keyed with.
	Identifier string
	// PageProgression is the spine's page-progression-direction: "ltr", "rtl", or "" for the
	// default of the book's language.
	PageProgression string
	// Layout is the book's rendition:layout: Reflowable or PrePaginated.
	Layout string

	zip   *zip.ReadCloser
	files map[string]*zip.File // by path in the zip
	// manifest is the files that the package document lists, by id, with their paths
	// resolved; manifestIDs is their ids by path.
	manifest    map[string]manifestItem
	manifestIDs map[string]string
	// spine is every item of the spine in reading order, and chapters its linear items.
	spine     []SpineItem
	chapters  []SpineItem
	navPath   string // the EPUB 3 navigation document, "" if none
	ncxPath   string // the EPUB 2 table of contents, "" if none
	coverPath string // the cover image, "" if none
	// obfuscated is the algorithm of each obfuscated font, by path in the zip. identifiers are
	// the dc:identifiers that may be the key, Identifier first: Adobe's key is a UUID, which
	// books often give as an identifier other than their unique one.
	obfuscated  map[string]string
	identifiers []string

	// cacheMu guards what the chapters share and is read once: stylesheets and fonts, by path
	// in the zip, nil for a file that cannot be read.
	cacheMu sync.Mutex
	sheets  map[string]*css.Stylesheet
	fonts   map[string]*fonts.File
}

// Open opens the EPUB at filePath.
func Open(filePath string) (*Book, error) {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, err
	}
	book := &Book{zip: reader, files: map[string]*zip.File{}}
	for _, file := range reader.File {
		book.files[file.Name] = file
	}
	if err := book.load(); err != nil {
		reader.Close()
		return nil, fmt.Errorf("%s: %w", path.Base(filePath), err)
	}
	return book, nil
}

// load refuses a book with DRM, then reads its package document: metadata, manifest and
// spine.
func (book *Book) load() error {
	if err := book.checkEncryption(); err != nil {
		return err
	}
	opfPath, err := book.packageDocumentPath()
	if err != nil {
		return err
	}
	var opf packageDocument
	if err := book.readXML(opfPath, &opf); err != nil {
		return err
	}
	book.readMetadata(opf)
	book.readManifest(opf, opfPath)
	if err := book.readSpine(opf); err != nil {
		return err
	}
	if len(book.chapters) == 0 {
		return errors.New("the book has no chapters")
	}
	return nil
}

// Chapters returns the number of chapters: the spine's linear items.
func (book *Book) Chapters() int { return len(book.chapters) }

// ChapterItem returns the spine item of chapter index (counted from 0).
func (book *Book) ChapterItem(index int) SpineItem { return book.chapters[index] }

// Spine returns every item of the spine, linear or not, in reading order. It must not be
// changed.
func (book *Book) Spine() []SpineItem { return book.spine }

// ChapterOfPath returns the first chapter whose spine item is at itemPath (the target of a
// link), or -1 if no linear item of the spine is. Package epub's Book.Document builds such an
// item.
func (book *Book) ChapterOfPath(itemPath string) int {
	return slices.IndexFunc(book.chapters, func(item SpineItem) bool { return item.Path == itemPath })
}

// ChapterLayout returns the rendition:layout of chapter index: its itemref's, else the book's.
func (book *Book) ChapterLayout(index int) string {
	return cmp.Or(book.chapters[index].Layout, book.Layout)
}

// rightToLeftLanguages are the languages written right to left.
var rightToLeftLanguages = []string{"ar", "dv", "fa", "he", "ps", "sd", "ug", "ur", "yi"}

// RightToLeft reports whether the book's pages turn from right to left: as its
// page-progression-direction says, else as its language is written.
func (book *Book) RightToLeft() bool {
	return PagesTurnRightToLeft(book.PageProgression, book.Language)
}

// PagesTurnRightToLeft reports whether the pages of a book whose page-progression-direction is
// pageProgression and whose language is language turn from right to left.
func PagesTurnRightToLeft(pageProgression, language string) bool {
	if pageProgression != "" {
		return pageProgression == "rtl"
	}
	primary, _, _ := strings.Cut(strings.ToLower(language), "-")
	return slices.Contains(rightToLeftLanguages, primary)
}

// Close closes the book's file.
func (book *Book) Close() error { return book.zip.Close() }

// resolve returns the path in the zip of href, relative to the file at base: without its
// fragment, percent-decoded, ".." at the root staying there.
func resolve(base, href string) string { return uri.Resolve(base, href) }

// collapse returns text with its whitespace trimmed and each run of it made one space.
func collapse(text string) string { return strings.Join(strings.Fields(text), " ") }
