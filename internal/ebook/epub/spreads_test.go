package epub

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"
)

// fixedBook opens a pre-paginated book of a page per entry of properties, each the properties
// of its itemref, with metadata (rendition meta elements) and spine attributes.
func fixedBook(t *testing.T, metadata, spineAttributes string, properties ...string) *Book {
	t.Helper()
	files := map[string]string{}
	var manifest, spine strings.Builder
	for index, itemProperties := range properties {
		name := fmt.Sprintf("page%d.xhtml", index)
		files["OEBPS/"+name] = `<html xmlns="http://www.w3.org/1999/xhtml"><head><meta name="viewport" content="width=600, height=800"/></head><body><p>Page</p></body></html>`
		fmt.Fprintf(&manifest, `<item id="p%d" href="%s" media-type="application/xhtml+xml"/>`, index, name)
		fmt.Fprintf(&spine, `<itemref idref="p%d" properties="%s"/>`, index, itemProperties)
	}
	return openPackage(t, "", `<meta property="rendition:layout">pre-paginated</meta>`+metadata, manifest.String(),
		spineAttributes+">"+spine.String(), files)
}

func TestSpreads(t *testing.T) {
	for _, test := range []struct {
		name, metadata, spine string
		properties            []string
		want                  []Spread
	}{
		{name: "the first page alone, then pairs", properties: []string{"", "", "", "", ""},
			want: []Spread{{First: 1, Left: 1, Right: 2}, {First: 3, Left: 3, Right: 4}}},
		{name: "no spreads", metadata: `<meta property="rendition:spread">none</meta>`, properties: []string{"", "", ""}},
		{name: "spread none on one page", properties: []string{"", "", "rendition:spread-none", "", ""},
			want: []Spread{{First: 3, Left: 3, Right: 4}}},
		{name: "a pinned side", properties: []string{"page-spread-left", "", "page-spread-left"},
			want: []Spread{{First: 0, Left: 0, Right: 1}}},
		{name: "centred, portrait and reflowable pages stand alone",
			properties: []string{"", "", "page-spread-center", "", "", "rendition:orientation-portrait", "", "", "rendition:layout-reflowable", "", ""},
			want:       []Spread{{First: 3, Left: 3, Right: 4}, {First: 6, Left: 6, Right: 7}, {First: 9, Left: 9, Right: 10}}},
		{name: "two pages pinned to the same side", properties: []string{"", "page-spread-left", "page-spread-left", ""},
			want: []Spread{{First: 2, Left: 2, Right: 3}}},
		{name: "right to left", spine: ` page-progression-direction="rtl"`, properties: []string{"", "", "", ""},
			want: []Spread{{First: 1, Left: 2, Right: 1}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := fixedBook(t, test.metadata, test.spine, test.properties...).Spreads(); !slices.Equal(got, test.want) {
				t.Errorf("Spreads = %v, want %v", got, test.want)
			}
		})
	}
}

// A reflowable book's image spreads: two chapters that are each one image, pinned to opposite
// sides, the first on the side reading starts from.
func TestImageSpreads(t *testing.T) {
	for _, test := range []struct {
		name, spine string
		chapters    []string // "text", or "image" with its itemref's properties after a space
		want        []Spread
	}{
		{name: "a spread", chapters: []string{"text", "image page-spread-left", "image page-spread-right", "text"},
			want: []Spread{{First: 1, Left: 1, Right: 2}}},
		{name: "the wrong way round", chapters: []string{"image page-spread-right", "image page-spread-left"}},
		{name: "not pinned", chapters: []string{"image", "image"}},
		{name: "text beside the image", chapters: []string{"image page-spread-left", "text page-spread-right"}},
		{name: "right to left", spine: ` page-progression-direction="rtl"`, chapters: []string{"image page-spread-right", "image page-spread-left"},
			want: []Spread{{First: 0, Left: 1, Right: 0}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{"OEBPS/picture.png": pngOf(t, image.Pt(800, 1200))}
			var manifest, spine strings.Builder
			manifest.WriteString(`<item id="picture" href="picture.png" media-type="image/png"/>`)
			for index, chapter := range test.chapters {
				kind, properties, _ := strings.Cut(chapter, " ")
				body := "<p>Some text.</p>"
				if kind == "image" {
					body = `<div><img src="picture.png" alt=""/></div>`
				}
				files[fmt.Sprintf("OEBPS/c%d.xhtml", index)] = xhtml(body)
				fmt.Fprintf(&manifest, `<item id="c%d" href="c%d.xhtml" media-type="application/xhtml+xml"/>`, index, index)
				fmt.Fprintf(&spine, `<itemref idref="c%d" properties="%s"/>`, index, properties)
			}
			book := openPackage(t, "", "", manifest.String(), test.spine+">"+spine.String(), files)
			index, err := BuildIndex(book)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(index.Spreads, test.want) {
				t.Errorf("Spreads = %v, want %v", index.Spreads, test.want)
			}
		})
	}
}
