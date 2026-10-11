// Package document is the model of a book's text: what package epub makes of a chapter's XHTML
// and CSS, and what package layout lays out into pages. It keeps only what layout needs, already
// worked out, so that layout knows nothing of HTML or CSS.
//
// A Document is a list of blocks, the boxes of text that stack down a page: a Block is a
// paragraph, a heading, an image or a horizontal rule, with its margins, indent and alignment. A
// paragraph's text is a list of runs: a Run is text in one style (bold, italic, a size, a font
// family, a link), or an inline image. Inline elements of HTML (<em>, <a>, <span>) become runs;
// block elements (<p>, <h1>, <li>) become blocks.
//
// What does not fit that flat list is described around it. A Table is a grid of cells whose
// blocks follow each other in the document, each pointing to its TableCell. A BorderBox draws a
// block element's border and background around the blocks it holds. A floated block names the
// side it goes to and the group of blocks it floats with.
//
// Positions in a document are offsets in runes: each block's text counts its runes, spaces
// included, and an image or a rule counts as one. They never depend on the layout, so a reading
// position saved as an offset finds the same text again at any font size.
//
// Lengths are mostly in hundredths of an em (Hundredths), so that margins and indents grow with
// the reader's font size, and proportions in percent (Percent).
package document
