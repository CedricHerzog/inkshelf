package epub

import (
	"fmt"
	"image"
	"reflect"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
)

// openRoll opens a book of layout roll whose chapters are bodies, in order.
func openRoll(t *testing.T, files map[string]string, bodies ...string) *Book {
	t.Helper()
	var manifest, spine strings.Builder
	for index, body := range bodies {
		name := fmt.Sprintf("strip%d.xhtml", index)
		files["OEBPS/"+name] = xhtml(body)
		fmt.Fprintf(&manifest, `<item id="strip%d" href="%s" media-type="application/xhtml+xml"/>`, index, name)
		fmt.Fprintf(&spine, `<itemref idref="strip%d"/>`, index)
	}
	return openPackage(t, "", `<dc:title>Webtoon</dc:title><meta property="rendition:layout">roll</meta>`, manifest.String(), ">"+spine.String(), files)
}

// A roll of three images of different shapes, on a screen 200 wide and 300 high: each image as
// wide as the screen (200x200, 200x700, 200x240), one under the other, 1140 high in all, cut
// into 4 pages. Every row of the strip is drawn, and the last page is white past its end.
func TestRollPages(t *testing.T) {
	files := map[string]string{
		"OEBPS/a.png": pngOf(t, image.Pt(100, 100)),
		"OEBPS/b.png": pngOf(t, image.Pt(100, 350)),
		"OEBPS/c.png": pngOf(t, image.Pt(50, 60)),
	}
	book := openRoll(t, files, `<img src="a.png"/>`, `<p><img src="b.png" alt="b"/></p> `, `<div><img src="c.png"/></div>`)
	if !book.Roll() || !book.DrawnAsPages() || book.FixedLayout() {
		t.Fatalf("roll %v, drawn as pages %v, fixed layout %v: want a roll", book.Roll(), book.DrawnAsPages(), book.FixedLayout())
	}
	screen := image.Pt(200, 300)
	if pages := book.PageCount(screen); pages != 4 {
		t.Fatalf("%d pages, want ceil(1140/300) = 4", pages)
	}
	for index := range 4 {
		page, err := book.RenderPage(index, screen, layout.Elements{})
		if err != nil {
			t.Fatal(err)
		}
		for row := range screen.Y {
			wantDark := index*screen.Y+row < 1140
			for _, column := range []int{0, 100, 199} {
				if dark := page.GrayAt(column, row).Y < 128; dark != wantDark {
					t.Fatalf("page %d, row %d, column %d: dark %v, want %v", index, row, column, dark, wantDark)
				}
			}
		}
	}
	if _, err := book.RenderPage(4, screen, layout.Elements{}); err == nil {
		t.Error("a fifth page was drawn")
	}
}

// A roll whose chapters hold text is read as a reflowable book.
func TestRollWithText(t *testing.T) {
	book := openRoll(t, map[string]string{"OEBPS/a.png": pngOf(t, image.Pt(10, 10))}, `<img src="a.png"/>`, `<p>Chapter two</p>`)
	if book.Roll() || book.DrawnAsPages() || book.PageCount(image.Pt(200, 300)) != 2 {
		t.Errorf("a roll with text: roll %v, drawn as pages %v", book.Roll(), book.DrawnAsPages())
	}
}

// A scrolled flow is shown paged: its chapters are those of the same book paginated.
func TestScrolledFlowPaginates(t *testing.T) {
	documents := map[string][]any{}
	for _, flow := range []string{"paginated", "scrolled-doc", "scrolled-continuous"} {
		book := openPackage(t, "", `<dc:title>Flow</dc:title><meta property="rendition:flow">`+flow+`</meta>`,
			`<item id="one" href="one.xhtml" media-type="application/xhtml+xml"/>`, `><itemref idref="one"/>`,
			map[string]string{"OEBPS/one.xhtml": xhtml(`<h1>Loomings</h1><p>Call me Ishmael.</p>`)})
		if book.Rendition.Flow != flow || book.DrawnAsPages() {
			t.Errorf("flow %q read as %q, drawn as pages %v", flow, book.Rendition.Flow, book.DrawnAsPages())
		}
		chapter, err := book.Chapter(0)
		if err != nil {
			t.Fatal(err)
		}
		documents[flow] = []any{chapter.Blocks, chapter.Chars}
	}
	for _, flow := range []string{"scrolled-doc", "scrolled-continuous"} {
		if !reflect.DeepEqual(documents[flow], documents["paginated"]) {
			t.Errorf("%s chapter differs from the paginated one", flow)
		}
	}
}
