package fakekomga

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"
)

// bookJSON is a book as Komga lists it.
type bookJSON struct {
	ID           string         `json:"id"`
	SeriesID     string         `json:"seriesId"`
	LibraryID    string         `json:"libraryId"`
	SeriesTitle  string         `json:"seriesTitle"`
	Name         string         `json:"name"`
	LastModified string         `json:"lastModified"`
	Metadata     map[string]any `json:"metadata"`
	Media        map[string]int `json:"media"`
	Progress     *progress      `json:"readProgress"`
}

// bookLocked returns book number of series item as Komga lists it. The caller holds fake.mu.
func (fake *server) bookLocked(item series, number int) bookJSON {
	book := bookJSON{
		ID:           bookID(item.id, number),
		SeriesID:     item.id,
		LibraryID:    item.library,
		SeriesTitle:  item.title,
		Name:         fmt.Sprintf("Volume %d", number),
		LastModified: lastModified,
		Metadata:     map[string]any{"title": fmt.Sprintf("%s, Vol. %d", item.title, number), "numberSort": number},
		Media:        map[string]int{"pagesCount": item.pages},
	}
	if read, ok := fake.progress[book.ID]; ok {
		book.Progress = &read
	}
	return book
}

func listLibraries(writer http.ResponseWriter, request *http.Request) {
	type libraryJSON struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	content := []libraryJSON{}
	for _, entry := range libraries {
		content = append(content, libraryJSON{ID: entry.id, Name: entry.name})
	}
	writeJSON(writer, content)
}

func (fake *server) listSeries(writer http.ResponseWriter, request *http.Request) {
	type metadata struct {
		Title            string `json:"title"`
		TitleSort        string `json:"titleSort"`
		ReadingDirection string `json:"readingDirection"`
	}
	type seriesJSON struct {
		ID                   string   `json:"id"`
		LibraryID            string   `json:"libraryId"`
		Name                 string   `json:"name"`
		BooksCount           int      `json:"booksCount"`
		BooksReadCount       int      `json:"booksReadCount"`
		BooksInProgressCount int      `json:"booksInProgressCount"`
		LastModified         string   `json:"lastModified"`
		Metadata             metadata `json:"metadata"`
	}
	content := []seriesJSON{}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	for _, item := range catalog {
		entry := seriesJSON{
			ID:           item.id,
			LibraryID:    item.library,
			Name:         item.id,
			BooksCount:   item.books,
			LastModified: lastModified,
			Metadata: metadata{
				Title:            item.title,
				TitleSort:        cmp.Or(item.titleSort, item.title),
				ReadingDirection: item.direction,
			},
		}
		for number := range item.bookNumbers() {
			read, ok := fake.progress[bookID(item.id, number)]
			switch {
			case ok && read.Completed:
				entry.BooksReadCount++
			case ok:
				entry.BooksInProgressCount++
			}
		}
		content = append(content, entry)
	}
	writeJSON(writer, map[string]any{"content": content})
}

func (fake *server) listBooks(writer http.ResponseWriter, request *http.Request) {
	content := []bookJSON{}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	for _, item := range catalog {
		if item.id != request.PathValue("series") {
			continue
		}
		for number := range item.bookNumbers() {
			content = append(content, fake.bookLocked(item, number))
		}
	}
	writeJSON(writer, map[string]any{"content": content})
}

// inProgress lists the books started and not finished, the last read first. It does only
// what the app asks of Komga's GET /api/v1/books: the read_status=IN_PROGRESS filter, and size.
func (fake *server) inProgress(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Query().Get("read_status") != "IN_PROGRESS" {
		http.Error(writer, "only read_status=IN_PROGRESS", http.StatusBadRequest)
		return
	}
	content := []bookJSON{}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	for _, item := range catalog {
		for number := range item.bookNumbers() {
			if read, ok := fake.progress[bookID(item.id, number)]; ok && read.Page > 0 && !read.Completed {
				content = append(content, fake.bookLocked(item, number))
			}
		}
	}
	slices.SortStableFunc(content, func(left, right bookJSON) int {
		return fake.progress[right.ID].at.Compare(fake.progress[left.ID].at)
	})
	if size, err := strconv.Atoi(request.URL.Query().Get("size")); err == nil && size < len(content) {
		content = content[:size]
	}
	writeJSON(writer, map[string]any{"content": content})
}

func (fake *server) listPages(writer http.ResponseWriter, request *http.Request) {
	item, book, ok := findBook(request.PathValue("book"))
	if !ok {
		http.NotFound(writer, request)
		return
	}
	type pageJSON struct {
		Number    int    `json:"number"`
		MediaType string `json:"mediaType"`
	}
	pages := []pageJSON{}
	for number := range item.pageNumbers() {
		pages = append(pages, pageJSON{Number: number, MediaType: mediaType(item, book, number)})
	}
	writeJSON(writer, pages)
}

// page serves a page as listed, or converted. A WebP page can't be encoded here: it is served
// as PNG, which the app's decoder recognizes by its content anyway.
func (fake *server) page(writer http.ResponseWriter, request *http.Request) {
	item, book, ok := findBook(request.PathValue("book"))
	number, err := strconv.Atoi(request.PathValue("number"))
	if !ok || err != nil || number < 1 || number > item.pages {
		http.NotFound(writer, request)
		return
	}
	format := "png"
	switch request.URL.Query().Get("convert") {
	case "jpeg":
		format = "jpeg"
	case "":
		if mediaType(item, book, number) == "image/jpeg" {
			format = "jpeg"
		}
	}
	data, err := fake.encodedPage(item, book, number, format)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writeImage(writer, format, data)
}

func (fake *server) seriesThumbnail(writer http.ResponseWriter, request *http.Request) {
	for _, item := range catalog {
		if item.id == request.PathValue("series") && !item.noCovers {
			fake.writeCover(writer, item, 0)
			return
		}
	}
	http.NotFound(writer, request)
}

func (fake *server) bookThumbnail(writer http.ResponseWriter, request *http.Request) {
	item, book, ok := findBook(request.PathValue("book"))
	if !ok || item.noCovers {
		http.NotFound(writer, request)
		return
	}
	fake.writeCover(writer, item, book)
}

// writeCover serves the cover of book (0: of the series) as a JPEG.
func (fake *server) writeCover(writer http.ResponseWriter, item series, book int) {
	data, err := fake.encodedCover(item, book)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writeImage(writer, "jpeg", data)
}

func (fake *server) setProgress(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("book")
	if _, _, ok := findBook(id); !ok {
		http.NotFound(writer, request)
		return
	}
	var read progress
	if err := json.NewDecoder(request.Body).Decode(&read); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	read.at = time.Now()
	fake.mu.Lock()
	fake.progress[id] = read
	fake.mu.Unlock()
	writer.WriteHeader(http.StatusNoContent)
}

func (fake *server) clearProgress(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("book")
	if _, _, ok := findBook(id); !ok {
		http.NotFound(writer, request)
		return
	}
	fake.mu.Lock()
	delete(fake.progress, id)
	fake.mu.Unlock()
	writer.WriteHeader(http.StatusNoContent)
}
