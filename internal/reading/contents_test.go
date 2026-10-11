package reading

import (
	"slices"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// The entry a location is in is the last one starting at or before it, in whatever order the
// entries are listed.
func TestEntryAt(t *testing.T) {
	entries := []epub.TOCEntry{
		{Title: "One", Chapter: 0},
		{Title: "One, part 2", Chapter: 0, Offset: 500},
		{Title: "Landmark of two", Chapter: 1, Offset: 0},
		{Title: "Three", Chapter: 2},
		{Title: "Back to one", Chapter: 0, Offset: 200},
	}
	for _, test := range []struct {
		location shelf.Location
		want     int
	}{
		{shelf.Location{Chapter: 0, Offset: 0}, 0},
		{shelf.Location{Chapter: 0, Offset: 199}, 0},
		{shelf.Location{Chapter: 0, Offset: 200}, 4},
		{shelf.Location{Chapter: 0, Offset: 800}, 1},
		{shelf.Location{Chapter: 1, Offset: 30}, 2},
		{shelf.Location{Chapter: 5, Offset: 0}, 3},
	} {
		if got := EntryAt(entries, test.location); got != test.want {
			t.Errorf("EntryAt(%+v) = %d, want %d", test.location, got, test.want)
		}
	}
	if got := EntryAt(entries[3:4], shelf.Location{Chapter: 1}); got != -1 {
		t.Errorf("EntryAt before every entry = %d, want -1", got)
	}
}

// An entry starts at or before the offset of its own chapter, and before any later chapter.
func TestAtOrBefore(t *testing.T) {
	entry := epub.TOCEntry{Chapter: 2, Offset: 100}
	for _, test := range []struct {
		chapter, offset int
		want            bool
	}{
		{2, 100, true}, {2, 99, false}, {2, 101, true}, {3, 0, true}, {1, 5000, false},
	} {
		if got := atOrBefore(entry, test.chapter, test.offset); got != test.want {
			t.Errorf("atOrBefore(%+v, %d, %d) = %v", entry, test.chapter, test.offset, got)
		}
	}
	if got := EntryLocation(entry); got != (shelf.Location{Chapter: 2, Offset: 100}) {
		t.Errorf("EntryLocation = %+v", got)
	}
}

// A table of contents that leads into fewer chapters than the book has (its title only) gives
// way to the chapters with text, titled, else numbered.
func TestTableOfContents(t *testing.T) {
	chapters := []epub.ChapterIndex{{Chars: 100, Title: "Cover"}, {Chars: 0}, {Chars: 3000}, {Chars: 2000, Title: "Two"}}
	titleOnly := epub.Index{Chapters: chapters, TOC: []epub.TOCEntry{{Title: "The book", Chapter: 0}}}
	var titles []string
	for _, entry := range tableOfContents(titleOnly) {
		titles = append(titles, entry.Title)
	}
	if want := []string{"Cover", "Section 2", "Two"}; !slices.Equal(titles, want) {
		t.Errorf("titles %q, want %q", titles, want)
	}
	full := epub.Index{Chapters: chapters, TOC: []epub.TOCEntry{{Title: "Start", Chapter: 0}, {Title: "Middle", Chapter: 2}}}
	if got := tableOfContents(full); len(got) != 2 || got[1].Title != "Middle" {
		t.Errorf("a table of contents of two chapters became %+v", got)
	}
}
