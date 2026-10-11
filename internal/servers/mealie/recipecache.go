package mealie

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/mealie/recipepages"
)

// Mealie lists recipes without what their pages show, and fetching a whole recipe to lay it out
// takes a request each. Server keeps the last list, so that the calls given only a recipe's id
// need no new one, and each recipe's layout until the recipe changes.

// detailWorkers is how many recipes Books fetches at once to count their pages.
const detailWorkers = 3

// laidOutRecipe is a recipe's pages, as laid out from summary's version of it.
type laidOutRecipe struct {
	summary RecipeSummary
	pages   []recipepages.Page
}

// listRecipes fetches all the recipes, and keeps them for the calls given only an id.
func (server *Server) listRecipes(ctx context.Context) ([]RecipeSummary, error) {
	recipes, err := server.client.Recipes(ctx)
	if err != nil {
		return nil, err
	}
	sortRecipes(recipes) // Mealie's order depends on its database's collation
	server.mu.Lock()
	defer server.mu.Unlock()
	server.recipes = make(map[string]RecipeSummary, len(recipes))
	for _, recipe := range recipes {
		server.recipes[recipe.ID] = recipe
	}
	return recipes, nil
}

// knownRecipes returns the recipes of the last list sorted by name, or fetches them if there
// is none yet.
func (server *Server) knownRecipes(ctx context.Context) ([]RecipeSummary, error) {
	server.mu.Lock()
	recipes := make([]RecipeSummary, 0, len(server.recipes))
	for _, recipe := range server.recipes {
		recipes = append(recipes, recipe)
	}
	server.mu.Unlock()
	if len(recipes) == 0 {
		return server.listRecipes(ctx)
	}
	sortRecipes(recipes)
	return recipes, nil
}

// summary returns the recipe of id from the last list, fetching the list if it lacks it.
func (server *Server) summary(ctx context.Context, id string) (RecipeSummary, error) {
	server.mu.Lock()
	recipe, ok := server.recipes[id]
	server.mu.Unlock()
	if ok {
		return recipe, nil
	}
	recipes, err := server.listRecipes(ctx)
	if err != nil {
		return RecipeSummary{}, err
	}
	if index := slices.IndexFunc(recipes, func(recipe RecipeSummary) bool { return recipe.ID == id }); index >= 0 {
		return recipes[index], nil
	}
	return RecipeSummary{}, fmt.Errorf("mealie: no recipe %s", id)
}

// recipePages returns the pages of a recipe, laying it out if it has not been since it last
// changed.
func (server *Server) recipePages(ctx context.Context, summary RecipeSummary) ([]recipepages.Page, error) {
	server.mu.Lock()
	cached, ok := server.laidOut[summary.ID]
	server.mu.Unlock()
	if ok && cached.summary.UpdatedAt.Equal(summary.UpdatedAt) && cached.summary.Image == summary.Image {
		return cached.pages, nil
	}
	recipe, err := server.client.Recipe(ctx, summary.Slug)
	if err != nil {
		return nil, err
	}
	// The summary decides whether there is a photo: OpenBook fetches it by the summary.
	pages := layout(recipe, summary.Image != "")
	server.mu.Lock()
	server.laidOut[summary.ID] = laidOutRecipe{summary: summary, pages: pages}
	server.mu.Unlock()
	return pages, nil
}

// countPages returns the number of pages of each recipe, laying out detailWorkers at once.
func (server *Server) countPages(ctx context.Context, recipes []RecipeSummary) ([]int, error) {
	counts := make([]int, len(recipes))
	indexes := make(chan int, len(recipes))
	for index := range recipes {
		indexes <- index
	}
	close(indexes)
	var workers sync.WaitGroup
	var mu sync.Mutex // guards firstErr
	var firstErr error
	for range min(detailWorkers, len(recipes)) {
		workers.Go(func() {
			for index := range indexes {
				pages, err := server.recipePages(ctx, recipes[index])
				if err != nil {
					mu.Lock()
					firstErr = cmp.Or(firstErr, err)
					mu.Unlock()
					continue
				}
				counts[index] = len(pages)
			}
		})
	}
	workers.Wait()
	return counts, firstErr
}

// sortRecipes sorts recipes by name, ignoring case and accents.
func sortRecipes(recipes []RecipeSummary) {
	slices.SortFunc(recipes, func(first, second RecipeSummary) int { return servers.CompareTitles(first.Name, second.Name) })
}

// recipeCategories returns the categories of recipe, or the uncategorized one if it has none.
func recipeCategories(recipe RecipeSummary) []Category {
	if len(recipe.Categories) == 0 {
		return []Category{{ID: uncategorizedID, Name: uncategorizedTitle}}
	}
	return recipe.Categories
}

// recipesInCategory returns the recipes filed in category seriesID, and the category's name.
func recipesInCategory(recipes []RecipeSummary, seriesID string) ([]RecipeSummary, string) {
	var members []RecipeSummary
	title := ""
	for _, recipe := range recipes {
		categories := recipeCategories(recipe)
		index := slices.IndexFunc(categories, func(category Category) bool { return category.ID == seriesID })
		if index >= 0 {
			members = append(members, recipe)
			title = categories[index].Name
		}
	}
	return members, title
}
