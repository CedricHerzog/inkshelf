package mealie

import (
	"context"
	"net/url"
	"time"
)

// RecipeSummary is a recipe as Mealie lists them.
type RecipeSummary struct {
	ID         string     `json:"id"`
	Slug       string     `json:"slug"`
	Name       string     `json:"name"`
	Image      string     `json:"image"` // "" if the recipe has no photo
	Categories []Category `json:"recipeCategory"`
	UpdatedAt  time.Time  `json:"updatedAt"` // zero if Mealie doesn't say
}

// Category is a category recipes are filed in.
type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Recipe is the whole of a recipe: what its pages show.
type Recipe struct {
	RecipeSummary
	Description    string        `json:"description"`
	PrepTime       string        `json:"prepTime"`
	PerformTime    string        `json:"performTime"` // Mealie's "Cook Time"
	TotalTime      string        `json:"totalTime"`
	RecipeYield    string        `json:"recipeYield"`
	RecipeServings float64       `json:"recipeServings"`
	Tools          []Tool        `json:"tools"`
	Ingredients    []Ingredient  `json:"recipeIngredient"`
	Instructions   []Instruction `json:"recipeInstructions"`
	Notes          []Note        `json:"notes"`
}

// Tool is a utensil a recipe needs.
type Tool struct {
	Name string `json:"name"`
}

// Ingredient is a line of a recipe's ingredients. A non-empty Title starts a section.
type Ingredient struct {
	Title   string `json:"title"`
	Display string `json:"display"` // quantity, unit, food and note, as Mealie shows them
	Note    string `json:"note"`
	Food    *Food  `json:"food"` // nil if the line was not parsed
}

// Food is the food of an ingredient.
type Food struct {
	Name       string `json:"name"`
	PluralName string `json:"pluralName"`
}

// Instruction is a step of a recipe. A non-empty Title starts a section.
type Instruction struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Text    string `json:"text"`
}

// Note is a note at the end of a recipe.
type Note struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// The photos of a recipe, by file name.
const (
	originalImage  = "original.webp"
	thumbnailImage = "min-original.webp"
)

// Recipes returns all the recipes, sorted by name.
func (client *Client) Recipes(ctx context.Context) ([]RecipeSummary, error) {
	var page resultPage[RecipeSummary]
	err := client.getJSON(ctx, "/api/recipes?perPage=-1&orderBy=name&orderDirection=asc", &page)
	return page.Items, err
}

// Recipe returns the recipe of slug.
func (client *Client) Recipe(ctx context.Context, slug string) (Recipe, error) {
	var recipe Recipe
	err := client.getJSON(ctx, "/api/recipes/"+url.PathEscape(slug), &recipe)
	return recipe, err
}

// Image returns a photo of a recipe (originalImage or thumbnailImage), as WebP.
func (client *Client) Image(ctx context.Context, recipeID, fileName string) ([]byte, error) {
	return client.getBytes(ctx, "/api/media/recipes/"+url.PathEscape(recipeID)+"/images/"+fileName)
}
