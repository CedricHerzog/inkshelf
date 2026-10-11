package calibreweb

import (
	"cmp"
	"context"
	"log"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// The formats the app reads, as Calibre-Web names them in its links.
const (
	epubFormat = "epub"
	cbzFormat  = "cbz"
)

// The id and title of the series of the books in none.
const (
	noSeriesID    = "no-series"
	noSeriesTitle = "No series"
)

// seriesWorkers is how many series feeds are fetched at once.
const seriesWorkers = 3

// catalogBook is a book the app can read.
type catalogBook struct {
	id      string // Calibre's
	title   string
	updated time.Time
	format  servers.Format
	file    string // the path of the file to download
	cover   string // the path of the cover, "" if none
}

// catalogSeries is a series, its books in their order in it.
type catalogSeries struct {
	id      string
	title   string
	bookIDs []string
	// numbers are the books' places in it by id: Calibre's series index, which may be
	// fractional (2.5, a novella between 2 and 3); missing for the books whose entry gives none.
	numbers map[string]float64
}

// catalog is the books the app can read and their series, sorted by title, the books in no
// series last.
type catalog struct {
	books  map[string]catalogBook
	series []catalogSeries
}

// loadCatalog reads the whole catalog: every book first, then the books of every series in
// their order in it. The books in no series go to a series of their own, listed last.
func (client *Client) loadCatalog(ctx context.Context) (catalog, error) {
	books, err := client.readBooks(ctx)
	if err != nil {
		return catalog{}, err
	}
	allSeries, err := client.readSeries(ctx, books)
	if err != nil {
		return catalog{}, err
	}
	if alone := seriesOfBooksInNone(books, allSeries); len(alone.bookIDs) > 0 {
		allSeries = append(allSeries, alone)
	}
	return catalog{books: books, series: allSeries}, nil
}

// readBooks returns every book the app can read, by id. The others are only counted in the
// log, so that a book missing from the app can be explained.
func (client *Client) readBooks(ctx context.Context) (map[string]catalogBook, error) {
	entries, err := client.readFeed(ctx, allBooksPath)
	if err != nil {
		return nil, err
	}
	books := map[string]catalogBook{}
	skipped := 0
	for _, item := range entries {
		book, ok := bookOf(item)
		if !ok {
			skipped++
			continue
		}
		books[book.id] = book
	}
	if skipped > 0 {
		log.Printf("calibreweb: %d books skipped, without an EPUB or a CBZ", skipped)
	}
	return books, nil
}

// readSeries returns the series that hold at least one of books, sorted by title.
func (client *Client) readSeries(ctx context.Context, books map[string]catalogBook) ([]catalogSeries, error) {
	seriesEntries, err := client.readFeed(ctx, allSeriesPath)
	if err != nil {
		return nil, err
	}
	allSeries := seriesOf(seriesEntries)
	if err := client.fillSeries(ctx, allSeries, books); err != nil {
		return nil, err
	}
	allSeries = slices.DeleteFunc(allSeries, func(series catalogSeries) bool { return len(series.bookIDs) == 0 })
	slices.SortFunc(allSeries, func(first, second catalogSeries) int { return servers.CompareTitles(first.title, second.title) })
	return allSeries, nil
}

// seriesOfBooksInNone returns the series of the books that are in none of allSeries, sorted
// by title.
func seriesOfBooksInNone(books map[string]catalogBook, allSeries []catalogSeries) catalogSeries {
	inSeries := map[string]bool{}
	for _, series := range allSeries {
		for _, id := range series.bookIDs {
			inSeries[id] = true
		}
	}
	alone := catalogSeries{id: noSeriesID, title: noSeriesTitle}
	for id := range books {
		if !inSeries[id] {
			alone.bookIDs = append(alone.bookIDs, id)
		}
	}
	slices.SortFunc(alone.bookIDs, func(first, second string) int {
		return servers.CompareTitles(books[first].title, books[second].title)
	})
	return alone
}

// bookOf returns the book of a catalog entry, and false if the app cannot read it (no EPUB
// nor CBZ).
func bookOf(item entry) (catalogBook, bool) {
	id, ok := item.bookID()
	if !ok {
		return catalogBook{}, false
	}
	book := catalogBook{id: id, title: strings.TrimSpace(item.Title), updated: item.Updated}
	files := item.acquisitions()
	switch {
	case files[epubFormat].Href != "":
		book.format, book.file = servers.Reflowable, files[epubFormat].Href
	case files[cbzFormat].Href != "":
		book.format, book.file = servers.ComicArchive, files[cbzFormat].Href
	default:
		return catalogBook{}, false
	}
	if cover, ok := linkTo(item.Links, relCover); ok {
		book.cover = cover.Href
	} else if thumbnail, ok := linkTo(item.Links, relThumbnail); ok {
		book.cover = thumbnail.Href
	}
	return book, true
}

// seriesOf returns the series that the entries of the series index link to, without their
// books yet.
func seriesOf(seriesEntries []entry) []catalogSeries {
	allSeries := make([]catalogSeries, 0, len(seriesEntries))
	for _, item := range seriesEntries {
		if subsection, ok := linkTo(item.Links, relSubsection); ok {
			allSeries = append(allSeries, catalogSeries{id: path.Base(subsection.Href), title: strings.TrimSpace(item.Title)})
		}
	}
	return allSeries
}

// fillSeries reads the feed of each of allSeries, seriesWorkers at a time, and fills in its
// books. Every feed is tried, and the first error is returned at the end.
func (client *Client) fillSeries(ctx context.Context, allSeries []catalogSeries, books map[string]catalogBook) error {
	pending := make(chan *catalogSeries, len(allSeries))
	for index := range allSeries {
		pending <- &allSeries[index]
	}
	close(pending)
	var workers sync.WaitGroup
	var mu sync.Mutex // guards firstErr
	var firstErr error
	for range min(seriesWorkers, len(allSeries)) {
		workers.Go(func() {
			for series := range pending {
				if err := client.fillOneSeries(ctx, series, books); err != nil {
					mu.Lock()
					firstErr = cmp.Or(firstErr, err)
					mu.Unlock()
				}
			}
		})
	}
	workers.Wait()
	return firstErr
}

// fillOneSeries reads the feed of series and adds the books of it that are in books, in their
// order in it, each with its series index when its entry gives one.
func (client *Client) fillOneSeries(ctx context.Context, series *catalogSeries, books map[string]catalogBook) error {
	entries, err := client.readFeed(ctx, seriesPath(series.id))
	if err != nil {
		return err
	}
	for _, item := range entries {
		id, ok := item.bookID()
		if !ok || books[id].id == "" || slices.Contains(series.bookIDs, id) {
			continue
		}
		series.bookIDs = append(series.bookIDs, id)
		if number, ok := item.seriesIndex(series.title); ok {
			if series.numbers == nil {
				series.numbers = map[string]float64{}
			}
			series.numbers[id] = number
		}
	}
	return nil
}

// seriesPath returns the path of the feed of series id's books, in their order in it.
func seriesPath(id string) string { return "/opds/series/" + id }
