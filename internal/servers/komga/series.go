package komga

import (
	"context"
	"net/url"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// Series is a Komga series.
type Series struct {
	ID                   string         `json:"id"`
	LibraryID            string         `json:"libraryId"`
	Name                 string         `json:"name"`
	BooksCount           int            `json:"booksCount"`
	BooksReadCount       int            `json:"booksReadCount"`
	BooksInProgressCount int            `json:"booksInProgressCount"`
	LastModified         string         `json:"lastModified"`
	Metadata             SeriesMetadata `json:"metadata"`
}

// SeriesMetadata is the part of a series' metadata the app uses.
type SeriesMetadata struct {
	Title     string `json:"title"`
	TitleSort string `json:"titleSort"`
	// ReadingDirection is LEFT_TO_RIGHT, RIGHT_TO_LEFT, VERTICAL, WEBTOON or "".
	ReadingDirection string `json:"readingDirection"`
}

// Title returns the series' metadata title, or its name.
func (series Series) Title() string {
	if series.Metadata.Title != "" {
		return series.Metadata.Title
	}
	return series.Name
}

// Modified returns when the series last changed, or the zero time if Komga doesn't say.
func (series Series) Modified() time.Time { return parseModified(series.LastModified) }

// parseModified parses a lastModified date of Komga: RFC 3339, or without a time zone (taken
// as UTC). It returns the zero time if it cannot.
func parseModified(date string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999"} {
		if parsed, err := time.Parse(layout, date); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// Direction returns how the series reads: right to left, left to right (also for vertical and
// webtoon series, whose next page is below), or "" if Komga doesn't say.
func (series Series) Direction() servers.Direction {
	switch series.Metadata.ReadingDirection {
	case "RIGHT_TO_LEFT":
		return servers.RightToLeft
	case "LEFT_TO_RIGHT", "VERTICAL", "WEBTOON":
		return servers.LeftToRight
	}
	return ""
}

// Series returns all the series, sorted by title.
func (client *Client) Series(ctx context.Context) ([]Series, error) {
	var result resultPage[Series]
	err := client.getJSON(ctx, "/api/v1/series?unpaged=true&sort=metadata.titleSort,asc", &result)
	return result.Content, err
}

// SeriesThumbnail returns the cover of a series, as Komga made it (JPEG, about 300 pixels wide).
func (client *Client) SeriesThumbnail(ctx context.Context, seriesID string) ([]byte, error) {
	return client.getBytes(ctx, "/api/v1/series/"+url.PathEscape(seriesID)+"/thumbnail")
}
