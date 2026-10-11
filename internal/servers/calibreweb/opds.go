package calibreweb

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The OPDS paths of Calibre-Web.
const (
	// allBooksPath lists every book, the newest first, 60 per page by default.
	allBooksPath = "/opds/new"
	// allSeriesPath lists every series, under the "all" letter of the series index.
	allSeriesPath = "/opds/series/letter/00"
)

// The link relations of OPDS.
const (
	relNext        = "next"
	relSubsection  = "subsection"
	relAcquisition = "http://opds-spec.org/acquisition"
	relCover       = "http://opds-spec.org/image"
	relThumbnail   = "http://opds-spec.org/image/thumbnail"
)

// maxFeedPages is how many pages of a feed are followed at most, in case a server's next link
// loops.
const maxFeedPages = 1000

// feed is an OPDS feed (Atom): a page of a list of books or of navigation entries.
type feed struct {
	Entries []entry `xml:"entry"`
	Links   []link  `xml:"link"`
}

// entry is a book, or a link to another feed (a series).
type entry struct {
	Title   string    `xml:"title"`
	ID      string    `xml:"id"`
	Updated time.Time `xml:"updated"`
	Links   []link    `xml:"link"`
	Content struct {
		XML string `xml:",innerxml"`
	} `xml:"content"` // a book's tags, series and description, as XHTML
}

// link is a link of a feed or an entry.
type link struct {
	Rel    string `xml:"rel,attr"`
	Href   string `xml:"href,attr"`
	Type   string `xml:"type,attr"`
	Title  string `xml:"title,attr"`
	Length int64  `xml:"length,attr"`
}

// linkTo returns the first link of links with relation rel, and false if there is none.
func linkTo(links []link, rel string) (link, bool) {
	for _, candidate := range links {
		if candidate.Rel == rel {
			return candidate, true
		}
	}
	return link{}, false
}

// readFeed fetches the feed at path and the pages after it, and returns all their entries.
func (client *Client) readFeed(ctx context.Context, path string) ([]entry, error) {
	var entries []entry
	for range maxFeedPages {
		data, err := client.getBytes(ctx, path)
		if err != nil {
			return nil, err
		}
		var page feed
		if err := xml.Unmarshal(data, &page); err != nil {
			return nil, fmt.Errorf("GET %s: %w", path, err)
		}
		entries = append(entries, page.Entries...)
		next, ok := linkTo(page.Links, relNext)
		if !ok || next.Href == "" || next.Href == path {
			return entries, nil
		}
		path = next.Href
	}
	return nil, fmt.Errorf("GET %s: more than %d pages", path, maxFeedPages)
}

// bookIDPattern finds the Calibre id of a book in its download or cover links.
var bookIDPattern = regexp.MustCompile(`^/opds/(?:download|cover)/(\d+)(?:/|$)`)

// bookID returns the Calibre id of the book entry is, and false if it is no book.
func (item entry) bookID() (string, bool) {
	for _, candidate := range item.Links {
		if match := bookIDPattern.FindStringSubmatch(candidate.Href); match != nil {
			return match[1], true
		}
	}
	return "", false
}

// acquisitions returns the files of the book entry is, by format in lower case ("epub",
// "cbz"…), from its acquisition links' titles or types.
func (item entry) acquisitions() map[string]link {
	files := map[string]link{}
	for _, candidate := range item.Links {
		if candidate.Rel != relAcquisition {
			continue
		}
		if format := formatOf(candidate); format != "" {
			files[format] = candidate
		}
	}
	return files
}

// formatPattern finds the format in a Calibre-Web download link: /opds/download/76/epub/.
var formatPattern = regexp.MustCompile(`^/opds/download/\d+/(\w+)/?$`)

// formatOf returns the format of an acquisition link in lower case, "" if unknown.
func formatOf(acquisition link) string {
	if match := formatPattern.FindStringSubmatch(acquisition.Href); match != nil {
		return match[1]
	}
	switch acquisition.Type {
	case "application/epub+zip":
		return epubFormat
	case "application/vnd.comicbook+zip", "application/x-cbz":
		return cbzFormat
	}
	return ""
}

// seriesPattern finds a book's series and its index in it in the content of its entry, where
// Calibre-Web writes "SERIES: Eragon [5]<br/>": the index without ".0" when whole, "2.5"
// otherwise. The name is escaped as XML and may hold brackets: the last ones hold the index.
var seriesPattern = regexp.MustCompile(`SERIES: (.*) \[(-?\d+(?:\.\d+)?)\]`)

// seriesIndex returns the place in series title of the book entry is, and false if its entry
// gives none for that series. Calibre-Web only names a book's first series.
func (item entry) seriesIndex(title string) (float64, bool) {
	match := seriesPattern.FindStringSubmatch(item.Content.XML)
	if len(match) != 3 || strings.TrimSpace(html.UnescapeString(match[1])) != title {
		return 0, false
	}
	index, err := strconv.ParseFloat(match[2], 64)
	return index, err == nil
}
