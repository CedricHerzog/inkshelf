package mealie

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"slices"
	"sync"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/mealie/recipepages"
)

// uncategorized is the id and title of the series of the recipes in no category.
const (
	uncategorizedID    = "uncategorized"
	uncategorizedTitle = "Uncategorized"
)

// Server is a Mealie server as the app sees it: a series per category, a book per recipe,
// whose pages are the recipe laid out and drawn. Mealie has no read progress: the reader keeps
// it.
type Server struct {
	client *Client

	mu sync.Mutex // guards recipes and laidOut, used by the app's goroutines at once
	// recipes are the recipes of the last list, by id: their slug and photo, for the calls
	// given only an id.
	recipes map[string]RecipeSummary
	// laidOut are the recipes laid out so far, by id, so that Books and OpenBook agree on the
	// number of pages without fetching a recipe twice.
	laidOut map[string]laidOutRecipe
}

// Server implements servers.Server.
var _ servers.Server = (*Server)(nil)

// NewServer returns the Server of the server of config, which sends its requests through
// httpClient.
func NewServer(config servers.Config, httpClient *http.Client) *Server {
	return &Server{
		client:  NewClient(config, httpClient),
		recipes: map[string]RecipeSummary{},
		laidOut: map[string]laidOutRecipe{},
	}
}

// Series returns a series per category that has recipes, and one of the recipes in none,
// sorted by title.
func (server *Server) Series(ctx context.Context) ([]servers.Series, error) {
	recipes, err := server.listRecipes(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[string]*servers.Series{}
	for _, recipe := range recipes {
		for _, category := range recipeCategories(recipe) {
			series := byID[category.ID]
			if series == nil {
				series = &servers.Series{ID: category.ID, Title: category.Name, SortTitle: servers.SortKey(category.Name), Direction: servers.LeftToRight}
				byID[category.ID] = series
			}
			series.Books++
			if recipe.UpdatedAt.After(series.Modified) {
				series.Modified = recipe.UpdatedAt
			}
		}
	}
	allSeries := make([]servers.Series, 0, len(byID))
	for _, series := range byID {
		allSeries = append(allSeries, *series)
	}
	slices.SortFunc(allSeries, func(first, second servers.Series) int { return servers.CompareTitles(first.Title, second.Title) })
	return allSeries, nil
}

// Libraries is unsupported: Mealie has no libraries.
func (server *Server) Libraries(ctx context.Context) ([]servers.Library, error) {
	return nil, fmt.Errorf("mealie libraries: %w", errors.ErrUnsupported)
}

// Books returns the recipes of a category, sorted by name. It lays out those it has not
// yet, to count their pages.
func (server *Server) Books(ctx context.Context, seriesID string) ([]servers.Book, error) {
	recipes, err := server.listRecipes(ctx)
	if err != nil {
		return nil, err
	}
	members, seriesTitle := recipesInCategory(recipes, seriesID)
	pageCounts, err := server.countPages(ctx, members)
	if err != nil {
		return nil, err
	}
	books := make([]servers.Book, len(members))
	for index, recipe := range members {
		books[index] = servers.Book{
			ID:          recipe.ID,
			SeriesID:    seriesID,
			SeriesTitle: seriesTitle,
			Title:       recipe.Name,
			Number:      float64(index + 1),
			Pages:       pageCounts[index],
			Modified:    recipe.UpdatedAt,
		}
	}
	return books, nil
}

// InProgress is unsupported: Mealie has no read progress.
func (server *Server) InProgress(ctx context.Context, count int) ([]servers.Book, error) {
	return nil, fmt.Errorf("mealie books in progress: %w", errors.ErrUnsupported)
}

// OpenBook fetches a recipe and its photo, to draw its pages.
func (server *Server) OpenBook(ctx context.Context, bookID string, pageCount int) (servers.Pages, error) {
	summary, err := server.summary(ctx, bookID)
	if err != nil {
		return nil, err
	}
	pages, err := server.recipePages(ctx, summary)
	if err != nil {
		return nil, err
	}
	return drawnPages{pages: pages, photo: server.photo(ctx, summary)}, nil
}

// photo returns the photo of a recipe, or nil if it has none or it cannot be fetched: the
// recipe is still readable without it.
func (server *Server) photo(ctx context.Context, summary RecipeSummary) image.Image {
	if summary.Image == "" {
		return nil
	}
	data, err := server.client.Image(ctx, summary.ID, originalImage)
	if err != nil {
		log.Printf("mealie: photo of %s: %v", summary.Slug, err)
		return nil
	}
	photo, err := recipepages.DecodePhoto(data)
	if err != nil {
		log.Printf("mealie: photo of %s: %v", summary.Slug, err)
		return nil
	}
	return photo
}

// OpenFile is unsupported: Mealie books are downloaded page by page.
func (server *Server) OpenFile(context.Context, string) (io.ReadCloser, int64, error) {
	return nil, 0, fmt.Errorf("mealie book files: %w", errors.ErrUnsupported)
}

// FirstPage returns page 1 of a recipe, as a PNG.
func (server *Server) FirstPage(ctx context.Context, bookID string) ([]byte, error) {
	pages, err := server.OpenBook(ctx, bookID, 1)
	if err != nil {
		return nil, err
	}
	return pages.Page(ctx, 1)
}

// SeriesThumbnail returns the photo of the category's first recipe that has one.
func (server *Server) SeriesThumbnail(ctx context.Context, seriesID string) ([]byte, error) {
	recipes, err := server.knownRecipes(ctx)
	if err != nil {
		return nil, err
	}
	for _, recipe := range recipes {
		inSeries := slices.ContainsFunc(recipeCategories(recipe), func(category Category) bool { return category.ID == seriesID })
		if inSeries && recipe.Image != "" {
			return server.client.Image(ctx, recipe.ID, thumbnailImage)
		}
	}
	return nil, fmt.Errorf("mealie: no photo in category %s", seriesID)
}

// BookThumbnail returns the small photo of a recipe (WebP).
func (server *Server) BookThumbnail(ctx context.Context, bookID string) ([]byte, error) {
	summary, err := server.summary(ctx, bookID)
	if err != nil {
		return nil, err
	}
	if summary.Image == "" {
		return nil, fmt.Errorf("mealie: recipe %s has no photo", summary.Slug)
	}
	return server.client.Image(ctx, summary.ID, thumbnailImage)
}

// SetProgress is unsupported: Mealie has no read progress.
func (server *Server) SetProgress(ctx context.Context, bookID string, page int, completed bool) error {
	return fmt.Errorf("mealie read progress: %w", errors.ErrUnsupported)
}

// ClearProgress is unsupported: Mealie has no read progress.
func (server *Server) ClearProgress(ctx context.Context, bookID string) error {
	return fmt.Errorf("mealie read progress: %w", errors.ErrUnsupported)
}
