package mealie

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"reflect"
	"slices"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/httpclient"
	"github.com/CedricHerzog/inkshelf/internal/servers/mealie/fakemealie"
	"github.com/CedricHerzog/inkshelf/internal/servers/mealie/recipepages"
)

// newTestServer returns the Server of a fake Mealie, closed at the end of the test.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	server := fakemealie.New()
	t.Cleanup(server.Close)
	return NewServer(servers.Config{Name: "test", Type: "mealie", Server: server.URL, APIKey: "token"}, server.Client())
}

// A series per category, and one of the recipes in none, sorted by title.
func TestSeries(t *testing.T) {
	allSeries, err := newTestServer(t).Series(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, series := range allSeries {
		got = append(got, fmt.Sprintf("%s:%d", series.Title, series.Books))
		if series.Direction != servers.LeftToRight || series.Modified.IsZero() {
			t.Errorf("series %+v", series)
		}
	}
	want := []string{"Desserts:2", "Entrées:1", "Pâtisserie:2", "Plats:1", "Uncategorized:1"}
	if !slices.Equal(got, want) {
		t.Fatalf("series %v, want %v", got, want)
	}
}

// Books are the category's recipes by name, and OpenBook draws as many pages as Books counts.
func TestBooksAndPages(t *testing.T) {
	mealie := newTestServer(t)
	books, err := mealie.Books(t.Context(), "c-plats")
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 || books[0].Title != "Bœuf bourguignon" || books[0].SeriesTitle != "Plats" || books[0].Number != 1 {
		t.Fatalf("books %+v", books)
	}
	book := books[0]
	if book.Pages < 2 {
		t.Fatalf("the long recipe has %d pages, want several", book.Pages)
	}
	pages, err := mealie.OpenBook(t.Context(), book.ID, book.Pages)
	if err != nil {
		t.Fatal(err)
	}
	for number := 1; number <= book.Pages; number++ {
		data, err := pages.Page(t.Context(), number)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("page %d: %v", number, err)
		}
		if decoded.Bounds() != image.Rect(0, 0, recipepages.Width, recipepages.Height) {
			t.Fatalf("page %d is %v", number, decoded.Bounds())
		}
	}
}

func TestUncategorized(t *testing.T) {
	books, err := newTestServer(t).Books(t.Context(), uncategorizedID)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 || books[0].Title != "Pâte à pizza" || books[0].Pages < 1 {
		t.Fatalf("books %+v", books)
	}
}

func TestThumbnails(t *testing.T) {
	mealie := newTestServer(t)
	if _, err := mealie.Series(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := mealie.BookThumbnail(t.Context(), "r-gateau"); err != nil {
		t.Errorf("thumbnail of a recipe with a photo: %v", err)
	}
	if _, err := mealie.BookThumbnail(t.Context(), "r-soupe"); err == nil {
		t.Error("thumbnail of a recipe without a photo: no error")
	}
	if _, err := mealie.SeriesThumbnail(t.Context(), "c-desserts"); err != nil {
		t.Errorf("thumbnail of a category: %v", err)
	}
	if _, err := mealie.SeriesThumbnail(t.Context(), "c-entrees"); err == nil {
		t.Error("thumbnail of a category without photos: no error")
	}
}

func TestProgressUnsupported(t *testing.T) {
	mealie := newTestServer(t)
	if err := mealie.SetProgress(t.Context(), "r-gateau", 1, true); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("SetProgress: %v, want ErrUnsupported", err)
	}
	if _, err := mealie.InProgress(t.Context(), 1); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("InProgress: %v, want ErrUnsupported", err)
	}
}

func TestTokenRequired(t *testing.T) {
	server := fakemealie.New()
	t.Cleanup(server.Close)
	mealie := NewServer(servers.Config{Name: "test", Server: server.URL}, server.Client())
	_, err := mealie.Series(t.Context())
	var statusErr *httpclient.StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != 401 {
		t.Fatalf("error %v, want a 401", err)
	}
}

// Empty ingredients, steps and notes stay empty, for recipepages to leave out, and a step
// without text shows its summary.
func TestPageRecipe(t *testing.T) {
	recipe := Recipe{
		Name:         "Test",
		Ingredients:  []Ingredient{{Display: "sel"}, {}},
		Instructions: []Instruction{{Text: "un"}, {}, {Summary: "deux"}},
		Notes:        []Note{{}},
	}
	got := pageRecipe(recipe)
	want := recipepages.Recipe{
		Name:        "Test",
		Ingredients: []recipepages.Entry{{Text: "sel"}, {}},
		Steps:       []recipepages.Entry{{Text: "un"}, {}, {Text: "deux"}},
		Notes:       []recipepages.Entry{{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pageRecipe = %+v, want %+v", got, want)
	}
}

// The note alone is shown when it is the whole line, Mealie's display otherwise.
func TestIngredientText(t *testing.T) {
	butter := &Food{Name: "beurre", PluralName: "beurre"}
	water := &Food{Name: "eau", PluralName: "eaux"}
	for _, test := range []struct {
		ingredient Ingredient
		want       string
	}{
		{Ingredient{Display: "100 grammes beurre 100g de beurre pommade", Note: "100g de beurre pommade", Food: butter}, "100g de beurre pommade"},
		{Ingredient{Display: "1 noix de beurre fondu", Note: "Beurre fondu", Food: butter}, "Beurre fondu"},
		{Ingredient{Display: "100 millilitres eau par pizza", Note: "par pizza", Food: water}, "100 millilitres eau par pizza"},
		{Ingredient{Display: "2 carottes"}, "2 carottes"},
		{Ingredient{Note: "sel"}, "sel"},
	} {
		if got := ingredientText(test.ingredient); got != test.want {
			t.Errorf("ingredientText(%+v) = %q, want %q", test.ingredient, got, test.want)
		}
	}
}
