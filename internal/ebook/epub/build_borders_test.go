package epub

import (
	"image"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// Text colors are grays, legible on e-ink: a light gray darkened, a near-white one black, a link
// black unless the book colors it, light text kept on a dark box.
func TestTextColors(t *testing.T) {
	doc := styledChapter(t, `
.dim { color: #646464 } .pale { color: #999 } .white { color: #fefefe } .red { color: red }
.half { color: rgba(0, 0, 0, 0.5) } a.colored { color: #555 }
.night { background: black; color: white } .dull { background: #333 }`,
		`<p><span class="dim">dim</span> <span class="pale">pale</span> <span class="white">white</span>
<span class="red">red</span> <span class="half">half</span> <font color="#606060">font</font>
<span class="dim"><a href="#x">link</a> <a class="colored" href="#x">colored</a></span></p>
<div class="night"><p>night</p></div><div class="dull"><p>dull</p></div>`)
	for text, want := range map[string]uint8{
		"dim": 0x64, "pale": legibleGray, "white": 0, "red": 0x4C, "half": legibleGray, "font": 0x60,
		"link": 0, "colored": 0x55, "night": 0xFF, "dull": 0,
	} {
		if gray := runOf(t, doc, text).Gray; gray != want {
			t.Errorf("%s: gray %#x, want %#x", text, gray, want)
		}
	}
	if len(doc.Boxes) != 2 || doc.Boxes[0].Background != 0 || doc.Boxes[1].Background != lightBackground {
		t.Errorf("boxes %+v, want a black one and one lightened under dark text", doc.Boxes)
	}
}

// textGray keeps text and its background apart, whatever they are.
func TestTextGrayContrast(t *testing.T) {
	for background := range 256 {
		for text := range 256 {
			gray := textGray(css.Color{Gray: uint8(text), Alpha: 255}, uint8(background))
			if distance := int(gray) - background; distance > -minContrast && distance < minContrast {
				t.Fatalf("text %#x on %#x: %#x", text, background, gray)
			}
		}
	}
}

// A block element with a border or a background is a box around its blocks: its margin outside
// its border, its padding inside; the text in it is inset by both. The page's background, a
// white one and a box that holds nothing are no boxes.
func TestBoxes(t *testing.T) {
	doc := styledChapter(t, `
body { background: #fff; color: #000 } .page { background-color: white }
.box { border: 2px solid #333; background: #eee; margin: 1em 2em; padding: 0.5em 1em }
.inner { border-left: thick double; padding-left: 1em } .rule { border-top: 1px solid } p { margin: 0 }`,
		`<div class="page"><p>before</p><div class="box"><p>one</p><div class="inner"><p>two</p></div></div>
<div class="rule"></div><p>after</p></div>`)
	if len(doc.Boxes) != 2 {
		t.Fatalf("boxes %+v, want 2", doc.Boxes)
	}
	outer, inner := doc.Boxes[0], doc.Boxes[1]
	blocks := doc.Blocks
	border := document.Border{Style: document.Solid, Width: 13, Gray: 0x33}
	wantOuter := document.BorderBox{
		Start: blocks[1].Start, End: blocks[3].Start, Top: border, Right: border, Bottom: border, Left: border,
		PaddingTop: 50, PaddingBottom: 50, MarginTop: 100, MarginBottom: 100, LeftEdge: 200, RightEdge: 200,
		Background: 0xEE, Filled: true,
	}
	if outer != wantOuter {
		t.Errorf("box\n%+v, want\n%+v", outer, wantOuter)
	}
	if inner.Start != blocks[2].Start || inner.End != blocks[3].Start || inner.Left.Style != document.Double || inner.Left.Width != 31 || inner.LeftEdge != 313 {
		t.Errorf("inner box %+v", inner)
	}
	if one, two := blocks[1], blocks[2]; one.Left != 313 || one.Right != 313 || two.Left != 313+31+100 || one.MarginTop != 0 {
		t.Errorf("text in boxes from %d and %d, margin %d", one.Left, two.Left, one.MarginTop)
	}
	if after := blocks[3]; !after.BorderTop {
		t.Errorf("an empty box's border: %+v, want a rule as before", after)
	}
}

// Boxes in boxes stop maxBoxDepth deep, their text still inset.
func TestBoxDepth(t *testing.T) {
	body := "text"
	for range maxBoxDepth + 4 {
		body = `<div class="box">` + body + `</div>`
	}
	doc := styledChapter(t, `.box { border: 1px solid; padding-left: 0.5em }`, body)
	if len(doc.Boxes) != maxBoxDepth {
		t.Errorf("%d boxes, want %d", len(doc.Boxes), maxBoxDepth)
	}
	if left := doc.Blocks[0].Left; left != (maxBoxDepth+4)*50+maxBoxDepth*6 {
		t.Errorf("text at %d", left)
	}
}

// Parsing and laying out any CSS of boxes and colors neither fails nor leaves the page.
func FuzzBoxes(f *testing.F) {
	f.Add(`.box { border: 2px dashed red; background: #333; color: #ccc; padding: 1em; margin: -3em 50% }`, 3)
	f.Add(`.box { border-width: 1000em; border-style: double; background: rgba(0,0,0,.2) } .inner { color: hsl(0 0% 90%) }`, 8)
	f.Add(`div { border-left: thick dotted; page-break-before: always; padding: 30em 0 }`, 1)
	f.Add(`.box { border: dashed; margin: 10% }`, 12) // margins wider than the page
	settings := layout.Settings{Width: 600, Height: 800, Margins: image.Rect(30, 20, 30, 20), FontSize: 24, LineSpacing: 130, Justify: true}
	faces := layout.NewFaces(fonts.Serif)
	f.Cleanup(faces.Close)
	f.Fuzz(func(t *testing.T, stylesheet string, depth int) {
		body := `<p class="inner">Some text that runs on, and on, and on across the lines of a page.</p>`
		for index := range min(max(depth, 0), 12) {
			if index%2 == 0 {
				body += `<p>After it, more text that runs on, and on.</p>`
			}
			body = `<div class="box">` + body + `</div>`
		}
		doc, err := openStyled(t, stylesheet, body, nil).Chapter(0)
		if err != nil {
			return
		}
		pages, err := layout.Paginate(doc, settings, faces, func(string) (image.Point, error) { return image.Pt(10, 10), nil })
		if err != nil {
			t.Fatal(err)
		}
		page := image.Rect(0, 0, settings.Width, settings.Height)
		for _, laidOut := range pages {
			for _, box := range laidOut.Boxes {
				if !box.Rect.In(page) {
					t.Errorf("box %v out of the page", box.Rect)
				}
			}
		}
	})
}
