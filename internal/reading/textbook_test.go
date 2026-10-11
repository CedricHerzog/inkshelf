package reading

import (
	"image"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/calibreweb"
	"github.com/CedricHerzog/inkshelf/internal/servers/calibreweb/fakecalibreweb"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
)

// Virtual pages and percents count the runes from the start of the book, across chapters.
func TestVirtualPageAndPercent(t *testing.T) {
	text := &TextBook{Index: epub.Index{
		Chapters: []epub.ChapterIndex{{Chars: 1000}, {Chars: 3 * shelf.CharsPerPage}, {Chars: 1000}},
		Total:    2000 + 3*shelf.CharsPerPage,
	}}
	location := shelf.Location{Chapter: 1, Offset: shelf.CharsPerPage}
	if got := text.global(location); got != 1000+shelf.CharsPerPage {
		t.Errorf("global = %d", got)
	}
	if got, want := text.VirtualPage(location, 100), (1000+shelf.CharsPerPage)/shelf.CharsPerPage+1; got != want {
		t.Errorf("VirtualPage = %d, want %d", got, want)
	}
	if got := text.VirtualPage(location, 1); got != 1 {
		t.Errorf("VirtualPage past the count = %d, want the last, 1", got)
	}
	if got := text.Percent(shelf.Location{Chapter: 2, Offset: 1000}); got != 100 {
		t.Errorf("Percent at the end = %d", got)
	}
	if got := (&TextBook{}).Percent(shelf.Location{}); got != 0 {
		t.Errorf("Percent of an empty book = %d", got)
	}
}

// openFakeBook downloads the fake Calibre-Web's first EPUB and opens it on a small page.
func openFakeBook(t *testing.T) (*TextBook, shelf.Book) {
	t.Helper()
	server := fakecalibreweb.New()
	t.Cleanup(server.Close)
	config := servers.Config{Name: "Books", Type: "calibreweb", Server: server.URL, Username: "me", Password: "secret"}
	source := calibreweb.NewServer(config, server.Client())
	books, err := source.Books(t.Context(), "1")
	if err != nil {
		t.Fatal(err)
	}
	book := shelf.Book{ID: books[0].ID, SeriesID: "1", Title: books[0].Title, Format: books[0].Format}
	store := shelf.Store{Dir: t.TempDir()}
	if err := store.Download(t.Context(), source, book, 702, 936, func(shelf.DownloadProgress) {}); err != nil {
		t.Fatal(err)
	}
	saved, _ := store.Book(book.ID)
	text, err := Open(store, saved, testSettings(24))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(text.Close)
	return text, saved
}

// testSettings lays text out at fontSize on a page of 702×936.
func testSettings(fontSize int) layout.Settings {
	return layout.Settings{Width: 702, Height: 936, Margins: image.Rect(40, 40, 40, 50), FontSize: fontSize, LineSpacing: 135, Justify: true, HideFootnotes: true}
}

// Turning forward then back crosses chapters and comes back to the first page; the last page
// marks the book read.
func TestTurnPages(t *testing.T) {
	text, book := openFakeBook(t)
	first := text.At()
	turns := 0
	for !text.AtLastPage() {
		if moved, err := text.Turn(true); err != nil || !moved {
			t.Fatalf("turn %d: %v, %v", turns, moved, err)
		}
		turns++
	}
	if text.At().Chapter == first.Chapter {
		t.Fatalf("the last page %+v is in the first chapter", text.At())
	}
	if moved, err := text.Turn(true); err != nil || moved {
		t.Errorf("a turn past the last page: %v, %v", moved, err)
	}
	if err := text.UpdateProgress(&book); err != nil || !book.Completed || book.Page != book.Pages {
		t.Errorf("progress at the end %+v, %v", book, err)
	}
	for range turns {
		if moved, err := text.Turn(false); err != nil || !moved {
			t.Fatalf("back: %v, %v", moved, err)
		}
	}
	if text.At() != first {
		t.Errorf("back at %+v, want %+v", text.At(), first)
	}
	if moved, _ := text.Turn(false); moved {
		t.Error("a turn back from the first page")
	}
}

// A larger font keeps the place read, a location surviving the new layout.
func TestRelayoutKeepsLocation(t *testing.T) {
	text, _ := openFakeBook(t)
	for range 3 {
		if _, err := text.Turn(true); err != nil {
			t.Fatal(err)
		}
	}
	location, err := text.Location(text.At())
	if err != nil {
		t.Fatal(err)
	}
	if err := text.Relayout(testSettings(40), location); err != nil {
		t.Fatal(err)
	}
	after, err := text.Location(text.At())
	if err != nil {
		t.Fatal(err)
	}
	if after.Chapter != location.Chapter || after.Offset > location.Offset {
		t.Errorf("relaid out at %+v, want the page holding %+v", after, location)
	}
}

// A jump keeps the page left for Return; a link out of the book and a footnote open a popup,
// a link to a chapter jumps.
func TestFollowLinks(t *testing.T) {
	text, _ := openFakeBook(t)
	page, err := text.Page(text.At())
	if err != nil {
		t.Fatal(err)
	}
	popup := PopupPage{Height: 400, MarginTop: 30, MarginBottom: 50}
	kinds := map[string]bool{}
	for _, link := range page.Links {
		pages, location, err := text.Follow(link.Target, popup)
		if err != nil {
			t.Fatalf("%s: %v", link.Target, err)
		}
		switch {
		case strings.HasPrefix(link.Target, "https:"):
			kinds["web"] = len(pages) > 0
		case strings.Contains(link.Target, "#fn1"):
			kinds["note"] = len(pages) > 0
		case len(pages) == 0:
			kinds["jump"] = true
			start := text.At()
			if err := text.Jump(location); err != nil || !text.CanReturn() {
				t.Fatalf("jump to %+v: %v", location, err)
			}
			if err := text.Return(); err != nil || text.At() != start || text.CanReturn() {
				t.Fatalf("returned to %+v, want %+v (%v)", text.At(), start, err)
			}
		}
	}
	for _, kind := range []string{"web", "note", "jump"} {
		if !kinds[kind] {
			t.Errorf("no %s link followed as such among %d links: %v", kind, len(page.Links), kinds)
		}
	}
	pages, err := text.MessagePages("Cannot follow this link", popup)
	if err != nil || len(pages) != 1 {
		t.Fatalf("MessagePages = %d pages, %v", len(pages), err)
	}
	drawn, err := text.DrawPopupPage(pages[0], popup.Height)
	if err != nil || drawn.Bounds().Dx() != 702 || drawn.Bounds().Dy() != popup.Height {
		t.Errorf("DrawPopupPage = %v, %v", drawn.Bounds(), err)
	}
}

// A page drawn is kept until the reader moves two pages away; a closed book keeps nothing.
func TestDrawnPages(t *testing.T) {
	text, _ := openFakeBook(t)
	at := text.At()
	neighbours := text.Neighbours(at)
	if len(neighbours) != 1 {
		t.Fatalf("neighbours of the first page %+v", neighbours)
	}
	if !text.WantsDrawn(neighbours[0], at) {
		t.Fatal("the next page isn't wanted")
	}
	drawn, err := text.Draw(neighbours[0])
	if err != nil {
		t.Fatal(err)
	}
	text.KeepDrawn(neighbours[0], drawn)
	if text.Drawn(neighbours[0]) != drawn || text.WantsDrawn(neighbours[0], at) {
		t.Fatal("the page drawn wasn't kept")
	}
	for range 3 {
		if _, err := text.Turn(true); err != nil {
			t.Fatal(err)
		}
	}
	text.Prune()
	if text.Drawn(neighbours[0]) != nil {
		t.Error("a page three pages back was kept")
	}
	text.Close()
	text.KeepDrawn(at, drawn)
	if text.Drawn(at) != nil {
		t.Error("a closed book kept a page")
	}
	if _, err := text.Draw(at); err == nil {
		t.Error("a closed book drew a page")
	}
}
