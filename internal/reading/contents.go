package reading

import (
	"cmp"
	"fmt"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// TableOfContents returns the book's table of contents, or its chapters if it leads into
// fewer of them (some books list only their title).
func (text *TextBook) TableOfContents() []epub.TOCEntry {
	return tableOfContents(text.Index)
}

// tableOfContents is TableOfContents of the book of index.
func tableOfContents(index epub.Index) []epub.TOCEntry {
	listed := map[int]bool{}
	for _, entry := range index.TOC {
		listed[entry.Chapter] = true
	}
	var entries []epub.TOCEntry
	for chapter, info := range index.Chapters {
		if info.Chars > 0 {
			title := cmp.Or(info.Title, fmt.Sprintf("Section %d", len(entries)+1))
			entries = append(entries, epub.TOCEntry{Title: title, Chapter: chapter})
		}
	}
	if len(index.TOC) > 0 && len(listed) >= min(len(entries), 2) {
		return index.TOC
	}
	return entries
}

// EntryAt returns the index of the entry that location is in: the last one starting at or
// before it, -1 if none does. The entries need not be in the book's order.
func EntryAt(entries []epub.TOCEntry, location shelf.Location) int {
	found := -1
	for index, entry := range entries {
		if !atOrBefore(entry, location.Chapter, location.Offset) {
			continue
		}
		if found < 0 || atOrBefore(entries[found], entry.Chapter, entry.Offset) { //nolint:gosec // G602: found is an index of entries
			found = index
		}
	}
	return found
}

// atOrBefore reports whether entry starts at or before offset in chapter.
func atOrBefore(entry epub.TOCEntry, chapter, offset int) bool {
	return entry.Chapter < chapter || (entry.Chapter == chapter && entry.Offset <= offset)
}

// EntryLocation returns where entry leads.
func EntryLocation(entry epub.TOCEntry) shelf.Location {
	return shelf.Location{Chapter: entry.Chapter, Offset: entry.Offset}
}
