package layout

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// gaps returns the space between each word of line and the next, as drawn.
func gaps(t *testing.T, faces *Faces, line Line) []int {
	t.Helper()
	spaces := make([]int, 0, len(line.Words))
	for index, word := range line.Words[:len(line.Words)-1] {
		font, err := faces.FamilyFont(word.Family, word.Style, word.Size)
		if err != nil {
			t.Fatal(err)
		}
		spaces = append(spaces, line.Words[index+1].X-word.X-font.Width(word.Text))
	}
	return spaces
}

// A space after a dash is kept: "a – b" is three words, a space apart, and so are a dialogue's
// "— Oui" and an aside between dashes.
func TestSpaceAfterDash(t *testing.T) {
	faces := newTestFaces(t)
	font, _ := faces.Font(0, 24)
	space := font.Width(" ")
	for _, text := range []string{"a – b", "— Oui", "a - b", "– pas le sien – lui"} {
		page := paginateOne(t, oneBlock(document.Block{Runs: []document.Run{{Text: text}}}), leftSettings, faces, noImages)
		if got, want := lineText(page.Lines[0]), text; got != want {
			t.Errorf("%q laid out as %q", text, got)
		}
		for index, gap := range gaps(t, faces, page.Lines[0]) {
			if gap != space {
				t.Errorf("%q: gap %d is %d pixels, want a space's %d", text, index, gap, space)
			}
		}
	}
}

// Justification stretches the spaces after dashes as the others.
func TestJustifyAfterDash(t *testing.T) {
	faces := newTestFaces(t)
	pages, err := Paginate(oneBlock(document.Block{Runs: []document.Run{{Text: strings.Repeat("— pas le sien – lui, dit-il. ", 12)}}}), testSettings, faces, noImages)
	if err != nil {
		t.Fatal(err)
	}
	lines := pages[0].Lines
	for index, line := range lines[:len(lines)-1] {
		var spaces []int
		for position, gap := range gaps(t, faces, line) {
			if !strings.HasSuffix(line.Words[position].Text, "dit-") { // no space after dit-
				spaces = append(spaces, gap)
			}
		}
		if len(spaces) < 2 {
			continue
		}
		smallest, largest := spaces[0], spaces[0]
		for _, gap := range spaces {
			smallest, largest = min(smallest, gap), max(largest, gap)
		}
		if largest-smallest > 1 || smallest <= faces.width(textFormat{size: 24}, " ")-1 {
			t.Errorf("line %d %q: spaces %v, want them equal and stretched", index, lineText(line), spaces)
		}
	}
}

// breaksOf returns the text of block cut where breakOpportunities allows breaks.
func breaksOf(block document.Block) []string {
	breaks := breakOpportunities(block, document.ScanScripts(block.Runs).Wide)
	var parts []string
	var part []rune
	index := 0
	for _, run := range block.Runs {
		for _, character := range run.Text {
			part = append(part, character)
			if breaks[index] {
				parts = append(parts, string(part))
				part = nil
			}
			index++
		}
	}
	return append(parts, string(part))
}

// Chinese and Japanese break between ideographs, but not before closing punctuation nor after
// opening one (kinsoku); keep-all keeps words of ideographs whole, break-all breaks Latin
// words too; strict line breaking keeps small kana with what comes before.
func TestBreakOpportunities(t *testing.T) {
	for _, test := range []struct {
		block document.Block
		want  []string
	}{
		{document.Block{Runs: []document.Run{{Text: "漢字、"}, {Text: "かな「引用」です。"}}}, []string{"漢", "字、", "か", "な", "「引", "用」", "で", "す。"}},
		{document.Block{Runs: []document.Run{{Text: "ちょっと待って"}}}, []string{"ち", "ょ", "っ", "と", "待", "っ", "て"}},
		{document.Block{LineBreak: document.LineBreakStrict, Runs: []document.Run{{Text: "ちょっと待って"}}}, []string{"ちょっ", "と", "待っ", "て"}},
		{document.Block{WordBreak: document.KeepAll, Runs: []document.Run{{Text: "漢字、かな 引用"}}}, []string{"漢字、", "かな ", "引用"}},
		{document.Block{WordBreak: document.BreakAll, Runs: []document.Run{{Text: "word, it"}}}, []string{"w", "o", "r", "d, ", "i", "t"}},
		{document.Block{LineBreak: document.LineBreakAnywhere, Runs: []document.Run{{Text: "a,b"}}}, []string{"a", ",", "b"}},
		{document.Block{Runs: []document.Run{{Text: "Latin "}, {Text: "日本"}}}, []string{"Latin ", "日", "本"}},
	} {
		if got := breaksOf(test.block); !slices.Equal(got, test.want) {
			t.Errorf("%+v: breaks %q, want %q", test.block, got, test.want)
		}
	}
	latin := document.Block{Runs: []document.Run{{Text: "Déjà vu — “quotes”, Ελληνικά, Русский"}}}
	if breaks := breakOpportunities(latin, document.ScanScripts(latin.Runs).Wide); breaks != nil {
		t.Errorf("alphabetic text has break opportunities %v, want none: it breaks at spaces", breaks)
	}
}

// Lines of Japanese break between characters, never before a closing bracket or a comma nor
// after an opening bracket; justified, they end at the right margin.
func TestCJKLines(t *testing.T) {
	faces := newTestFaces(t)
	text := strings.Repeat("「日本語の文章です。」と彼は言った、そして笑った。", 6)
	pages, err := Paginate(oneBlock(document.Block{Runs: []document.Run{{Text: text}}}), testSettings, faces, noImages)
	if err != nil {
		t.Fatal(err)
	}
	lines := pages[0].Lines
	if len(lines) < 3 {
		t.Fatalf("%d lines, want several", len(lines))
	}
	content := testSettings.content()
	for index, line := range lines {
		text := strings.ReplaceAll(lineText(line), " ", "")
		if first, _ := utf8.DecodeRuneInString(text); strings.ContainsRune("。、」", first) {
			t.Errorf("line %d %q starts with closing punctuation", index, text)
		}
		if last, _ := utf8.DecodeLastRuneInString(text); last == '「' {
			t.Errorf("line %d %q ends with an opening bracket", index, text)
		}
		if _, right := lineEdges(faces, line); index < len(lines)-1 && right < content.Max.X-1 {
			t.Errorf("line %d %q ends at %d, want justified to %d", index, text, right, content.Max.X)
		}
	}
}

// FuzzBreakOpportunities checks that breakOpportunities gives a break point per rune of any
// text, whatever its settings.
func FuzzBreakOpportunities(f *testing.F) {
	for _, text := range []string{"漢字、かな「引用」", "ちょっと", "word, it", "a\u00adb", "\xff日"} {
		f.Add(text, uint8(0), uint8(0))
	}
	f.Fuzz(func(t *testing.T, text string, wordBreak, lineBreak uint8) {
		block := document.Block{WordBreak: document.WordBreak(wordBreak % 3), LineBreak: document.LineBreak(lineBreak % 5), Runs: []document.Run{{Text: text}, {Image: "image.png"}}}
		if breaks := breakOpportunities(block, document.ScanScripts(block.Runs).Wide); breaks != nil && len(breaks) != block.Length() {
			t.Errorf("%d break points for %d runes", len(breaks), block.Length())
		}
	})
}

// A hyphenated word stays joined, without a space, and may break after its hyphen.
func TestHyphenatedWord(t *testing.T) {
	faces := newTestFaces(t)
	page := paginateOne(t, oneBlock(document.Block{Runs: []document.Run{{Text: "Viens, dit-il."}}}), leftSettings, faces, noImages)
	words := page.Lines[0].Words
	if got := lineText(page.Lines[0]); got != "Viens, dit- il." {
		t.Fatalf("words %q", got)
	}
	if gap := gaps(t, faces, page.Lines[0])[1]; gap != 0 {
		t.Errorf("%q and %q %d pixels apart, want joined", words[1].Text, words[2].Text, gap)
	}
	broken := false
	doc := oneBlock(document.Block{Runs: []document.Run{{Text: strings.Repeat("Viens, dit-il. ", 10)}}})
	for width := 150; width <= 400; width += 7 {
		settings := leftSettings
		settings.Width = width
		pages, err := Paginate(doc, settings, faces, noImages)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range allLines(pages) {
			broken = broken || strings.HasSuffix(line, "dit-")
			if strings.HasPrefix(line, "-") {
				t.Errorf("width %d: line %q starts with the hyphen", width, line)
			}
		}
	}
	if !broken {
		t.Error("no line broke after the hyphen")
	}
}
