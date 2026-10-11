package shelf

import (
	"image"
	"image/color"
	"os"
	"slices"
	"testing"
)

// A book downloaded as PNG pages by an older version still reads.
func TestLoadPagePNG(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := os.MkdirAll(store.bookDir("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	paletted := image.NewPaletted(image.Rect(0, 0, 4, 2), color.Palette{color.Gray{Y: 0}, color.Gray{Y: 255}})
	paletted.Pix[1] = 1
	if err := os.WriteFile(store.pagePath("old", 1, pngExtension), encodePNG(t, paletted), 0o644); err != nil {
		t.Fatal(err)
	}
	gray, err := store.LoadPage("old", 1)
	if err != nil {
		t.Fatal(err)
	}
	if gray.Pix[0] != 0 || gray.Pix[1] != 255 {
		t.Fatalf("pixels %v, want 0, 255 first", gray.Pix)
	}
}

// List sorts by series title, then by number, or by title with numbers compared as numbers.
func TestListOrder(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	saveBooks(t, store,
		Book{ID: "b10", SeriesTitle: "B", Title: "Vol. 10"},
		Book{ID: "b2", SeriesTitle: "B", Title: "Vol. 2"},
		Book{ID: "b2b", SeriesTitle: "B", Title: "Vol. 02 extra"},
		Book{ID: "a3", SeriesTitle: "A", Title: "Zeta", Number: 3},
		Book{ID: "a1", SeriesTitle: "A", Title: "Omega", Number: 1},
		Book{ID: "a2", SeriesTitle: "A", Title: "Alpha", Number: 2},
	)
	var ids []string
	for _, book := range store.Books() {
		ids = append(ids, book.ID)
	}
	if want := []string{"a1", "a2", "a3", "b2", "b2b", "b10"}; !slices.Equal(ids, want) {
		t.Errorf("List order = %v, want %v", ids, want)
	}
}

// List sorts series without their leading article, and with a dash in a title as a hyphen.
func TestListOrderTitles(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	saveBooks(t, store,
		Book{ID: "trone", SeriesID: "1", SeriesTitle: "Le Trône de Fer"},
		Book{ID: "dune2", SeriesID: "2", SeriesTitle: "Dune – Tome 2"},
		Book{ID: "seigneur", SeriesID: "3", SeriesTitle: "Le Seigneur des Anneaux"},
		Book{ID: "dune6", SeriesID: "4", SeriesTitle: "Dune - Tome 6"},
		Book{ID: "lyra", SeriesID: "5", SeriesTitle: "Lyra"},
	)
	var ids []string
	for _, book := range store.Books() {
		ids = append(ids, book.ID)
	}
	if want := []string{"dune2", "dune6", "lyra", "seigneur", "trone"}; !slices.Equal(ids, want) {
		t.Errorf("List order = %v, want %v", ids, want)
	}
}

// removeTestStore returns a store of two books of a series, the second to be removed.
func removeTestStore(t *testing.T) Store {
	t.Helper()
	store := Store{Dir: t.TempDir()}
	saveBooks(t, store,
		Book{ID: "b1", SeriesID: "s1", Title: "Vol 1", Pages: 2},
		Book{ID: "b2", SeriesID: "s1", Title: "Vol 2", Pages: 2, Unsynced: true, PendingRemoval: true},
	)
	return store
}

func TestListLeavesOutRemoved(t *testing.T) {
	if books := removeTestStore(t).Books(); len(books) != 1 || books[0].ID != "b1" {
		t.Errorf("List = %+v, want b1 only", books)
	}
}

func TestAllKeepsRemoved(t *testing.T) {
	if books := removeTestStore(t).AllBooks(); len(books) != 2 {
		t.Errorf("All = %+v, want both books", books)
	}
}

func TestGetLeavesOutRemoved(t *testing.T) {
	store := removeTestStore(t)
	if _, ok := store.Book("b2"); ok {
		t.Error("Get returns a book to be removed")
	}
	if _, ok := store.BookEvenIfRemoved("b2"); !ok {
		t.Error("Find doesn't return a book to be removed")
	}
}

func TestSize(t *testing.T) {
	if size := removeTestStore(t).Size("b1"); size == 0 {
		t.Error("the size of a saved book is 0")
	}
}

func TestFreeSpace(t *testing.T) {
	if free, ok := FreeSpace(t.TempDir()); !ok || free <= 0 {
		t.Errorf("FreeSpace = %d, %v", free, ok)
	}
}
