// Package fakecalibreweb is a Calibre-Web server with a fixed library of generated EPUBs and
// CBZs, for running the app off the reader (the emulator) without a real server. Its OPDS feeds are
// laid out as Calibre-Web's. Its content is always the same.
package fakecalibreweb

import (
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// feedPageSize is how many books a feed page holds: few, so that the app follows next links.
const feedPageSize = 3

// server is the state of a fake Calibre-Web.
type server struct {
	mu    sync.Mutex        // guards files
	files map[string][]byte // generated files and covers, by path
}

// NewHandler returns the handler of a fake Calibre-Web. Any username and password are
// accepted, but they are required, as Basic auth.
func NewHandler() http.Handler {
	fake := &server{files: map[string][]byte{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /opds/new", listBooks)
	mux.HandleFunc("GET /opds/series/letter/00", listSeries)
	mux.HandleFunc("GET /opds/series/{id}", seriesBooks)
	mux.HandleFunc("GET /opds/readbooks", func(writer http.ResponseWriter, request *http.Request) {
		writeFeed(writer, request.URL.Path, "", nil)
	})
	mux.HandleFunc("GET /opds/download/{id}/{format}/", fake.download)
	mux.HandleFunc("GET /opds/cover/{id}", fake.cover)
	return requireBasicAuth(mux)
}

// New starts a fake Calibre-Web, as NewHandler serves it.
func New() *httptest.Server { return httptest.NewServer(NewHandler()) }

// requireBasicAuth answers 401 to the requests without a username, as Calibre-Web's OPDS does.
func requireBasicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if username, _, ok := request.BasicAuth(); !ok || username == "" {
			writer.Header().Set("WWW-Authenticate", `Basic realm="Authentication Required"`)
			http.Error(writer, "Unauthorized Access", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// listBooks sends a page of all the books, the newest first.
func listBooks(writer http.ResponseWriter, request *http.Request) {
	offset, _ := strconv.Atoi(request.URL.Query().Get("offset")) // none or bad: the first page
	offset = min(max(offset, 0), len(books))
	end := min(offset+feedPageSize, len(books))
	next := ""
	if end < len(books) {
		next = fmt.Sprintf("/opds/new?offset=%d", end)
	}
	writeFeed(writer, request.URL.Path, next, bookEntries(books[offset:end]))
}

// listSeries sends every series, as navigation entries.
func listSeries(writer http.ResponseWriter, request *http.Request) {
	var entries []string
	for _, item := range allSeries {
		href := fmt.Sprintf("/opds/series/%d", item.ID)
		entries = append(entries, fmt.Sprintf(`  <entry>
    <title>%s</title>
    <id>%s</id>
    <link rel="subsection" type="application/atom+xml;profile=opds-catalog" href="%s"/>
  </entry>
`, html.EscapeString(item.Title), href, href))
	}
	writeFeed(writer, request.URL.Path, "", entries)
}

// seriesBooks sends the books of a series, in their order in it.
func seriesBooks(writer http.ResponseWriter, request *http.Request) {
	id, _ := strconv.Atoi(request.PathValue("id"))
	index := slices.IndexFunc(allSeries, func(item series) bool { return item.ID == id })
	if index < 0 {
		http.NotFound(writer, request)
		return
	}
	var members []book
	for _, bookID := range allSeries[index].BookIDs {
		if item, ok := bookByID(bookID); ok {
			members = append(members, item)
		}
	}
	writeFeed(writer, request.URL.Path, "", bookEntries(members))
}

// bookEntries returns the entries of books, as Calibre-Web writes them.
func bookEntries(items []book) []string {
	entries := make([]string, len(items))
	for index, item := range items {
		var links strings.Builder
		fmt.Fprintf(&links, `    <link type="image/jpeg" href="/opds/cover/%d" rel="http://opds-spec.org/image"/>
    <link type="image/jpeg" href="/opds/cover/%d" rel="http://opds-spec.org/image/thumbnail"/>
`, item.ID, item.ID)
		for _, format := range item.Formats {
			fmt.Fprintf(&links, `    <link rel="http://opds-spec.org/acquisition" href="/opds/download/%d/%s/"
          length="1000" title="%s" mtime="%s" type="%s"/>
`, item.ID, format, strings.ToUpper(format), item.Updated.Format(time.RFC3339), mimeTypes[format])
		}
		entries[index] = fmt.Sprintf(`  <entry>
    <title>%s</title>
    <id>urn:uuid:00000000-0000-4000-8000-%012d</id>
    <updated>%s</updated>
    <author>
      <name>%s</name>
    </author>
    <dcterms:language>fra</dcterms:language>
    <content type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml">
%s    </div></content>
%s  </entry>
`, html.EscapeString(item.Title), item.ID, item.Updated.Format(time.RFC3339), html.EscapeString(item.Author), seriesLine(item), links.String())
	}
	return entries
}

// seriesLine returns the line of a book's content that names its series and its index there,
// as Calibre-Web writes it ("2.5", but "3" for 3.0), or "" for a book in no series.
func seriesLine(item book) string {
	inSeries, ok := seriesOf(item.ID)
	if !ok {
		return ""
	}
	return fmt.Sprintf("    SERIES: %s [%s]<br/>\n", html.EscapeString(inSeries.Title), strconv.FormatFloat(item.SeriesIndex, 'f', -1, 64))
}

// mimeTypes are the types of the formats, as Calibre-Web gives them.
var mimeTypes = map[string]string{
	"epub":  "application/epub+zip",
	"kepub": "application/kepub+zip",
	"cbz":   "application/x-cbz",
	"pdf":   "application/pdf",
}

// writeFeed sends an OPDS feed of entries, with a next link if next is not "".
func writeFeed(writer http.ResponseWriter, self, next string, entries []string) {
	writer.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	nextLink := ""
	if next != "" {
		nextLink = fmt.Sprintf(`  <link rel="next" title="Next" href="%s"
        type="application/atom+xml;profile=opds-catalog;type=feed;kind=navigation"/>
`, html.EscapeString(next))
	}
	fmt.Fprintf(writer, `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:dc="http://purl.org/dc/terms/" xmlns:dcterms="http://purl.org/dc/terms/">
  <icon>/static/favicon.ico</icon>
  <id>urn:uuid:2853dacf-ed79-42f5-8e8a-a7bb3d1ae6a2</id>
  <updated>2026-10-01T12:00:00+00:00</updated>
  <link rel="self" href="%s?" type="application/atom+xml;profile=opds-catalog;type=feed;kind=navigation"/>
  <link rel="start" href="/opds" type="application/atom+xml;profile=opds-catalog;type=feed;kind=navigation"/>
%s  <title>Calibre-Web</title>
%s</feed>
`, html.EscapeString(self), nextLink, strings.Join(entries, ""))
}

// download sends a book's file, generated on first use.
func (fake *server) download(writer http.ResponseWriter, request *http.Request) {
	id, _ := strconv.Atoi(request.PathValue("id"))
	item, ok := bookByID(id)
	format := request.PathValue("format")
	if !ok || !slices.Contains(item.Formats, format) {
		http.NotFound(writer, request)
		return
	}
	var generate func(book) ([]byte, error)
	switch format {
	case "epub":
		generate = makeEPUB
		if item.FixedLayout {
			generate = makeFixedLayoutEPUB
		}
		if item.Showcase {
			generate = makeShowcaseEPUB
		}
	case "cbz":
		generate = makeCBZ
	default:
		http.Error(writer, "the fake has no "+format, http.StatusNotFound)
		return
	}
	fake.send(writer, request, mimeTypes[format], func() ([]byte, error) { return generate(item) })
}

// cover sends a book's cover, generated on first use.
func (fake *server) cover(writer http.ResponseWriter, request *http.Request) {
	id, _ := strconv.Atoi(request.PathValue("id"))
	item, ok := bookByID(id)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	fake.send(writer, request, "image/jpeg", func() ([]byte, error) { return drawCover(item) })
}

// send sends the file of the request's path, generating it if it isn't cached yet.
func (fake *server) send(writer http.ResponseWriter, request *http.Request, contentType string, generate func() ([]byte, error)) {
	fake.mu.Lock()
	data, ok := fake.files[request.URL.Path]
	fake.mu.Unlock()
	if !ok {
		var err error
		if data, err = generate(); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		fake.mu.Lock()
		fake.files[request.URL.Path] = data
		fake.mu.Unlock()
	}
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = writer.Write(data) // the client went away: nothing to do
}
