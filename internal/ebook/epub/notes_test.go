package epub

import (
	"cmp"
	"strings"
	"testing"
)

// footnotesOf returns the Footnote of each block of a chapter of book, "-" for none, and its
// text.
func footnotesOf(t *testing.T, book *Book, itemPath string) []string {
	t.Helper()
	doc, err := book.Document(itemPath)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, block := range doc.Blocks {
		var text strings.Builder
		for _, run := range block.Runs {
			text.WriteString(run.Text)
		}
		got = append(got, cmp.Or(block.Footnote, "-")+" "+text.String())
	}
	return got
}

// checkFootnotes checks the blocks of a document against want.
func checkFootnotes(t *testing.T, book *Book, itemPath string, want ...string) {
	t.Helper()
	if got := footnotesOf(t, book, itemPath); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("%s:\n got %q\nwant %q", itemPath, got, want)
	}
}

// A French publisher's notes, in the chapter: the call has an id, the note links back to it.
func TestNotesByBackLinkInChapter(t *testing.T) {
	book := openPackage(t, "", "", `<item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/>`,
		`><itemref idref="c1"/>`, map[string]string{
			"OEBPS/c1.xhtml": xhtml(`<h1>Un</h1><p>Texte<a class="apnote" id="ap_ntb-006-1" href="#ntb-006"><sup>1</sup></a>.</p>
<p>Suite.</p>
<div class="ntb" id="ntb-006"><p><a href="#ap_ntb-006-1">1</a>. La note.</p><p>Encore.</p></div>
<p>Fin.</p>`),
		})
	checkFootnotes(t, book, "OEBPS/c1.xhtml", "- Un", "- Texte1.", "- Suite.", "ntb-006 1. La note.", "ntb-006 Encore.", "- Fin.")
}

// Notes in a document of their own, linear or not: a paragraph with the id, or the link back
// with it; the paragraphs after one continue it, up to the next note or a heading.
func TestNotesByBackLinkAcrossDocuments(t *testing.T) {
	notes := `<h1>Notes</h1><p id="n1"><a href="ch01.xhtml#r1">1</a> Une.</p><p>Suite de une.</p>
<p><a id="n2" href="ch01.xhtml#r2">2</a> Deux.</p><h2>Chapitre 2</h2><p>Pas une note.</p>`
	for _, linear := range []string{"yes", "no"} {
		book := openPackage(t, "", "", `<item id="c1" href="ch01.xhtml" media-type="application/xhtml+xml"/>
<item id="notes" href="notes.xhtml" media-type="application/xhtml+xml"/>`,
			`><itemref idref="c1"/><itemref idref="notes" linear="`+linear+`"/>`, map[string]string{
				"OEBPS/ch01.xhtml":  xhtml(`<p>Un<a id="r1" href="notes.xhtml#n1">1</a> deux<a id="r2" href="notes.xhtml#n2">2</a>.</p>`),
				"OEBPS/notes.xhtml": xhtml(notes),
			})
		checkFootnotes(t, book, "OEBPS/notes.xhtml", "- Notes", "n1 1 Une.", "n1 Suite de une.", "n2 2 Deux.", "- Chapitre 2", "- Pas une note.")
	}
}

// A table of contents and the chapter titles that link back to it are no notes: a heading, a
// target of the navigation, the start of a chapter.
func TestNotesByBackLinkNotTOC(t *testing.T) {
	book := openPackage(t, "", "", `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
<item id="toc" href="toc.xhtml" media-type="application/xhtml+xml"/>
<item id="c1" href="ch01.xhtml" media-type="application/xhtml+xml"/>`,
		`><itemref idref="toc"/><itemref idref="c1"/>`, map[string]string{
			"OEBPS/nav.xhtml": xhtml(`<nav epub:type="toc"><ol><li><a id="nav2" href="ch01.xhtml#c2">Deux</a></li></ol></nav>`),
			"OEBPS/toc.xhtml": xhtml(`<p><a id="t0" href="ch01.xhtml#c0">Début</a></p><p><a id="t1" href="ch01.xhtml#c1">Un</a></p>
<p><a id="t2" href="ch01.xhtml#c2">Deux</a></p>`),
			"OEBPS/ch01.xhtml": xhtml(`<p class="title" id="c0"><a href="toc.xhtml#t0">Début</a></p><p>Texte.</p>
<h2 id="c1"><a href="toc.xhtml#t1">Un</a></h2><p>Texte.</p>
<p class="title" id="c2"><a href="toc.xhtml#t2">Deux</a></p><p>Texte.</p>`),
		})
	checkFootnotes(t, book, "OEBPS/ch01.xhtml", "- Début", "- Texte.", "- Un", "- Texte.", "- Deux", "- Texte.")
}

// A cross-reference is no note: its target does not begin with a link back to it.
func TestNotesByBackLinkNotCrossReference(t *testing.T) {
	book := openPackage(t, "", "", `<item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/>`,
		`><itemref idref="c1"/>`, map[string]string{
			"OEBPS/c1.xhtml": xhtml(`<p>Début.</p><p>Voir <a id="x1" href="#fig">la figure</a> et <a id="x2" href="#sec">la section</a>.</p>
<p id="fig"><a href="#other">Figure</a> 1.</p>
<p id="sec">Section, <a href="#x2">retour</a>.</p>`),
		})
	checkFootnotes(t, book, "OEBPS/c1.xhtml", "- Début.", "- Voir la figure et la section.", "- Figure 1.", "- Section, retour.")
}
