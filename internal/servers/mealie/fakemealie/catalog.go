package fakemealie

import (
	"fmt"
	"strings"
	"time"
)

// recipe is a recipe of the fake Mealie, as its API sends it.
type recipe struct {
	ID                 string        `json:"id"`
	Slug               string        `json:"slug"`
	Name               string        `json:"name"`
	Image              *string       `json:"image"` // null without a photo, as Mealie sends it
	RecipeCategory     []category    `json:"recipeCategory"`
	UpdatedAt          time.Time     `json:"updatedAt"`
	Description        string        `json:"description"`
	PrepTime           *string       `json:"prepTime"`
	PerformTime        *string       `json:"performTime"`
	TotalTime          *string       `json:"totalTime"`
	RecipeYield        string        `json:"recipeYield"`
	RecipeServings     float64       `json:"recipeServings"`
	Tools              []tool        `json:"tools"`
	RecipeIngredient   []ingredient  `json:"recipeIngredient"`
	RecipeInstructions []instruction `json:"recipeInstructions"`
	Notes              []note        `json:"notes"`
}

type category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type tool struct {
	Name string `json:"name"`
}

type ingredient struct {
	Title   string `json:"title"`
	Display string `json:"display"`
	Note    string `json:"note"`
	Food    *food  `json:"food"`
}

type food struct {
	Name       string `json:"name"`
	PluralName string `json:"pluralName"`
}

// parsed returns an ingredient as Mealie parses it: display repeats the quantity, unit and
// food it found before the note.
func parsed(quantity, foodName, note string) ingredient {
	return ingredient{Display: quantity + " " + foodName + " " + note, Note: note, Food: &food{Name: foodName, PluralName: foodName + "s"}}
}

type instruction struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Text    string `json:"text"`
}

type note struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// The categories of the fake Mealie.
var (
	pastry  = category{ID: "c-patisserie", Name: "Pâtisserie", Slug: "patisserie"}
	dessert = category{ID: "c-desserts", Name: "Desserts", Slug: "desserts"}
	mains   = category{ID: "c-plats", Name: "Plats", Slug: "plats"}
	starter = category{ID: "c-entrees", Name: "Entrées", Slug: "entrees"}
)

// updated is when every recipe of the fake Mealie last changed.
var updated = time.Date(2025, 11, 29, 11, 54, 13, 0, time.UTC)

// ingredients returns an ingredient line for each of displays.
func ingredients(displays ...string) []ingredient {
	lines := make([]ingredient, len(displays))
	for index, display := range displays {
		lines[index] = ingredient{Display: display}
	}
	return lines
}

// steps returns a step for each of texts.
func steps(texts ...string) []instruction {
	all := make([]instruction, len(texts))
	for index, stepText := range texts {
		all[index] = instruction{Text: stepText}
	}
	return all
}

// longSteps returns count long steps, enough to fill several pages.
func longSteps(count int) []instruction {
	all := make([]instruction, count)
	for index := range all {
		all[index] = instruction{Text: fmt.Sprintf("Étape %d : faire revenir la viande à feu vif dans une cocotte en fonte, "+
			"par petites quantités pour qu'elle colore sans bouillir. %s", index+1,
			strings.Repeat("Remuer de temps en temps, goûter et rectifier l'assaisonnement. ", 1+index%3))}
	}
	all[0].Title = "La veille"
	all[count/2].Title = "Le jour même"
	return all
}

// catalog is the fake Mealie's recipes, by name.
var catalog = []recipe{
	{
		ID: "r-boeuf", Slug: "boeuf-bourguignon", Name: "Bœuf bourguignon", Image: new("b0uf"),
		RecipeCategory: []category{mains},
		Description:    "Le classique bourguignon, mijoté longtemps : meilleur réchauffé le lendemain.",
		PrepTime:       new("45min"), PerformTime: new("3h"), TotalTime: new("3h45"), RecipeYield: "6 personnes",
		Tools: []tool{{Name: "Cocotte en fonte"}, {Name: "Planche à découper"}},
		RecipeIngredient: append(append([]ingredient{{Title: "La marinade"}},
			ingredients("1 bouteille de vin rouge de Bourgogne", "2 carottes en rondelles", "1 oignon piqué de clous de girofle", "1 bouquet garni")...),
			append([]ingredient{{Title: "La cuisson"}},
				ingredients("1,5 kg de bœuf à braiser (paleron, joue, macreuse) coupé en gros cubes", "200 g de lardons fumés",
					"250 g de champignons de Paris", "2 c. à soupe de farine", "Sel, poivre")...)...),
		RecipeInstructions: longSteps(12),
		Notes:              []note{{Title: "Conservation", Text: "Se garde trois jours au réfrigérateur ; se congèle très bien."}},
	},
	{
		ID: "r-crepes", Slug: "crepes", Name: "Crêpes", Image: new("cr3p"),
		RecipeCategory: []category{dessert},
		PrepTime:       new("10min"), PerformTime: new("30min"), RecipeServings: 4,
		RecipeIngredient: ingredients("250 g de farine", "4 œufs", "50 cl de lait", "1 pincée de sel", "50 g de beurre fondu"),
		RecipeInstructions: steps("Mélanger la farine et le sel, creuser un puits et y casser les œufs.",
			"Verser le lait petit à petit en fouettant pour éviter les grumeaux, puis le beurre fondu.",
			"Laisser reposer la pâte 1 h.", "Cuire les crêpes dans une poêle chaude et beurrée, 1 min de chaque côté."),
		Notes: []note{{Text: "Une cuillère de rhum ou de fleur d'oranger dans la pâte, au goût."}},
	},
	{
		ID: "r-gateau", Slug: "gateau-a-la-banane", Name: "Gâteau à la banane", Image: new("g4t0"),
		RecipeCategory: []category{pastry},
		PrepTime:       new("15min"), PerformTime: new("45min"), TotalTime: new("1h"),
		Tools: []tool{{Name: "Moule rond"}},
		RecipeIngredient: []ingredient{
			parsed("100 grammes", "beurre", "100g de beurre pommade"), parsed("200 grammes", "sucre", "200g de sucre"),
			parsed("2", "œuf", "2 œufs"), parsed("200 grammes", "banane", "3 bananes écrasées (~200-250g)"),
			parsed("200 grammes", "farine", "200g de farine"), parsed("1 paquet", "levure", "1 paquet de levure"),
			{Display: "sel", Note: "sel"}, {Display: "vanille", Note: "vanille"},
			parsed("100 grammes", "poudre de noisettes", "100g de noisettes en poudre"),
		},
		RecipeInstructions: steps("Mélanger les ingrédients à la main jusqu'à obtenir quelque chose d'homogène.\n",
			"Cuire à 180°C pendant 40-45 minutes.\nPlanter un couteau, il doit ressortir sec ou très légèrement humide.", ""),
		Notes: []note{{Title: "Origine recette", Text: "La recette a été donnée à Maman par Nathalie."}, {}},
	},
	{
		ID: "r-pizza", Slug: "pate-a-pizza", Name: "Pâte à pizza", Image: new("p1zz"),
		Description: "https://www.youtube.com/watch?v=5DSebxu4RLY", PrepTime: new("24h"),
		RecipeIngredient: []ingredient{
			parsed("100 millilitres", "eau", "par pizza"), parsed("1 gramme", "levure", "par litre d'eau"),
			parsed("30 grammes", "sel", "par litre d'eau"), parsed("100 millilitres", "farine", "par pizza"), {},
		},
		RecipeInstructions: append(steps("Mélanger eau et levure", "Ajouter sel", "Mettre autant de farine que d'eau"),
			append([]instruction{{Summary: "Ajouter la farine au fur et à mesure"}},
				steps("Rabattre la pâte et attendre 30min", "Faire des plis et laisser reposer 24h sous un torchon\n")...)...),
	},
	{
		ID: "r-soupe", Slug: "soupe-a-l-oignon", Name: "Soupe à l'oignon",
		RecipeCategory: []category{starter},
		PrepTime:       new("20min"), PerformTime: new("40min"), RecipeYield: "4 bols",
		RecipeIngredient: ingredients("6 oignons", "40 g de beurre", "1 l de bouillon de bœuf", "4 tranches de pain", "100 g de comté râpé"),
		RecipeInstructions: steps("Émincer les oignons et les faire fondre au beurre 20 min, jusqu'à ce qu'ils soient bien dorés.",
			"Mouiller avec le bouillon et laisser mijoter 20 min.",
			"Verser dans des bols, poser le pain, couvrir de fromage et gratiner sous le gril."),
	},
	{
		ID: "r-tarte", Slug: "tarte-aux-pommes", Name: "Tarte aux pommes", Image: new("t4rt"),
		RecipeCategory: []category{pastry, dessert},
		PrepTime:       new("20min"), PerformTime: new("35min"),
		Tools:            []tool{{Name: "Moule à tarte"}},
		RecipeIngredient: ingredients("1 pâte brisée", "5 pommes", "50 g de sucre", "30 g de beurre", "1 sachet de sucre vanillé"),
		RecipeInstructions: steps("Foncer le moule avec la pâte et la piquer à la fourchette.",
			"Éplucher les pommes, les couper en fines lamelles et les disposer en rosace.",
			"Saupoudrer de sucre, parsemer de beurre et cuire 35 min à 200°C."),
	},
}

func init() {
	for index := range catalog {
		catalog[index].UpdatedAt = updated
	}
}
