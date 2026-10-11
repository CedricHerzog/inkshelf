package shelf

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// The files of the downloaded books must keep their format: the readers' files are not
// rewritten when the app is updated. These tests read files as the app saves them, and save
// them again.

// goldenDir holds a store of two books and an index.json.
var goldenDir = filepath.Join("testdata", "golden")

// readFixture returns a file of goldenDir, without its final newline.
func readFixture(t *testing.T, path ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{goldenDir}, path...)...))
	if err != nil {
		t.Fatal(err)
	}
	return bytes.TrimSpace(data)
}

func TestBookGolden(t *testing.T) {
	golden := Store{Dir: filepath.Join(goldenDir, "books")}
	pageBook, ok := golden.BookEvenIfRemoved("komga-42")
	if !ok {
		t.Fatal("komga-42 not found")
	}
	want := Book{
		ID: "komga-42", SeriesID: "series-1", LibraryID: "lib-manga", SeriesTitle: "Alpha Saga", Title: "Vol. 2",
		Pages: 187, Format: servers.ComicArchive, Number: 2.5, Direction: servers.RightToLeft, Page: 64,
		Unsynced: true, PendingRemoval: true, ReadAt: time.Date(2025, time.March, 14, 9, 41, 0, 0, time.UTC),
	}
	if !reflect.DeepEqual(pageBook, want) {
		t.Errorf("komga-42: got %+v, want %+v", pageBook, want)
	}
	textBook, ok := golden.BookEvenIfRemoved("epub-7")
	if !ok {
		t.Fatal("epub-7 not found")
	}
	if textBook.Format != servers.Reflowable || !textBook.Completed || textBook.Location == nil ||
		*textBook.Location != (Location{Chapter: 3, Offset: 1534}) || textBook.ReadAt.Unix() != 1741896330 {
		t.Errorf("epub-7: got %+v", textBook)
	}

	saved := Store{Dir: t.TempDir()}
	for _, book := range []Book{pageBook, textBook} {
		if err := os.MkdirAll(saved.bookDir(book.ID), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := saved.Save(book); err != nil {
			t.Fatal(err)
		}
		encoded, err := os.ReadFile(filepath.Join(saved.Dir, book.ID, bookFile))
		if err != nil {
			t.Fatal(err)
		}
		if fixture := readFixture(t, "books", book.ID, bookFile); !bytes.Equal(encoded, fixture) {
			t.Errorf("%s saved again as\n%s\nwant\n%s", book.ID, encoded, fixture)
		}
	}
}

func TestIndexGolden(t *testing.T) {
	fixture := readFixture(t, indexFile)
	var index epub.Index
	if err := json.Unmarshal(fixture, &index); err != nil {
		t.Fatal(err)
	}
	if index.Title != "Les Étoiles" || index.Language != "fr" || index.Total != 21910 || index.PageProgression != "rtl" ||
		len(index.Chapters) != 3 || index.Chapters[1] != (epub.ChapterIndex{Path: "OEBPS/chapter1.xhtml", Title: "Chapitre 1", Chars: 12034}) ||
		len(index.TOC) != 2 || index.TOC[1] != (epub.TOCEntry{Title: "Scène 2", Level: 1, Chapter: 1, Offset: 5120}) ||
		len(index.PageList) != 2 || len(index.Landmarks) != 2 || index.Landmarks[1].Type != "bodymatter" {
		t.Errorf("read %+v", index)
	}
	encoded, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, fixture) {
		t.Errorf("encoded again as\n%s\nwant\n%s", encoded, fixture)
	}

	// LoadIndex reads an index of the current version as it was saved, without the EPUB.
	index.Version = epub.IndexVersion
	store := Store{Dir: t.TempDir()}
	if err := os.MkdirAll(store.bookDir("epub-7"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.saveIndex("epub-7", index); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadIndex("epub-7", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, index) {
		t.Errorf("LoadIndex: got %+v, want %+v", loaded, index)
	}
}
