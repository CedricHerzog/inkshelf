package epub

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/container"
	"github.com/CedricHerzog/inkshelf/internal/ebook/fixedlayout"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// unsupportedContent is the text shown for a part of the book that has nothing the app can
// show.
const unsupportedContent = "This part of the book cannot be shown."

// Chapter returns chapter index (counted from 0) of the book as a document, built from its
// XHTML.
func (book *Book) Chapter(index int) (*document.Document, error) {
	if index < 0 || index >= book.Chapters() {
		return nil, fmt.Errorf("chapter %d of %d", index, book.Chapters())
	}
	return book.documentOf(book.ChapterItem(index), false)
}

// chapterWithoutFonts returns chapter index as Chapter does, but without the fonts it embeds
// and without its notes. BuildIndex, at download time, only needs its length and anchors.
func (book *Book) chapterWithoutFonts(index int) (*document.Document, error) {
	return book.documentOf(book.ChapterItem(index), true)
}

// Document returns the first item of the spine at itemPath, linear or not, as a document. The
// reader asks for it when a link leads to an item that ChapterOfPath finds in no chapter.
func (book *Book) Document(itemPath string) (*document.Document, error) {
	for _, item := range book.Spine() {
		if item.Path == itemPath {
			return book.documentOf(item, false)
		}
	}
	return nil, fmt.Errorf("%s: not in the spine", itemPath)
}

// documentOf returns the document of a spine item: its XHTML, its image, or a paragraph saying
// that the app cannot show it. A pre-paginated item becomes a single image, its page drawn
// whole. forIndex leaves out what BuildIndex doesn't need.
func (book *Book) documentOf(item container.SpineItem, forIndex bool) (*document.Document, error) {
	switch {
	case cmp.Or(item.Layout, book.Layout) == PrePaginated:
		doc := &document.Document{}
		doc.Add(document.Block{Kind: document.Image, Image: renderedPagePrefix + item.Path})
		return doc, nil
	case item.Content == "":
		doc := &document.Document{}
		doc.Add(document.Block{Kind: document.Paragraph, Align: document.AlignCenter, Runs: []document.Run{{Text: unsupportedContent, Style: document.Italic}}})
		return doc, nil
	case strings.HasPrefix(item.ContentType, "image/"):
		doc := &document.Document{}
		doc.Add(document.Block{Kind: document.Image, Image: item.Content})
		return doc, nil
	}
	return book.buildDocument(item.Content, nil, forIndex)
}

// buildDocument builds the document of the XHTML at chapterPath. For a pre-paginated chapter,
// page receives the regions of the page, and the document returned is the content of the
// page's flow; page is nil for a reflowable chapter.
func (book *Book) buildDocument(chapterPath string, page *fixedlayout.Page, forIndex bool) (*document.Document, error) {
	data, err := book.Resource(chapterPath)
	if err != nil {
		return nil, err
	}
	root := &element{name: documentName, style: rootStyle, block: &blockProps{fontSize: 100}, color: css.Black, background: 255}
	parser := &builder{book: book, chapterPath: chapterPath, cascade: css.NewCascade(), sheetsAdded: map[string]bool{}, stack: []*element{root}, forIndex: forIndex, page: page}
	parser.cascade.Add(userAgentSheet)
	if err := parser.parse(data); err != nil {
		return nil, fmt.Errorf("%s: %w", chapterPath, err)
	}
	parser.doc.Language = cmp.Or(parser.doc.Language, book.Language)
	return &parser.doc, nil
}
