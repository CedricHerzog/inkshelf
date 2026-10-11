package epub

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/container"
	"github.com/CedricHerzog/inkshelf/internal/timing"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// IndexVersion is the version of the parser that builds an Index. Offsets in a chapter depend
// on the parser: an index of another version is built again.
const IndexVersion = 7

// Index is what the reader needs of a book before laying out its chapters: their lengths, for
// the progress, the table of contents and the navigation document's other lists. It is saved
// next to the book.
type Index struct {
	Version  int            `json:"version"`
	Title    string         `json:"title"`
	Language string         `json:"language,omitempty"`
	Chapters []ChapterIndex `json:"chapters"`
	// Total is the length of the book, in runes.
	Total int        `json:"total"`
	TOC   []TOCEntry `json:"toc"`
	// PageProgression is the book's page-progression-direction: "ltr", "rtl", or "" for the
	// default of its language.
	PageProgression string `json:"pageProgression,omitempty"`
	// PageList is the pages of the printed book, if the navigation document lists them.
	PageList []TOCEntry `json:"pageList,omitempty"`
	// Landmarks is the navigation document's landmarks (the cover, the start of the text…).
	Landmarks []TOCEntry `json:"landmarks,omitempty"`
}

// RightToLeft reports whether the book's pages turn from right to left: as its
// page-progression-direction says, else as its language is written.
func (index Index) RightToLeft() bool {
	return container.PagesTurnRightToLeft(index.PageProgression, index.Language)
}

// ChapterIndex is a chapter of an Index.
type ChapterIndex struct {
	Path  string `json:"path"`
	Title string `json:"title,omitempty"` // from the table of contents, else its first heading
	Chars int    `json:"chars"`
}

// TOCEntry is an entry of the table of contents, the page list or the landmarks: a position
// in the book.
type TOCEntry struct {
	Title   string `json:"title"`
	Level   int    `json:"level"` // 0 for the top entries
	Chapter int    `json:"chapter"`
	Offset  int    `json:"offset"`
	// Type is a landmark's epub:type, such as "bodymatter"; "" in the other lists.
	Type string `json:"type,omitempty"`
}

// BuildIndex parses every chapter of book to measure it, and reads its navigation. A chapter
// that cannot be parsed counts as empty; a book none of whose chapters can is an error.
func BuildIndex(book *Book) (Index, error) {
	defer timing.Start(context.Background(), "build index").End()
	index := Index{
		Version: IndexVersion, Title: book.Title, Language: book.Language, PageProgression: book.PageProgression,
		Chapters: make([]ChapterIndex, book.Chapters()),
	}
	ids := make([]map[string]int, book.Chapters())
	var firstErr error
	parsed := 0
	for chapter := range book.Chapters() {
		index.Chapters[chapter].Path = book.ChapterItem(chapter).Path
		doc, err := book.chapterWithoutFonts(chapter)
		if err != nil {
			log.Printf("epub: %v", err)
			firstErr = cmp.Or(firstErr, err)
			continue
		}
		parsed++
		index.Chapters[chapter].Chars = doc.Chars
		index.Chapters[chapter].Title = firstHeading(doc)
		index.Total += doc.Chars
		ids[chapter] = doc.IDs
	}
	if parsed == 0 {
		return Index{}, fmt.Errorf("no chapter could be read: %w", firstErr)
	}
	navigation, err := book.Navigation()
	if err != nil {
		log.Printf("epub: table of contents: %v", err) // the book reads without one
	}
	index.TOC = entriesOf(book, navigation.TOC, ids)
	index.PageList = entriesOf(book, navigation.PageList, ids)
	index.Landmarks = entriesOf(book, navigation.Landmarks, ids)
	// A chapter's title is that of its first entry in the table of contents, rather than its
	// first heading.
	titled := make([]bool, len(index.Chapters))
	for _, entry := range index.TOC {
		if !titled[entry.Chapter] {
			index.Chapters[entry.Chapter].Title = entry.Title
			titled[entry.Chapter] = true
		}
	}
	return index, nil
}

// entriesOf returns the links of the navigation that lead into a chapter, as entries of the
// index. ids are the anchors of each chapter, by id: a link's fragment becomes an offset in
// its chapter (0, the chapter's start, for an anchor that isn't found).
func entriesOf(book *Book, links []container.Link, ids []map[string]int) []TOCEntry {
	var found []TOCEntry
	for _, link := range links {
		chapter := book.ChapterOfPath(link.Path)
		if chapter < 0 {
			continue
		}
		offset := 0
		if link.Fragment != "" {
			offset = ids[chapter][link.Fragment]
		}
		found = append(found, TOCEntry{Title: link.Title, Level: link.Level, Chapter: chapter, Offset: offset, Type: link.Kind})
	}
	return found
}

// firstHeading returns the text of a document's first heading, on one line and without soft
// hyphens, "" if it has none.
func firstHeading(doc *document.Document) string {
	for _, block := range doc.Blocks {
		if block.Kind != document.Heading {
			continue
		}
		var text strings.Builder
		for _, run := range block.Runs {
			text.WriteString(run.Text)
		}
		words := strings.Fields(strings.NewReplacer("\u00ad", "", "\u200b", "").Replace(text.String()))
		if title := strings.Join(words, " "); title != "" {
			return title
		}
	}
	return ""
}
