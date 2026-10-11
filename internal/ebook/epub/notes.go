package epub

import (
	"bytes"
	"cmp"
	"encoding/xml"
	"slices"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/container"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// Notes without an epub:type, as publishers write them: the call is a link with an id of its
// own, and the note begins with a link back to it.
//
//	<a class="apnote" id="ap_ntb-006-1" href="#ntb-006"><sup>1</sup></a>
//	<div class="ntb" id="ntb-006"><p><a href="#ap_ntb-006-1">1</a>. The note…</p></div>
//
// The note is the element at the target, or the block the target is at the start of, and the
// blocks after it in the same parent, up to the next note or heading. Neither a heading, a
// target of the navigation nor the start of a chapter is one: tables of contents and chapter
// titles link to each other too.

// noteCandidate is an element at the target of a call whose text has not started yet: a note
// if its first run links back to one of callers.
type noteCandidate struct {
	target  *element
	id      string
	callers []string // "path#id"
}

// callsTo returns the calls that lead to target ("path#fragment"): "path#id" of each link
// with an id that does.
func (book *Book) callsTo(target string) []string {
	book.callsOnce.Do(book.readCalls)
	return book.calls[target]
}

// readCalls finds the links with an id of the spine's documents: a scan of their <a> tags,
// cheaper than parsing them. Those of the navigation document and of a table of contents that
// its landmarks name, and those that lead to its targets, are left out.
func (book *Book) readCalls() {
	navigated := map[string]bool{}
	skipped := map[string]bool{book.NavigationPath(): true}
	navigation, _ := book.Navigation()
	for _, links := range [][]container.Link{navigation.TOC, navigation.PageList, navigation.Landmarks} {
		for _, link := range links {
			navigated[link.Path+"#"+link.Fragment] = true
			if slices.Contains(strings.Fields(link.Kind), "toc") {
				skipped[link.Path] = true
			}
		}
	}
	book.calls = map[string][]string{}
	for _, item := range book.Spine() {
		documentPath := item.Content
		if skipped[documentPath] || !container.IsDocumentType(item.ContentType) {
			continue
		}
		skipped[documentPath] = true // read once
		data, err := book.Resource(documentPath)
		if err != nil {
			continue
		}
		for _, attributes := range anchorTags(data) {
			id := cmp.Or(attributes["id"], attributes["name"])
			target := linkTarget(documentPath, attributes["href"])
			if id != "" && strings.Contains(target, "#") && !strings.Contains(target, ":") && !navigated[target] {
				book.calls[target] = append(book.calls[target], documentPath+"#"+id)
			}
		}
	}
}

// anchorTags returns the attributes of the <a> start tags of an XHTML document, by lowercase
// local name.
func anchorTags(data []byte) []map[string]string {
	var tags []map[string]string
	for offset := 0; ; {
		start := bytes.IndexByte(data[offset:], '<')
		if start < 0 {
			return tags
		}
		start += offset
		offset = start + 1
		if start+2 >= len(data) || data[start+1] != 'a' && data[start+1] != 'A' || !isXMLSpace(data[start+2]) {
			continue
		}
		end := bytes.IndexByte(data[start:], '>')
		if end < 0 {
			return tags
		}
		token, err := container.NewDecoder(data[start : start+end+1]).Token()
		if tag, ok := token.(xml.StartElement); err == nil && ok {
			tags = append(tags, attributesOf(tag.Attr))
		}
		offset = start + end + 1
	}
}

// isXMLSpace reports whether character is XML whitespace.
func isXMLSpace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\n' || character == '\r'
}

// considerNote makes opened, whose id is id, the candidate note if links lead to it: unless it
// is at the start of a chapter, or inline after text of its block.
func (parser *builder) considerNote(opened *element, id string) {
	if id == "" || parser.forIndex || opened.note != nil && !opened.note.backLinked {
		return
	}
	atStart := parser.doc.Chars == 0 && parser.pendingRunes == 0
	isBlock := opened.style.display == "block" || opened.style.display == "list-item"
	if atStart && parser.book.ChapterOfPath(parser.chapterPath) >= 0 || !isBlock && parser.pendingRunes > 0 {
		return
	}
	if callers := parser.book.callsTo(parser.chapterPath + "#" + id); len(callers) > 0 {
		parser.candidate = &noteCandidate{target: opened, id: id, callers: callers}
	}
}

// checkBackLink makes the candidate note one if run, the first after it, links back to a call
// of it out of a heading. The note is the candidate's element if it is an open block, else the
// block it begins; with the open elements in it.
func (parser *builder) checkBackLink(run document.Run) {
	candidate := parser.candidate
	if candidate == nil {
		return
	}
	parser.candidate = nil
	if run.Link == "" || !slices.Contains(candidate.callers, run.Link) || parser.blockElement().block.kind == document.Heading {
		return
	}
	note, flat := parser.blockElement(), true
	if candidate.target.block != nil && slices.Contains(parser.stack, candidate.target) {
		note, flat = candidate.target, candidate.target == note
	}
	if note.name == "body" || note.name == "html" || note.name == documentName {
		return
	}
	note.noteID, note.backLinked, note.flatNote = candidate.id, true, flat
	for _, opened := range parser.stack[slices.Index(parser.stack, note):] {
		opened.note = note
	}
	parser.lastNote = nil
}

// continueNote makes opened, a block, part of the last flat note closed if it follows it in
// the same parent; a heading ends that note.
func (parser *builder) continueNote(opened *element) {
	lastNote := parser.lastNote
	if lastNote == nil || opened.parent != lastNote.parent || opened.note != nil {
		return
	}
	if opened.block.kind == document.Heading {
		parser.lastNote = nil
		return
	}
	opened.note = lastNote
}

// closeNote follows the closing of an element: a flat note's own, after which the blocks in
// the same parent continue it; or its parent's, which ends it.
func (parser *builder) closeNote(closed *element) {
	switch {
	case closed.flatNote:
		parser.lastNote = closed
	case parser.lastNote != nil && closed == parser.lastNote.parent:
		parser.lastNote = nil
	}
}

// noteTypes are the epub:types of an element that is a note.
var noteTypes = []string{"footnote", "endnote", "rearnote", "note"}

// isNoteType reports whether an epub:type makes an element a note.
func isNoteType(epubType string) bool { return slices.Contains(noteTypes, epubType) }

// noteIDOf returns the id of the footnote an element is in, "" if none.
func noteIDOf(opened *element) string {
	if opened.note == nil {
		return ""
	}
	return opened.note.noteID
}
