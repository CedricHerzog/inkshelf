package komga

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Book is a Komga book (a volume or a chapter).
type Book struct {
	ID           string        `json:"id"`
	SeriesID     string        `json:"seriesId"`
	LibraryID    string        `json:"libraryId"`
	SeriesTitle  string        `json:"seriesTitle"`
	Name         string        `json:"name"`
	LastModified string        `json:"lastModified"`
	Metadata     BookMetadata  `json:"metadata"`
	Media        BookMedia     `json:"media"`
	ReadProgress *ReadProgress `json:"readProgress"` // nil if the book is unread
}

// BookMetadata is the part of a book's metadata the app uses.
type BookMetadata struct {
	Title      string  `json:"title"`
	NumberSort float64 `json:"numberSort"` // the book's place in its series
}

// BookMedia describes a book's file.
type BookMedia struct {
	PagesCount int `json:"pagesCount"`
}

// ReadProgress is how far a book has been read.
type ReadProgress struct {
	Page      int  `json:"page"` // the last page read, counted from 1
	Completed bool `json:"completed"`
}

// Title returns the book's metadata title, or its name.
func (book Book) Title() string {
	if book.Metadata.Title != "" {
		return book.Metadata.Title
	}
	return book.Name
}

// Modified returns when the book last changed, or the zero time if Komga doesn't say.
func (book Book) Modified() time.Time { return parseModified(book.LastModified) }

// PageInfo is a page of a book as Komga stores it.
type PageInfo struct {
	Number    int    `json:"number"` // counted from 1
	MediaType string `json:"mediaType"`
}

// Conversion is the format Page asks Komga to convert a page to.
type Conversion string

// The conversions.
const (
	Original      Conversion = "" // the page as it is in the book's file
	ConvertToJPEG Conversion = "jpeg"
	ConvertToPNG  Conversion = "png"
)

// Books returns the books of a series, sorted by number.
func (client *Client) Books(ctx context.Context, seriesID string) ([]Book, error) {
	var result resultPage[Book]
	err := client.getJSON(ctx, "/api/v1/series/"+url.PathEscape(seriesID)+"/books?unpaged=true&sort=metadata.numberSort,asc", &result)
	return result.Content, err
}

// InProgress returns at most size books started and not finished, the last read first.
func (client *Client) InProgress(ctx context.Context, size int) ([]Book, error) {
	var result resultPage[Book]
	err := client.getJSON(ctx, fmt.Sprintf("/api/v1/books?read_status=IN_PROGRESS&sort=readProgress.readDate,desc&size=%d", size), &result)
	return result.Content, err
}

// Pages returns the pages of a book.
func (client *Client) Pages(ctx context.Context, bookID string) ([]PageInfo, error) {
	var pages []PageInfo
	err := client.getJSON(ctx, "/api/v1/books/"+url.PathEscape(bookID)+"/pages", &pages)
	return pages, err
}

// Page returns page number (counted from 1) of a book, converted by Komga as convert says.
func (client *Client) Page(ctx context.Context, bookID string, number int, convert Conversion) ([]byte, error) {
	path := fmt.Sprintf("/api/v1/books/%s/pages/%d", url.PathEscape(bookID), number)
	if convert != Original {
		path += "?convert=" + url.QueryEscape(string(convert))
	}
	return client.getBytes(ctx, path)
}

// BookThumbnail returns the cover of a book, as Komga made it (JPEG, about 300 pixels wide).
func (client *Client) BookThumbnail(ctx context.Context, bookID string) ([]byte, error) {
	return client.getBytes(ctx, "/api/v1/books/"+url.PathEscape(bookID)+"/thumbnail")
}

// ClearProgress marks a book unread.
func (client *Client) ClearProgress(ctx context.Context, bookID string) error {
	resp, err := client.do(ctx, http.MethodDelete, "/api/v1/books/"+url.PathEscape(bookID)+"/read-progress", nil)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// progressUpdate is the body of SetProgress's request.
type progressUpdate struct {
	Completed bool `json:"completed"`
	Page      int  `json:"page"`
}

// SetProgress marks page (counted from 1) as the last one read of a book, and the book as
// completed or not.
func (client *Client) SetProgress(ctx context.Context, bookID string, page int, completed bool) error {
	body, err := json.Marshal(progressUpdate{Completed: completed, Page: page})
	if err != nil {
		return err
	}
	resp, err := client.do(ctx, http.MethodPatch, "/api/v1/books/"+url.PathEscape(bookID)+"/read-progress", body)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}
