// Package mealie reads a Mealie server, a server of cooking recipes, through its REST API, and
// makes it look like a server of books: one series per category (and one for the recipes in
// none), one book per recipe.
//
// Every request carries the user's API token as "Authorization: Bearer" (created in Mealie
// under Profile > API Tokens). The app lists every recipe at once (/api/recipes?perPage=-1),
// then fetches a whole recipe (/api/recipes/{slug}) to know what its pages show, and its photo
// from /api/media/recipes/{id}/images/.
//
// A recipe has no pages of its own: package recipepages lays its name, ingredients, steps and
// notes out into pages and draws each as a picture, so that a recipe downloads and reads like
// a comic. The number of pages is only known once a recipe is laid out, so Books lays out
// every recipe it lists and keeps the result until the recipe changes, and OpenBook later
// agrees on the count. Mealie keeps no reading progress: the reader keeps it alone.
//
// Client is the API, call by call; Server turns it into a servers.Server.
package mealie
