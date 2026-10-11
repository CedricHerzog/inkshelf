// Package epub turns the chapters of an EPUB book (versions 2 and 3) into documents that the
// typesetting packages lay out on the reader's pages.
//
// An EPUB is a zip of web pages. Package container opens it and finds its chapters: the spine,
// the book's reading order, lists XHTML documents, each styled by CSS stylesheets and perhaps
// fonts of its own. This package reads each chapter as a browser would, but for an e-ink page
// rather than a screen:
//
//   - A builder reads the XHTML token by token, keeping the open elements on a stack. The
//     stack is what CSS selectors match against (an element knows its parent and its previous
//     sibling), so each element's declared values come from the chapter's cascade as soon as
//     it opens: the user agent's stylesheet first, then the book's.
//   - computeStyle turns the declared values into the few properties the app follows, with
//     inheritance and lengths in hundredths of an em.
//   - Text is gathered into runs of the same style, its whitespace collapsed. A block element
//     ends the block being built and starts another: the document is a list of blocks of runs.
//   - Some elements make more than blocks: a border or a background makes a border box drawn
//     under its blocks, a float makes a float group, a table makes a grid of cells, an image
//     an image block or an image run in a line, MathML a formula and inline SVG an image.
//   - Footnotes are recognized, by epub:type or by a link back to their call, so that the
//     reader can show them in a popup rather than in the text.
//
// A pre-paginated (fixed-layout) chapter is a page designed at a fixed size, not text that
// flows: its elements become the regions of a fixedlayout.Page, which RenderPage draws.
//
// BuildIndex measures every chapter once, at download time, so that the reader knows the
// book's length and where its table of contents leads without laying it all out.
//
// Positions in a document are rune offsets, an image counting as one rune. They never depend on
// the font or the page size, which is why a reading position survives a change of font size.
package epub
