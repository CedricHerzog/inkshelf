package state

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"slices"

	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// Preferences are what the reader chose and the app keeps across runs, per server: the
// filters of the lists, the reading direction of each series, and the size of book text.
// They also keep the server's libraries, so that the library filter works offline. They are
// saved in the server's filters.json, whose name and format stay as older versions wrote them.
type Preferences struct {
	Library   string            `json:"library"` // a library's ID; "" is every library
	Status    string            `json:"status"`  // the Name of one of StatusFilters
	Books     string            `json:"books"`   // the Name of one of BookFilters
	Libraries []servers.Library `json:"libraries"`
	// Directions are the reading directions chosen on the reader, by series ID. They win over
	// the server's.
	Directions map[string]servers.Direction `json:"directions,omitempty"`
	// TextSize is the font size of book text, in InkPad 3 pixels; 0 until the reader picks one.
	TextSize int `json:"textSize,omitempty"`
}

// StatusFilter is a choice of the series list's read status filter.
type StatusFilter struct {
	Name   string // saved in filters.json; "" is All
	Label  string
	Status servers.ReadStatus
}

// StatusFilters are the choices of the read status filter, in the order shown.
var StatusFilters = []StatusFilter{
	{"", "All", 0},
	{"reading", "Reading", servers.Reading},
	{"unread", "Unread", servers.Unread},
	{"read", "Read", servers.Read},
}

// Names of the book list's filters, saved in filters.json.
const (
	BooksDownloaded = "downloaded"
	BooksUnread     = "unread" // not read to the end: the books in progress stay
)

// BookFilter is a choice of the book list's filter.
type BookFilter struct {
	Name  string // saved in filters.json; "" is All
	Label string
}

// BookFilters are the choices of the book list's filter, in the order shown.
var BookFilters = []BookFilter{
	{"", "All"},
	{BooksDownloaded, "Downloaded"},
	{BooksUnread, "Unread"},
}

func preferencesPath(dir string) string { return filepath.Join(dir, "filters.json") }

// LoadPreferences reads the preferences saved in dir. Until some are saved, everything shows.
func LoadPreferences(dir string) Preferences {
	var saved Preferences
	data, err := os.ReadFile(preferencesPath(dir))
	if err != nil {
		return saved // none saved yet
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		log.Printf("reading filters: %v", err)
	}
	return saved
}

// Save writes the preferences in dir, through a temporary file so that a reader switched off
// meanwhile keeps the previous ones whole.
func (saved Preferences) Save(dir string) error {
	data, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	tempPath := preferencesPath(dir) + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		// Clean up what may have been written; the write error is the one to report.
		_ = os.Remove(tempPath)
		return err
	}
	return os.Rename(tempPath, preferencesPath(dir))
}

// Match reports whether item passes the series list's filters.
func (saved Preferences) Match(item SeriesItem) bool {
	if saved.Library != "" && item.LibraryID != saved.Library {
		return false
	}
	for _, choice := range StatusFilters {
		if choice.Name != "" && choice.Name == saved.Status {
			return item.Status() == choice.Status
		}
	}
	return true
}

// StatusIndex is the index in StatusFilters of the read status filter.
func (saved Preferences) StatusIndex() int {
	index := slices.IndexFunc(StatusFilters, func(choice StatusFilter) bool { return choice.Name == saved.Status })
	return max(index, 0) // 0, All, for a name this version doesn't know
}

// BookIndex is the index in BookFilters of the book list's filter.
func (saved Preferences) BookIndex() int {
	index := slices.IndexFunc(BookFilters, func(choice BookFilter) bool { return choice.Name == saved.Books })
	return max(index, 0) // 0, All, for a name this version doesn't know
}

// LibraryIndex is the index in Libraries of the library filter, or -1 for every library.
func (saved Preferences) LibraryIndex() int {
	return slices.IndexFunc(saved.Libraries, func(library servers.Library) bool { return library.ID == saved.Library })
}
