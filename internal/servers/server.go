package servers

import (
	"context"
	"io"
	"time"
)

// Server is a server of series and their books, as the app sees it. A server that cannot do
// something (list libraries, keep reading progress) returns an error wrapping
// errors.ErrUnsupported, and the app does without it. A server that keeps no reading progress
// says so from InProgress, SetProgress and ClearProgress: the reader's own progress is then
// the only one, never overridden by the server's.
type Server interface {
	// Series returns all the series, sorted by title.
	Series(ctx context.Context) ([]Series, error)
	// Libraries returns the libraries the series are filed in.
	Libraries(ctx context.Context) ([]Library, error)
	// Books returns the books of a series, in their order in it.
	Books(ctx context.Context, seriesID string) ([]Book, error)
	// InProgress returns at most count books started and not finished, the last read first.
	InProgress(ctx context.Context, count int) ([]Book, error)

	// OpenBook prepares the download of the pageCount pages of a book.
	OpenBook(ctx context.Context, bookID string, pageCount int) (Pages, error)
	// OpenFile returns the file of a book whose Format is ComicArchive or Reflowable, and its
	// size in bytes, -1 if unknown. The caller closes it.
	OpenFile(ctx context.Context, bookID string) (io.ReadCloser, int64, error)
	// FirstPage returns page 1 of a book (its cover for a Reflowable one), in a format the
	// app decodes.
	FirstPage(ctx context.Context, bookID string) ([]byte, error)
	// SeriesThumbnail and BookThumbnail return the small cover of a series or a book.
	SeriesThumbnail(ctx context.Context, seriesID string) ([]byte, error)
	BookThumbnail(ctx context.Context, bookID string) ([]byte, error)

	// SetProgress marks page (counted from 1) as the last one read of a book, and the book as
	// completed or not.
	SetProgress(ctx context.Context, bookID string, page int, completed bool) error
	// ClearProgress marks a book unread.
	ClearProgress(ctx context.Context, bookID string) error
}

// Pages are the pages of a book being downloaded. Page may be called from several goroutines
// at once.
type Pages interface {
	// Page returns page number (counted from 1), as JPEG, PNG or WebP.
	Page(ctx context.Context, number int) ([]byte, error)
}

// Opener returns the Server of a server of config.json.
type Opener func(config Config) (Server, error)

// Series is a series of books.
type Series struct {
	ID        string
	LibraryID string // "" if the server has no libraries
	Title     string
	SortTitle string // what the A–Z jump files it under; "" to use Title
	Direction Direction
	Modified  time.Time // when it last changed; zero if unknown
	// Books is the number of books, BooksRead of those read and BooksInProgress of those
	// started and not finished.
	Books, BooksRead, BooksInProgress int
}

// Book is a book of a series (a volume or a chapter).
type Book struct {
	ID          string
	SeriesID    string
	LibraryID   string
	SeriesTitle string
	Title       string
	Number      float64          // its place in its series
	Pages       int              // 0 if unknown before the download (Format other than PageImages)
	Format      Format           // how the book is downloaded and read
	Modified    time.Time        // when it last changed; zero if unknown
	Progress    *ReadingProgress // nil if the book is unread
}

// Format is how a book is downloaded and read.
type Format int

// The formats of books.
const (
	// PageImages is a book of page images, downloaded through OpenBook.
	PageImages Format = iota
	// ComicArchive is a CBZ (a zip of page images) downloaded through OpenFile, whose pages
	// are known once it is downloaded.
	ComicArchive
	// Reflowable is an EPUB downloaded through OpenFile and laid out while it is read.
	Reflowable
)

// ReadingProgress is how far a book has been read.
type ReadingProgress struct {
	Page      int // the last page read, counted from 1
	Completed bool
}

// Library is a group of series on the server, e.g. Manga, Comics, BD.
type Library struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Direction is how a series reads, as the app saves it.
type Direction string

// The reading directions; "" is unknown.
const (
	LeftToRight Direction = "ltr"
	RightToLeft Direction = "rtl"
)

// ReadStatus is how far a series has been read.
type ReadStatus int

// The read statuses.
const (
	Unread  ReadStatus = iota // no book started
	Reading                   // some books started or read, not all read
	Read                      // every book read
)

// SeriesStatus returns how far a series has been read (as Komga's read_status filter defines
// it) from its number of books, how many of them are read, and how many are started and not
// finished.
func SeriesStatus(books, read, inProgress int) ReadStatus {
	switch {
	case books > 0 && read >= books:
		return Read
	case read == 0 && inProgress == 0:
		return Unread
	}
	return Reading
}

// Status returns how far the series has been read.
func (series Series) Status() ReadStatus {
	return SeriesStatus(series.Books, series.BooksRead, series.BooksInProgress)
}
