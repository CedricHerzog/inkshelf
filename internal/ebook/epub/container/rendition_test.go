package container

import (
	"fmt"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/epubtest"
)

// renditionBook writes a book of three pages whose package has metas and whose itemrefs have
// properties, and opens it.
func renditionBook(t *testing.T, metas string, properties [3]string) *Book {
	t.Helper()
	files := map[string]string{
		"mimetype": "application/epub+zip",
		"META-INF/container.xml": `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`,
	}
	manifest, spine := "", ""
	for index, itemProperties := range properties {
		path := fmt.Sprintf("page%d.xhtml", index+1)
		files["OEBPS/"+path] = `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Page</title></head><body><p>Page</p></body></html>`
		manifest += fmt.Sprintf(`<item id="page%d" href="%s" media-type="application/xhtml+xml"/>`, index+1, path)
		spine += fmt.Sprintf(`<itemref idref="page%d" properties="%s"/>`, index+1, itemProperties)
	}
	files["OEBPS/content.opf"] = `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:identifier id="id">rendition-test</dc:identifier><dc:title>Rendition</dc:title><dc:language>en</dc:language>
` + metas + `
</metadata>
<manifest>` + manifest + `</manifest>
<spine>` + spine + `</spine>
</package>`
	book, err := Open(epubtest.Write(t, files))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { book.Close() })
	return book
}

func TestPackageRenditionDefaults(t *testing.T) {
	book := renditionBook(t, "", [3]string{})
	want := Rendition{Layout: Reflowable, Flow: FlowAuto, Spread: SpreadAuto, Orientation: OrientationAuto}
	if book.Rendition != want {
		t.Errorf("rendition %+v, want the defaults %+v", book.Rendition, want)
	}
}

func TestPackageRenditionMetas(t *testing.T) {
	metas := `<meta property="rendition:layout">pre-paginated</meta>
<meta property="rendition:flow">scrolled-doc</meta>
<meta property="rendition:spread">landscape</meta>
<meta property="rendition:orientation">portrait</meta>
<meta property="rendition:spread">none</meta>
<meta refines="#page1" property="rendition:layout">reflowable</meta>`
	book := renditionBook(t, metas, [3]string{})
	want := Rendition{Layout: PrePaginated, Flow: FlowScrolledDoc, Spread: SpreadLandscape, Orientation: OrientationPortrait}
	if book.Rendition != want {
		t.Errorf("rendition %+v, want %+v: the first valid value of each, refining metas ignored", book.Rendition, want)
	}
}

func TestUnknownValuesAreIgnored(t *testing.T) {
	metas := `<meta property="rendition:layout">sideways</meta>
<meta property="rendition:spread">sometimes</meta>
<meta property="rendition:spread">both</meta>`
	book := renditionBook(t, metas, [3]string{"rendition:flow-upwards page-spread-middle"})
	if book.Rendition.Layout != Reflowable || book.Rendition.Spread != SpreadBoth {
		t.Errorf("rendition %+v: an unknown value must leave the default or the next valid value", book.Rendition)
	}
	if item := book.Spine()[0].Rendition; item != (Rendition{}) {
		t.Errorf("item rendition %+v: unknown properties must set nothing", item)
	}
}

func TestPortraitSpreadReadsAsBoth(t *testing.T) {
	book := renditionBook(t, `<meta property="rendition:spread">portrait</meta>`, [3]string{"rendition:spread-portrait"})
	if book.Rendition.Spread != SpreadBoth || book.Spine()[0].Rendition.Spread != SpreadBoth {
		t.Errorf("package spread %q, item spread %q; want both for the deprecated portrait",
			book.Rendition.Spread, book.Spine()[0].Rendition.Spread)
	}
}

func TestItemRenditionOverridesPackage(t *testing.T) {
	metas := `<meta property="rendition:layout">pre-paginated</meta><meta property="rendition:spread">auto</meta>`
	book := renditionBook(t, metas, [3]string{
		"rendition:layout-reflowable rendition:spread-none rendition:orientation-landscape rendition:flow-paginated",
		"page-spread-left",
		"rendition:page-spread-right rendition:layout-pre-paginated",
	})
	spine := book.Spine()
	tests := []struct {
		item int
		want Rendition
	}{
		{0, Rendition{Layout: Reflowable, Flow: FlowPaginated, Spread: SpreadNone, Orientation: OrientationLandscape}},
		{1, Rendition{Layout: PrePaginated, Flow: FlowAuto, Spread: SpreadAuto, Orientation: OrientationAuto, PageSpread: PageSpreadLeft}},
		{2, Rendition{Layout: PrePaginated, Flow: FlowAuto, Spread: SpreadAuto, Orientation: OrientationAuto, PageSpread: PageSpreadRight}},
	}
	for _, test := range tests {
		if got := book.RenditionOf(spine[test.item]); got != test.want {
			t.Errorf("item %d: rendition %+v, want %+v", test.item, got, test.want)
		}
	}
	if book.ChapterLayout(0) != Reflowable || book.ChapterLayout(1) != PrePaginated {
		t.Errorf("ChapterLayout = %q, %q; want the item's, else the book's", book.ChapterLayout(0), book.ChapterLayout(1))
	}
}

func TestPageSpreadSpellings(t *testing.T) {
	tests := map[string]string{
		"page-spread-left":                   PageSpreadLeft,
		"rendition:page-spread-left":         PageSpreadLeft,
		"page-spread-right":                  PageSpreadRight,
		"rendition:page-spread-right":        PageSpreadRight,
		"rendition:page-spread-center":       PageSpreadCenter,
		"page-spread-center":                 PageSpreadCenter,
		"page-spread-left page-spread-right": PageSpreadLeft, // the first one given wins
	}
	for properties, want := range tests {
		if got := readItemRendition(properties).PageSpread; got != want {
			t.Errorf("readItemRendition(%q).PageSpread = %q, want %q", properties, got, want)
		}
	}
}

// FuzzItemRendition reads any properties attribute: no panic, and every value set is one the
// property may take.
func FuzzItemRendition(f *testing.F) {
	f.Add("rendition:layout-pre-paginated page-spread-left")
	f.Add("rendition:spread- rendition:-auto rendition:")
	f.Fuzz(func(t *testing.T, properties string) {
		rendition := readItemRendition(properties)
		for name, value := range map[string]string{"layout": rendition.Layout, "flow": rendition.Flow, "spread": rendition.Spread, "orientation": rendition.Orientation} {
			if value == "" {
				continue
			}
			known := false
			for _, allowed := range renditionValues[name] {
				known = known || allowed == value
			}
			if !known {
				t.Errorf("readItemRendition(%q) set %s to %q", properties, name, value)
			}
		}
	})
}
