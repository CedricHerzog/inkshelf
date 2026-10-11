package fakekomga

import (
	"fmt"
	"iter"
	"strings"
)

// lastModified is when every series and book last changed.
const lastModified = "2024-01-01T00:00:00Z"

// series is a generated series.
type series struct {
	id, title string
	titleSort string      // Komga's sort title, if not the title
	library   string      // the id of its library, in libraries
	direction string      // Komga's readingDirection
	books     int         // how many books it has
	pages     int         // how many pages each of its books has
	progress  map[int]int // the page read of some books, by book number, at the start; the last page means completed
	webpBook  int         // the book whose third page is listed as WebP (0: none)
	noCovers  bool        // the series and its books have no thumbnail
	// jpegOffset shifts which pages are JPEG: pages alternate PNG and JPEG.
	jpegOffset int
}

// libraryEntry is a library of the server.
type libraryEntry struct {
	id, name string
}

// libraries are the server's libraries, sorted by name as Komga lists them.
var libraries = []libraryEntry{
	{id: "bd", name: "BD"},
	{id: "comics", name: "Comics"},
	{id: "manga", name: "Manga"},
}

// catalog is the server's content, sorted by sort title, in three libraries:
//   - a short series being read, which reads right to left;
//   - one with enough books to page the book list, read up to its 14th book;
//   - two BD, which read left to right: a single book with a WebP page, and a series read to
//     the end;
//   - a series whose title is too long for a line, and which has no covers;
//   - then unread comics, enough to page the series list, the last one sorted by a title
//     other than its own.
//
// The series that don't say how they read leave it to the app.
var catalog = append([]series{
	{
		id: "alpha", title: "Alpha Saga", library: "manga", direction: "RIGHT_TO_LEFT",
		books: 3, pages: 10, progress: map[int]int{2: 4},
	},
	{
		id: "beta", title: "Beta Chronicles", library: "manga",
		books: 25, pages: 6, progress: readUpTo(14, 6),
	},
	{
		id: "delta", title: "Delta", library: "bd", direction: "LEFT_TO_RIGHT",
		books: 1, pages: 5, webpBook: 1,
	},
	{
		id: "epsilon", title: "Epsilon", library: "bd", direction: "LEFT_TO_RIGHT",
		books: 2, pages: 4, progress: map[int]int{1: 4, 2: 4},
	},
	{
		id: "gamma", title: "Gamma: The Remarkably Long Title of a Series That Never Fits on One Line", library: "comics",
		books: 2, pages: 8, jpegOffset: 1, noCovers: true,
	},
}, fillers("Kappa", "Lambda", "Mu", "Nu", "Omicron", "Pi", "Rho", "Sigma", "Tau", "Upsilon", "The Zeta")...)

// fillers returns unread one-book comics with these titles. "The X" sorts as "X, The".
func fillers(titles ...string) []series {
	var list []series
	for _, title := range titles {
		item := series{
			id:      strings.ToLower(strings.ReplaceAll(title, " ", "-")),
			title:   title,
			library: "comics",
			books:   1,
			pages:   4,
		}
		if name, ok := strings.CutPrefix(title, "The "); ok {
			item.titleSort = name + ", The"
		}
		list = append(list, item)
	}
	return list
}

// readUpTo returns the progress of a series whose books 1 to last are read, each having
// pages pages.
func readUpTo(last, pages int) map[int]int {
	progress := map[int]int{}
	for number := range upTo(last) {
		progress[number] = pages
	}
	return progress
}

// upTo yields the numbers 1 to last, as books and pages are numbered.
func upTo(last int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for index := range last {
			if !yield(index + 1) {
				return
			}
		}
	}
}

// bookNumbers yields the numbers of the series' books.
func (item series) bookNumbers() iter.Seq[int] { return upTo(item.books) }

// pageNumbers yields the numbers of the pages of each of the series' books.
func (item series) pageNumbers() iter.Seq[int] { return upTo(item.pages) }

// bookID returns the id of book number of a series.
func bookID(seriesID string, number int) string { return fmt.Sprintf("%s-%02d", seriesID, number) }

// findBook returns the series and the number of a book id.
func findBook(id string) (series, int, bool) {
	for _, item := range catalog {
		for number := range item.bookNumbers() {
			if bookID(item.id, number) == id {
				return item, number, true
			}
		}
	}
	return series{}, 0, false
}

// mediaType returns the type that page number of book is listed with.
func mediaType(item series, book, number int) string {
	switch {
	case book == item.webpBook && number == 3:
		return "image/webp"
	case (number+item.jpegOffset)%2 == 0:
		return "image/jpeg"
	default:
		return "image/png"
	}
}
