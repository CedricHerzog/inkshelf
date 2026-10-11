package epub

import (
	"image"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// Floated images, blocks and drop caps are float boxes of their blocks, in the order of the
// text; clear goes on the next block.
func TestFloats(t *testing.T) {
	book := openStyled(t, `
.side { float: right; width: 30% }
.cap { float: left; font-size: 3em }
.shot { float: left; width: 150px }
.past { clear: both }
p { text-indent: 1em; margin: 1em 0 }
blockquote { margin-left: 2em }`,
		`<blockquote><div class="side"><p>Note one</p><p>Note two</p></div></blockquote>
<p><span class="cap">L</span>orem ipsum</p>
<p>Before <img class="shot" src="big.png"/> after</p>
<p>Text <img align="right" src="icon.png"/>.</p>
<p class="past">Below</p>
<p>Line<br clear="all"/>next</p>
<table style="float: left"><tr><td>Cell</td></tr></table>`,
		map[string]string{"OEBPS/big.png": pngOf(t, image.Pt(1200, 900)), "OEBPS/icon.png": pngOf(t, image.Pt(16, 16))})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Note one", "Note two", "L", "orem ipsum", "Before", "[image OEBPS/big.png]", "after",
		"Text", "[image OEBPS/icon.png]", ".", "Below", "Line", "next", "Cell",
	}
	if got := texts(doc); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("blocks\n%q\nwant\n%q", got, want)
	}
	blocks := doc.Blocks
	for _, index := range []int{0, 1} {
		if block := blocks[index]; block.Float != document.FloatRight || block.FloatGroup != 1 || block.FloatWidth != 30 || block.Left != 200 || block.MarginTop != 100 && index == 0 {
			t.Errorf("block %d of the sidebar: %+v, want a box 30%% wide at the blockquote's margin", index, block)
		}
	}
	if capBlock, rest := blocks[2], blocks[3]; capBlock.Float != document.FloatLeft || capBlock.FloatGroup != 2 || capBlock.FontScale != 300 ||
		capBlock.MarginTop != 0 || rest.Float != document.NoFloat || rest.MarginTop != 100 || rest.Indent != 100 {
		t.Errorf("drop cap %+v, then %+v: want the paragraph's margin and indent after it", capBlock, rest)
	}
	if shot, after := blocks[5], blocks[6]; shot.Float != document.FloatLeft || shot.Kind != document.Image || shot.FloatWidth != 25 || after.Indent != 0 || after.MarginTop != 0 {
		t.Errorf("floated image %+v, then %+v: want 25%% wide, the paragraph continued", shot, after)
	}
	if icon := blocks[8]; icon.Float != document.FloatRight || icon.Kind != document.Image {
		t.Errorf("an image aligned right floats: %+v", icon)
	}
	if blocks[10].Clear != document.ClearBoth || blocks[12].Clear != document.ClearBoth || blocks[11].Clear != 0 {
		t.Errorf("clears: %d, %d, %d; want both, none, both", blocks[10].Clear, blocks[11].Clear, blocks[12].Clear)
	}
	if blocks[13].Float != document.NoFloat {
		t.Errorf("a floated table stays in the text: %+v", blocks[13])
	}
	// The offsets go on in the order of the text.
	for index := 1; index < len(blocks); index++ {
		if blocks[index].Start <= blocks[index-1].Start {
			t.Errorf("block %d starts at %d, after %d", index, blocks[index].Start, blocks[index-1].Start)
		}
	}
}

func TestFloatSide(t *testing.T) {
	for _, test := range []struct {
		value     string
		direction document.Direction
		float     document.Float
		clear     document.Clear
	}{
		{"left", document.LeftToRight, document.FloatLeft, document.ClearLeft},
		{"right", document.RightToLeft, document.FloatRight, document.ClearRight},
		{"inline-start", document.LeftToRight, document.FloatLeft, document.ClearLeft},
		{"inline-start", document.RightToLeft, document.FloatRight, document.ClearRight},
		{"inline-end", document.LeftToRight, document.FloatRight, document.ClearRight},
		{"none", document.LeftToRight, document.NoFloat, 0},
		{"both", document.LeftToRight, document.NoFloat, document.ClearBoth},
	} {
		opened := &element{name: "div", style: style{float: test.value, inherited: inherited{direction: test.direction}}}
		if float := floatSide(opened); float != test.float {
			t.Errorf("float: %s in %d = %d, want %d", test.value, test.direction, float, test.float)
		}
		if side := clearOf(test.value, test.direction); side != test.clear {
			t.Errorf("clear: %s in %d = %d, want %d", test.value, test.direction, side, test.clear)
		}
	}
}
