package recipepages

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"testing"
)

// benchmarkRecipe is a recipe as Mealie keeps them: a dozen ingredients in two sections, steps
// long enough to wrap, and notes. It lays out on two pages with its photo.
var benchmarkRecipe = Recipe{
	Name:        "Tarte aux pommes à la crème d'amande et à la cannelle",
	Times:       "Préparation 30 min · Cuisson 45 min · 8 parts",
	Tools:       "moule à tarte, rouleau à pâtisserie, fouet, saladier",
	Description: "La tarte du dimanche : une pâte sablée, une crème d'amande parfumée à la cannelle et des pommes en lamelles, dorées au four.",
	Ingredients: []Entry{
		{Title: "Pâte sablée", Text: "250 g de farine"},
		{Text: "125 g de beurre mou"},
		{Text: "80 g de sucre glace"},
		{Text: "1 œuf"},
		{Text: "1 pincée de sel"},
		{Title: "Garniture", Text: "6 pommes Reinette"},
		{Text: "100 g de poudre d'amande"},
		{Text: "80 g de sucre"},
		{Text: "2 œufs"},
		{Text: "60 g de beurre fondu"},
		{Text: "1 cuillère à café de cannelle"},
		{Text: "2 cuillères à soupe de confiture d'abricot"},
	},
	Steps: []Entry{
		{Text: "Mélanger la farine, le sucre glace et le sel, puis ajouter le beurre mou en morceaux et sabler du bout des doigts."},
		{Text: "Ajouter l'œuf, former une boule sans trop travailler la pâte, l'envelopper et la laisser reposer 30 minutes au frais."},
		{Text: "Préchauffer le four à 180 °C. Étaler la pâte sur 3 mm et en garnir le moule beurré, piquer le fond à la fourchette."},
		{Text: "Fouetter les œufs avec le sucre jusqu'à ce que le mélange blanchisse, puis ajouter la poudre d'amande, le beurre fondu et la cannelle."},
		{Text: "Éplucher les pommes, les couper en quatre, retirer le cœur et les détailler en fines lamelles."},
		{Title: "Montage", Text: "Verser la crème d'amande sur la pâte et disposer les lamelles de pommes en rosace en les faisant se chevaucher."},
		{Text: "Enfourner pour 45 minutes, jusqu'à ce que les pommes soient dorées et la crème prise."},
		{Text: "À la sortie du four, napper de confiture d'abricot tiédie pour faire briller la tarte. Servir tiède ou froide."},
	},
	Notes: []Entry{
		{Title: "Variante", Text: "Remplacer la moitié des pommes par des poires, et la cannelle par de la vanille."},
		{Text: "La pâte se garde deux jours au réfrigérateur, ou un mois au congélateur."},
	},
}

// benchmarkPhoto returns a recipe's photo, about the size of Mealie's original images: a
// gradient with noise, which a JPEG does not encode to nothing.
func benchmarkPhoto() *image.RGBA {
	const width, height = 1200, 800
	random := rand.New(rand.NewPCG(1, 2))
	photo := image.NewRGBA(image.Rect(0, 0, width, height))
	for row := range height {
		for column := range width {
			noise := uint8(random.IntN(40))
			photo.Set(column, row, color.RGBA{uint8(column*200/width) + noise, uint8(row*200/height) + noise, 120 + noise, 255})
		}
	}
	return photo
}

// BenchmarkLayout lays the recipe out into pages, with its photo.
func BenchmarkLayout(b *testing.B) {
	if pages := Layout(benchmarkRecipe, true); len(pages) < 2 {
		b.Fatalf("%d page, want the recipe on several", len(pages))
	}
	b.ReportAllocs()
	for b.Loop() {
		Layout(benchmarkRecipe, true)
	}
}

// BenchmarkRender draws the first page, the photo scaled into its box, and a page of text
// only, each encoded as a PNG.
func BenchmarkRender(b *testing.B) {
	pages := Layout(benchmarkRecipe, true)
	photo := benchmarkPhoto()
	for _, page := range []struct {
		name   string
		layout Page
	}{
		{name: "with photo", layout: pages[0]},
		{name: "text", layout: pages[1]},
	} {
		b.Run(page.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Render(page.layout, photo); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkDecodePhoto decodes the photo as a JPEG and as a PNG. Mealie also serves WebP,
// which the standard library cannot encode to make one here.
func BenchmarkDecodePhoto(b *testing.B) {
	photo := benchmarkPhoto()
	var jpegData, pngData bytes.Buffer
	if err := jpeg.Encode(&jpegData, photo, &jpeg.Options{Quality: 85}); err != nil {
		b.Fatal(err)
	}
	if err := png.Encode(&pngData, photo); err != nil {
		b.Fatal(err)
	}
	for _, format := range []struct {
		name string
		data []byte
	}{
		{name: "JPEG", data: jpegData.Bytes()},
		{name: "PNG", data: pngData.Bytes()},
	} {
		b.Run(format.name, func(b *testing.B) {
			b.SetBytes(int64(len(format.data)))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := DecodePhoto(format.data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
