package fakecalibreweb

import (
	"slices"
	"time"
)

// book is a book of the fake library.
type book struct {
	ID      int
	Title   string
	Author  string
	Formats []string // in lower case: "epub", "cbz", "pdf"
	Updated time.Time
	// Pages is the number of pages of a CBZ.
	Pages int
	// Chapters is the number of chapters of an EPUB.
	Chapters int
	// FixedLayout makes the EPUB a fixed-layout one, of 3 pages (makeFixedLayoutEPUB).
	FixedLayout bool
	// Showcase makes the EPUB the showcase of scripts and CSS Text (makeShowcaseEPUB).
	Showcase bool
	// SeriesIndex is the book's place in its series, as Calibre keeps it.
	SeriesIndex float64
}

// series is a series of the fake library, its books in their order in it.
type series struct {
	ID      int
	Title   string
	BookIDs []int
}

// day returns noon UTC on a day of 2026.
func day(month time.Month, number int) time.Time {
	return time.Date(2026, month, number, 12, 0, 0, 0, time.UTC)
}

// books are the books of the fake library, the newest first, as /opds/new lists them.
var books = []book{
	{ID: 31, Title: "Contes courts", Author: "Anne Martin", Formats: []string{"epub"}, Updated: day(9, 30), Chapters: 2},
	{ID: 32, Title: "Le voyage illustré", Author: "Anne Martin", Formats: []string{"epub"}, Updated: day(9, 25), FixedLayout: true},
	{ID: 41, Title: "Notes de cours", Author: "Paul Durand", Formats: []string{"pdf"}, Updated: day(9, 20)},
	{ID: 22, Title: "Ruines, Tome 3", Author: "Léa Petit", Formats: []string{"cbz"}, Updated: day(9, 10), Pages: 6, SeriesIndex: 3},
	{ID: 21, Title: "Ruines, Tome 1", Author: "Léa Petit", Formats: []string{"cbz"}, Updated: day(9, 1), Pages: 8, SeriesIndex: 1},
	{ID: 13, Title: "Les Étoiles, Tome 3", Author: "Marc Leroy", Formats: []string{"epub", "kepub"}, Updated: day(8, 20), Chapters: 3, SeriesIndex: 3},
	{ID: 12, Title: "Les Étoiles, Tome 2", Author: "Marc Leroy", Formats: []string{"epub", "kepub"}, Updated: day(8, 10), Chapters: 3, SeriesIndex: 2},
	{ID: 11, Title: "Les Étoiles, Tome 1", Author: "Marc Leroy", Formats: []string{"epub", "kepub"}, Updated: day(8, 1), Chapters: 3, SeriesIndex: 1},
	{ID: 51, Title: "World scripts", Author: "Anne Martin", Formats: []string{"epub"}, Updated: day(7, 1), Showcase: true},
}

// allSeries are the series of the fake library. Ruines lacks its second book.
var allSeries = []series{
	{ID: 2, Title: "Ruines", BookIDs: []int{21, 22}},
	{ID: 1, Title: "Les Étoiles", BookIDs: []int{11, 12, 13}},
}

// bookByID returns the book id, and false if there is none.
func bookByID(id int) (book, bool) {
	for _, item := range books {
		if item.ID == id {
			return item, true
		}
	}
	return book{}, false
}

// seriesOf returns the series of book id, and false if it is in none.
func seriesOf(id int) (series, bool) {
	for _, item := range allSeries {
		if slices.Contains(item.BookIDs, id) {
			return item, true
		}
	}
	return series{}, false
}
