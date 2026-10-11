package layout

import (
	"image"
	"slices"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/gomono"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/hyphen"
)

// leftSettings are testSettings, not justified.
var leftSettings = func() Settings {
	settings := testSettings
	settings.Justify = false
	return settings
}()

// paginateOne lays out doc with settings and returns its only page.
func paginateOne(t *testing.T, doc *document.Document, settings Settings, faces *Faces, sizer ImageSizer) Page {
	t.Helper()
	pages, err := Paginate(doc, settings, faces, sizer)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("%d pages, want 1", len(pages))
	}
	return pages[0]
}

// oneBlock returns a document of a paragraph of runs.
func oneBlock(block document.Block) *document.Document {
	doc := &document.Document{}
	doc.Add(block)
	return doc
}

// baseline returns the baseline of word.
func baseline(t *testing.T, faces *Faces, word PlacedWord) int {
	t.Helper()
	font, err := faces.FamilyFont(word.Family, word.Style, word.Size)
	if err != nil {
		t.Fatal(err)
	}
	return word.Top + font.Ascent()
}

// wordAt returns the word of page whose text is text.
func wordAt(t *testing.T, page Page, text string) PlacedWord {
	t.Helper()
	for _, line := range page.Lines {
		for _, word := range line.Words {
			if word.Text == text {
				return word
			}
		}
	}
	t.Fatalf("no word %q in %q", text, allLines([]Page{page}))
	return PlacedWord{}
}

// whiteCanvas returns a white canvas of settings' page.
func whiteCanvas(settings Settings) *image.Gray {
	canvas := image.NewGray(image.Rect(0, 0, settings.Width, settings.Height))
	for index := range canvas.Pix {
		canvas.Pix[index] = 255
	}
	return canvas
}

// Words of mixed sizes share the line's baseline, which a larger run moves down, and make the
// line taller; a shifted run is raised.
func TestMixedSizes(t *testing.T) {
	faces := newTestFaces(t)
	doc := &document.Document{}
	doc.Add(document.Block{Runs: []document.Run{{Text: "small "}, {Text: "BIG", FontScale: 200}, {Text: " note", Style: document.Small, Shift: 33}}})
	doc.Add(document.Block{Runs: []document.Run{{Text: "next"}}})
	doc.Add(document.Block{Runs: []document.Run{{Text: "last"}}})
	page := paginateOne(t, doc, leftSettings, faces, noImages)
	small, big, note := wordAt(t, page, "small"), wordAt(t, page, "BIG"), wordAt(t, page, "note")
	if baseline(t, faces, small) != baseline(t, faces, big) {
		t.Errorf("baselines %d and %d differ", baseline(t, faces, small), baseline(t, faces, big))
	}
	if raised := baseline(t, faces, small) - baseline(t, faces, note); raised != 33*24/100 {
		t.Errorf("shifted run raised by %d, want %d", raised, 33*24/100)
	}
	plain := leftSettings.FontSize * leftSettings.LineSpacing / 100
	next, last := wordAt(t, page, "next"), wordAt(t, page, "last")
	if got := next.Top - small.Top; got <= plain {
		t.Errorf("the line of a larger run is %d high, want more than %d", got, plain)
	}
	if got := last.Top - next.Top; got != plain {
		t.Errorf("a plain line is %d high, want %d", got, plain)
	}
}

// A block's LineHeight replaces the settings' line spacing.
func TestLineHeight(t *testing.T) {
	doc := &document.Document{}
	doc.Add(document.Block{LineHeight: 200, Runs: []document.Run{{Text: "one"}}})
	doc.Add(document.Block{Runs: []document.Run{{Text: "two"}}})
	page := paginateOne(t, doc, leftSettings, newTestFaces(t), noImages)
	if got := wordAt(t, page, "two").Top - wordAt(t, page, "one").Top; got != 48 {
		t.Errorf("line %d high, want 48", got)
	}
}

// Letter spacing widens the run as measured and as drawn.
func TestLetterSpacing(t *testing.T) {
	faces := newTestFaces(t)
	plain := paginateOne(t, oneBlock(document.Block{Runs: []document.Run{{Text: "abcd"}, {Text: " next"}}}), leftSettings, faces, noImages)
	spaced := paginateOne(t, oneBlock(document.Block{Runs: []document.Run{{Text: "abcd", LetterSpacing: 50}, {Text: " next"}}}), leftSettings, faces, noImages)
	spacing := 50 * 24 / 100
	if got, want := wordAt(t, spaced, "next").X, wordAt(t, plain, "next").X+4*spacing+spacing; got != want {
		t.Errorf("next word at %d, want %d", got, want) // the space after is spaced too
	}
	right := func(page Page) int {
		canvas := whiteCanvas(leftSettings)
		page.Lines[0].Words = page.Lines[0].Words[:1] // abcd only
		if err := page.Draw(canvas, faces, nil); err != nil {
			t.Fatal(err)
		}
		rightmost := 0
		for y := range canvas.Rect.Dy() {
			for x := range canvas.Rect.Dx() {
				if canvas.GrayAt(x, y).Y < 128 {
					rightmost = max(rightmost, x)
				}
			}
		}
		return rightmost
	}
	if got, want := right(spaced)-right(plain), 3*spacing; got < want-1 || got > want+1 {
		t.Errorf("the last letter drawn %d pixels further, want %d", got, want)
	}
}

// Underlines and strike-throughs run under the words of their run, across its spaces, and are
// drawn.
func TestStrokes(t *testing.T) {
	faces := newTestFaces(t)
	doc := oneBlock(document.Block{Runs: []document.Run{{Text: "plain "}, {Text: "two words", Style: document.Underline | document.Strike}, {Text: " after"}}})
	page := paginateOne(t, doc, leftSettings, faces, noImages)
	if len(page.Strokes) != 2 {
		t.Fatalf("strokes %v, want an underline and a strike-through", page.Strokes)
	}
	two, words := wordAt(t, page, "two"), wordAt(t, page, "words")
	font, _ := faces.Font(0, 24)
	underline, strike := page.Strokes[0], page.Strokes[1]
	if underline.Min.X != two.X || underline.Max.X != words.X+font.Width("words") {
		t.Errorf("underline from %d to %d, want %d to %d", underline.Min.X, underline.Max.X, two.X, words.X+font.Width("words"))
	}
	if base := baseline(t, faces, two); underline.Min.Y <= base || strike.Max.Y >= base || strike.Min.Y <= two.Top {
		t.Errorf("underline at %d, strike-through at %d, baseline %d", underline.Min.Y, strike.Min.Y, base)
	}
	canvas := whiteCanvas(leftSettings)
	if err := page.Draw(canvas, faces, nil); err != nil {
		t.Fatal(err)
	}
	// In the space between the words.
	if gray := canvas.GrayAt(words.X-2, underline.Min.Y).Y; gray != 0 {
		t.Errorf("underline not drawn: %d", gray)
	}
}

// A raised run, a footnote call, is underlined below the line's baseline, as the line's text
// is, clear of its digits; a lowered one, below its own baseline.
func TestShiftedUnderline(t *testing.T) {
	faces := newTestFaces(t)
	doc := oneBlock(document.Block{Runs: []document.Run{
		{Text: "note", Style: document.Underline}, {Text: "1", Style: document.Small | document.Underline, Shift: 33},
		{Text: " water"}, {Text: "2", Style: document.Small | document.Underline, Shift: -20},
	}})
	page := paginateOne(t, doc, leftSettings, faces, noImages)
	if len(page.Strokes) != 3 {
		t.Fatalf("strokes %v, want 3 underlines", page.Strokes)
	}
	note, raised, lowered := page.Strokes[0], page.Strokes[1], page.Strokes[2]
	if raised.Min.Y != note.Min.Y || raised.Dy() != note.Dy() {
		t.Errorf("a raised run's underline %v, want the line's text's %v", raised, note)
	}
	if call := wordAt(t, page, "1"); raised.Min.Y <= baseline(t, faces, call)+1 {
		t.Errorf("a raised run's underline at %d touches it: its baseline at %d", raised.Min.Y, baseline(t, faces, call))
	}
	if subscript := wordAt(t, page, "2"); lowered.Min.Y <= baseline(t, faces, subscript) {
		t.Errorf("a lowered run's underline at %d, above its baseline %d", lowered.Min.Y, baseline(t, faces, subscript))
	}
}

// Small capitals are the lowercase letters drawn as capitals of a smaller font.
func TestSmallCaps(t *testing.T) {
	faces := newTestFaces(t)
	page := paginateOne(t, oneBlock(document.Block{Runs: []document.Run{{Text: "Hello", Style: document.SmallCaps}, {Text: " next"}}}), leftSettings, faces, noImages)
	font, _ := faces.Font(0, 24)
	small, _ := faces.Font(0, 24*smallCapsScale/100)
	if got, want := wordAt(t, page, "next").X-wordAt(t, page, "Hello").X, font.Width("H")+small.Width("ELLO")+font.Width(" "); got != want {
		t.Errorf("small capitals %d wide with the space, want %d", got, want)
	}
	if !(font.Width("Hello") < small.Width("HELLO")) {
		t.Errorf("small capitals no wider than lowercase")
	}
}

// An inline image sits on the baseline, at its size or ImageHeight high, and makes its line
// taller; one too wide fits the line.
func TestInlineImages(t *testing.T) {
	faces := newTestFaces(t)
	sizes := map[string]image.Point{"icon.png": image.Pt(40, 20), "wide.png": image.Pt(2000, 100)}
	sizer := func(path string) (image.Point, error) { return sizes[path], nil }
	doc := &document.Document{}
	doc.Add(document.Block{Runs: []document.Run{{Text: "an "}, {Image: "icon.png"}, {Text: " icon"}}})
	doc.Add(document.Block{Runs: []document.Run{{Text: "big "}, {Image: "icon.png", ImageHeight: 300}}})
	doc.Add(document.Block{Runs: []document.Run{{Text: "after"}}})
	doc.Add(document.Block{Runs: []document.Run{{Image: "wide.png"}}})
	page := paginateOne(t, doc, leftSettings, faces, sizer)
	if len(page.Images) != 3 {
		t.Fatalf("images %+v, want 3", page.Images)
	}
	an, icon := wordAt(t, page, "an"), page.Images[0].Rect
	if icon.Size() != image.Pt(40, 20) || icon.Max.Y != baseline(t, faces, an) {
		t.Errorf("icon at %v, want 40x20 on the baseline %d", icon, baseline(t, faces, an))
	}
	if after := wordAt(t, page, "icon"); after.X-icon.Max.X != faces.width(textFormat{size: 24}, " ") {
		t.Errorf("the word after the icon at %d, the icon ends at %d", after.X, icon.Max.X)
	}
	bigIcon := page.Images[1].Rect
	if bigIcon.Size() != image.Pt(144, 72) {
		t.Errorf("icon 3em high is %v, want 144x72", bigIcon.Size())
	}
	plain := leftSettings.FontSize * leftSettings.LineSpacing / 100
	// The icon is at the top of its line.
	if got := wordAt(t, page, "after").Top - bigIcon.Min.Y; got <= 72 || got >= 72+plain {
		t.Errorf("the line of the big icon is %d high", got)
	}
	if wide := page.Images[2].Rect; wide.Dx() != leftSettings.content().Dx() {
		t.Errorf("wide image %v, want the text's width", wide)
	}
	if page.End != doc.Chars || doc.Chars != len("an ")+1+len(" icon")+len("big ")+1+len("after")+1 {
		t.Errorf("document %d runes, page ends at %d", doc.Chars, page.End)
	}
}

// Block.Right narrows the lines, justified to it.
func TestRightMargin(t *testing.T) {
	faces := newTestFaces(t)
	doc := oneBlock(document.Block{Right: 500, Runs: []document.Run{{Text: strings.Repeat("Une phrase pour remplir. ", 6)}}})
	page := paginateOne(t, doc, testSettings, faces, noImages)
	right := testSettings.content().Max.X - 120
	for index, line := range page.Lines[:len(page.Lines)-1] {
		last := line.Words[len(line.Words)-1]
		font, _ := faces.Font(last.Style, last.Size)
		if end := last.X + font.Width(last.Text); end != right {
			t.Errorf("line %d ends at %d, want %d", index, end, right)
		}
	}
}

// Borders are rules across the block's text, around it.
func TestBorders(t *testing.T) {
	doc := oneBlock(document.Block{Left: 100, BorderTop: true, BorderBottom: true, Runs: []document.Run{{Text: "boxed"}}})
	page := paginateOne(t, doc, leftSettings, newTestFaces(t), noImages)
	if len(page.Rules) != 2 {
		t.Fatalf("rules %v, want 2", page.Rules)
	}
	content := leftSettings.content()
	top, bottom, word := page.Rules[0], page.Rules[1], wordAt(t, page, "boxed")
	if top.Min.X != content.Min.X+24 || top.Max.X != content.Max.X || bottom.Dx() != top.Dx() {
		t.Errorf("rules %v and %v, want them across the text", top, bottom)
	}
	if top.Max.Y > word.Top || bottom.Min.Y < word.Top+leftSettings.FontSize {
		t.Errorf("rules at %d and %d around a line at %d", top.Min.Y, bottom.Min.Y, word.Top)
	}
}

// An image block of ImageWidth is that share of the text's width.
func TestImageWidth(t *testing.T) {
	doc := oneBlock(document.Block{Kind: document.Image, Image: "small.png", ImageWidth: 50})
	sizer := func(string) (image.Point, error) { return image.Pt(40, 20), nil }
	page := paginateOne(t, doc, leftSettings, newTestFaces(t), sizer)
	if size := page.Images[0].Rect.Size(); size != image.Pt(270, 135) {
		t.Errorf("image %v, want 270x135", size)
	}
}

// Hyphenation breaks words at their hyphenation points, with a hyphen drawn, only when asked.
func TestHyphenation(t *testing.T) {
	text := strings.Repeat("The extraordinary representation of international organizations. ", 6)
	faces := newTestFaces(t)
	hyphenator := hyphen.For("en")
	for _, hyphenate := range []bool{false, true} {
		doc := oneBlock(document.Block{Hyphenate: hyphenate, Runs: []document.Run{{Text: text}}})
		doc.Language = "en-US"
		hyphenated := 0
		for width := 200; width <= 500; width += 9 {
			settings := testSettings
			settings.Width = width
			pages, err := Paginate(doc, settings, faces, noImages)
			if err != nil {
				t.Fatal(err)
			}
			lines := allLines(pages)
			for index, line := range lines[:len(lines)-1] {
				if !strings.HasSuffix(line, "-") {
					continue
				}
				hyphenated++
				words := strings.Fields(line)
				head := strings.TrimSuffix(words[len(words)-1], "-")
				tail := strings.Fields(lines[index+1])[0]
				if !slices.Contains(hyphenator.Points(head+tail), len(head)) {
					t.Errorf("width %d: %q broken at %d, not a hyphenation point", width, head+tail, len(head))
				}
			}
		}
		if hyphenate != (hyphenated > 0) {
			t.Errorf("hyphenate %v: %d lines hyphenated", hyphenate, hyphenated)
		}
	}
}

// A link's words on a line make one box, the line's height.
func TestLinks(t *testing.T) {
	faces := newTestFaces(t)
	doc := oneBlock(document.Block{Runs: []document.Run{{Text: "see "}, {Text: "the ", Link: "notes.xhtml#n1"}, {Text: "note", Style: document.Italic, Link: "notes.xhtml#n1"}, {Text: " here"}}})
	page := paginateOne(t, doc, leftSettings, faces, noImages)
	if len(page.Links) != 1 || page.Links[0].Target != "notes.xhtml#n1" {
		t.Fatalf("links %+v, want one", page.Links)
	}
	the, note := wordAt(t, page, "the"), wordAt(t, page, "note")
	italic, _ := faces.Font(document.Italic, 24)
	want := image.Rect(the.X, the.Top, note.X+italic.Width("note"), the.Top+leftSettings.FontSize*leftSettings.LineSpacing/100)
	if page.Links[0].Rect != want {
		t.Errorf("link at %v, want %v", page.Links[0].Rect, want)
	}
}

// HideFootnotes leaves out a footnote linked from its document, keeping its offsets, and shows
// one that nothing in the document links to (an endnote).
func TestHideFootnotes(t *testing.T) {
	faces := newTestFaces(t)
	doc := &document.Document{}
	doc.Add(document.Block{Runs: []document.Run{{Text: "text"}, {Text: "1", Link: "chapter.xhtml#fn1"}}})
	doc.Add(document.Block{Runs: []document.Run{{Text: "footnote"}}, Footnote: "fn1"})
	doc.Add(document.Block{Runs: []document.Run{{Text: "endnote"}}, Footnote: "en1"})
	settings := leftSettings
	settings.HideFootnotes = true
	page := paginateOne(t, doc, settings, faces, noImages)
	if got := allLines([]Page{page}); !slices.Equal(got, []string{"text 1", "endnote"}) {
		t.Errorf("lines %q, want the text and the endnote", got)
	}
	if page.End != doc.Chars {
		t.Errorf("page ends at %d, want %d", page.End, doc.Chars)
	}
}

// A family the document embeds is used, its regular style for a style it lacks; a generic
// family is the reader's; an unknown one is the book font.
func TestFamilies(t *testing.T) {
	faces := newTestFaces(t)
	mono, err := fonts.Parse("mono", gomono.TTF)
	if err != nil {
		t.Fatal(err)
	}
	doc := oneBlock(document.Block{Runs: []document.Run{
		{Text: "iiii ", Family: "Mono"}, {Text: "jjjj ", Family: "Mono", Style: document.Bold},
		{Text: "kkkk ", Family: "monospace"}, {Text: "llll ", Family: "Unknown"}, {Text: "end"},
	}})
	doc.Fonts = map[string]map[fonts.Style]*fonts.File{"mono": {fonts.Regular: mono}}
	page := paginateOne(t, doc, leftSettings, faces, noImages)
	monoFont, err := mono.Open(24)
	if err != nil {
		t.Fatal(err)
	}
	book, _ := faces.Font(0, 24)
	goMono, _ := fonts.Open(fonts.Monospace, fonts.Regular, 24)
	tests := []struct {
		word, next string
		want       int
	}{
		{"iiii", "jjjj", monoFont.Width("iiii ")},
		{"jjjj", "kkkk", monoFont.Width("jjjj ")},
		{"kkkk", "llll", goMono.Width("kkkk ")},
		{"llll", "end", book.Width("llll ")},
	}
	for _, test := range tests {
		if got := wordAt(t, page, test.next).X - wordAt(t, page, test.word).X; got != test.want {
			t.Errorf("%q is %d wide, want %d", test.word, got, test.want)
		}
	}
	if err := page.Draw(whiteCanvas(leftSettings), faces, nil); err != nil {
		t.Fatal(err)
	}
}

// lineEdges returns where the text of line starts and ends, measured in faces.
func lineEdges(faces *Faces, line Line) (left, right int) {
	first, last := line.Words[0], line.Words[len(line.Words)-1]
	return first.X, last.X + faces.width(last.format(), last.Text)
}

// The last line of a block aligns as AlignLast says: centred, justified to both margins.
func TestAlignLast(t *testing.T) {
	faces := newTestFaces(t)
	text := strings.Repeat("Il fallait partir avant l'aube, sans bruit. ", 4) + "Fin du jour"
	content := testSettings.content()
	for _, test := range []struct {
		alignLast document.Align
		check     func(left, right int) bool
	}{
		{document.AlignDefault, func(left, right int) bool { return left == content.Min.X && right < content.Max.X-50 }},
		{document.AlignCenter, func(left, right int) bool {
			return left > content.Min.X+50 && max(left-content.Min.X-(content.Max.X-right), content.Max.X-right-(left-content.Min.X)) <= 1
		}},
		{document.AlignJustify, func(left, right int) bool { return left == content.Min.X && right >= content.Max.X-1 }},
		{document.AlignEnd, func(left, right int) bool { return right >= content.Max.X-1 && left > content.Min.X+50 }},
	} {
		page := paginateOne(t, oneBlock(document.Block{AlignLast: test.alignLast, Runs: []document.Run{{Text: text}}}), testSettings, faces, noImages)
		if len(page.Lines) < 2 {
			t.Fatalf("%d lines, want several", len(page.Lines))
		}
		if left, right := lineEdges(faces, page.Lines[len(page.Lines)-1]); !test.check(left, right) {
			t.Errorf("AlignLast %d: last line from %d to %d, in %d to %d", test.alignLast, left, right, content.Min.X, content.Max.X)
		}
		if left, right := lineEdges(faces, page.Lines[0]); left != content.Min.X || right < content.Max.X-1 {
			t.Errorf("AlignLast %d: first line from %d to %d, want justified", test.alignLast, left, right)
		}
	}
}

// AlignStart and AlignEnd follow the direction; right-to-left text starts at the right.
func TestAlignStartEnd(t *testing.T) {
	faces := newTestFaces(t)
	content := leftSettings.content()
	for _, test := range []struct {
		block document.Block
		right bool
	}{
		{document.Block{Align: document.AlignStart}, false},
		{document.Block{Align: document.AlignEnd}, true},
		{document.Block{Align: document.AlignStart, Direction: document.RightToLeft}, true},
		{document.Block{Align: document.AlignEnd, Direction: document.RightToLeft}, false},
		{document.Block{Direction: document.RightToLeft}, true},
	} {
		test.block.Runs = []document.Run{{Text: "a short line"}}
		page := paginateOne(t, oneBlock(test.block), leftSettings, faces, noImages)
		left, right := lineEdges(faces, page.Lines[0])
		if atRight := right >= content.Max.X-1 && left > content.Min.X; atRight != test.right {
			t.Errorf("align %d, direction %d: line from %d to %d", test.block.Align, test.block.Direction, left, right)
		}
	}
}

// An underline under the descenders goes below the font's descent; the other, above it.
func TestUnderlineUnder(t *testing.T) {
	faces := newTestFaces(t)
	font, _ := faces.Font(0, 24)
	page := paginateOne(t, oneBlock(document.Block{Runs: []document.Run{{Text: "plain", Style: document.Underline}, {Text: " gypsy", Style: document.Underline | document.UnderlineUnder}}}), leftSettings, faces, noImages)
	if len(page.Strokes) != 2 {
		t.Fatalf("strokes %v, want 2", page.Strokes)
	}
	base := baseline(t, faces, wordAt(t, page, "plain"))
	if plain, under := page.Strokes[0], page.Strokes[1]; plain.Min.Y >= base+font.Descent() || under.Min.Y != base+font.Descent() {
		t.Errorf("underlines at %d and %d, baseline %d, descent %d", plain.Min.Y, under.Min.Y, base, font.Descent())
	}
}

// Emphasis marks are words half the text's size, centred above (or under) each character but
// spaces and punctuation; they make the line taller.
func TestEmphasisMarks(t *testing.T) {
	faces := newTestFaces(t)
	font, _ := faces.Font(0, 24)
	markFont, _ := faces.Font(0, 12)
	plainHeight := leftSettings.FontSize * leftSettings.LineSpacing / 100
	for _, under := range []bool{false, true} {
		doc := &document.Document{}
		doc.Add(document.Block{Runs: []document.Run{{Text: "mark it, ", Emphasis: document.Emphasis{Mark: '•', Under: under}}, {Text: "now"}}})
		doc.Add(document.Block{Runs: []document.Run{{Text: "next"}}})
		page := paginateOne(t, doc, leftSettings, faces, noImages)
		var marks []PlacedWord
		for _, word := range page.Lines[0].Words {
			if word.Text == "•" {
				marks = append(marks, word)
			}
		}
		if len(marks) != 6 {
			t.Fatalf("under %v: %d marks, want one per letter: %q", under, len(marks), lineText(page.Lines[0]))
		}
		text := wordAt(t, page, "mark")
		if marks[0].Size != 12 {
			t.Errorf("marks of size %d, want 12", marks[0].Size)
		}
		if center, want := marks[0].X+markFont.Width("•")/2, text.X+font.Width("m")/2; center < want-1 || center > want+1 {
			t.Errorf("mark centred at %d, want %d over its letter", center, want)
		}
		if above := marks[0].Top+markFont.Ascent() <= text.Top; above == under {
			t.Errorf("under %v: mark at %d, text at %d", under, marks[0].Top, text.Top)
		}
		if height := wordAt(t, page, "next").Top - leftSettings.Margins.Min.Y; height <= plainHeight {
			t.Errorf("under %v: line with marks %d high, want more than %d", under, height, plainHeight)
		}
		if err := page.Draw(whiteCanvas(leftSettings), faces, nil); err != nil {
			t.Fatal(err)
		}
	}
}

// A mark that no font has, sesame, is drawn as a dot; an open one as an open dot.
func TestEmphasisMarkFallback(t *testing.T) {
	faces := newTestFaces(t)
	for mark, want := range map[rune]string{'﹅': "•", '﹆': "◦", '▲': "▲"} {
		page := paginateOne(t, oneBlock(document.Block{Runs: []document.Run{{Text: "a", Emphasis: document.Emphasis{Mark: mark}}}}), leftSettings, faces, noImages)
		if got := page.Lines[0].Words[1].Text; got != want {
			t.Errorf("mark %q drawn as %q, want %q", mark, got, want)
		}
	}
}

// A chapter of a novel, hyphenated: compare with BenchmarkPaginateChapter.
func BenchmarkPaginateHyphenated(b *testing.B) {
	doc := testDocument(60)
	doc.Language = "fr"
	for index := range doc.Blocks {
		doc.Blocks[index].Hyphenate = true
	}
	faces := newTestFaces(b)
	for b.Loop() {
		if _, err := Paginate(doc, benchmarkSettings, faces, noImages); err != nil {
			b.Fatal(err)
		}
	}
}
