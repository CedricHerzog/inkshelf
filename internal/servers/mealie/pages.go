package mealie

import (
	"context"
	"fmt"
	"image"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/servers/mealie/recipepages"
)

// layout lays out recipe into pages. With photo, page 1 starts with the photo's box.
func layout(recipe Recipe, photo bool) []recipepages.Page {
	return recipepages.Layout(pageRecipe(recipe), photo)
}

// pageRecipe returns what the pages of recipe show: Mealie's fields chosen and joined into
// lines.
func pageRecipe(recipe Recipe) recipepages.Recipe {
	pages := recipepages.Recipe{
		Name:        recipe.Name,
		Times:       recipeTimes(recipe),
		Tools:       toolNames(recipe.Tools),
		Description: recipe.Description,
	}
	for _, ingredient := range recipe.Ingredients {
		pages.Ingredients = append(pages.Ingredients, recipepages.Entry{Title: ingredient.Title, Text: ingredientText(ingredient)})
	}
	for _, step := range recipe.Instructions {
		pages.Steps = append(pages.Steps, recipepages.Entry{Title: step.Title, Text: stepText(step)})
	}
	for _, note := range recipe.Notes {
		pages.Notes = append(pages.Notes, recipepages.Entry{Title: note.Title, Text: note.Text})
	}
	return pages
}

// recipeTimes returns the times and the yield of recipe, as one line, or "".
func recipeTimes(recipe Recipe) string {
	var parts []string
	for _, part := range []struct{ label, value string }{
		{"Prep", recipe.PrepTime},
		{"Cook", recipe.PerformTime},
		{"Total", recipe.TotalTime},
		{"Yield", recipe.RecipeYield},
	} {
		if value := strings.TrimSpace(part.value); value != "" {
			parts = append(parts, part.label+" "+value)
		}
	}
	if strings.TrimSpace(recipe.RecipeYield) == "" && recipe.RecipeServings > 0 {
		parts = append(parts, fmt.Sprintf("Servings %g", recipe.RecipeServings))
	}
	return strings.Join(parts, "  •  ")
}

// toolNames returns the names of tools, separated by commas.
func toolNames(tools []Tool) string {
	var names []string
	for _, tool := range tools {
		if name := strings.TrimSpace(tool.Name); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

// ingredientText returns the line of an ingredient. Mealie's display adds the note to the
// quantity, unit and food it parsed, which repeats them when the note is the whole line
// already ("100 grammes beurre 100g de beurre pommade"): the note alone is shown then, when it
// starts with a quantity or names the food.
func ingredientText(ingredient Ingredient) string {
	note := strings.TrimSpace(ingredient.Note)
	if note != "" && (startsWithDigit(note) || namesFood(note, ingredient.Food)) {
		return note
	}
	if display := strings.TrimSpace(ingredient.Display); display != "" {
		return display
	}
	return note
}

// startsWithDigit reports whether text starts with a digit.
func startsWithDigit(text string) bool {
	return text[0] >= '0' && text[0] <= '9'
}

// namesFood reports whether text names food, ignoring case.
func namesFood(text string, food *Food) bool {
	if food == nil {
		return false
	}
	lower := strings.ToLower(text)
	for _, name := range []string{food.Name, food.PluralName} {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" && strings.Contains(lower, name) {
			return true
		}
	}
	return false
}

// stepText returns the text of a step, or its summary if it has no text.
func stepText(step Instruction) string {
	if text := strings.TrimSpace(step.Text); text != "" {
		return text
	}
	return strings.TrimSpace(step.Summary)
}

// drawnPages are the pages of a recipe being downloaded: drawn on demand, as PNGs.
type drawnPages struct {
	pages []recipepages.Page
	photo image.Image // nil if the recipe has none or it could not be fetched
}

// Page draws page number. A page past the layout (the recipe got shorter since it was
// listed) is blank.
func (pages drawnPages) Page(ctx context.Context, number int) ([]byte, error) {
	if number < 1 || number > len(pages.pages) {
		return recipepages.Render(recipepages.Page{}, nil)
	}
	return recipepages.Render(pages.pages[number-1], pages.photo)
}
