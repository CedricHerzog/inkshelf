// Package layout typesets a document (package document) into pages for the reader's screen, and
// draws them. It does the job of a browser's layout engine for the parts of CSS that books use,
// in the steps below.
//
//  1. Preparing a block's text. Each run gets a format: its font at its size, its color, its
//     link. The scripts of the text decide what else it needs: Chinese and Japanese, which have
//     no spaces, need the Unicode line breaking algorithm (UAX #14) to find where lines may
//     break; Hebrew and Arabic need the Unicode bidirectional algorithm to give each character a
//     level (even levels run left to right, odd ones right to left); and scripts whose letters
//     join or move, like Arabic and the scripts of India, need shaping by the font. Latin text
//     needs none of these, and pays for none.
//
//  2. Fragments and words. The text is cut at spaces, dashes and other break points into
//     fragments, each measured in its format; the fragments between two break points make a word,
//     the smallest thing a line may hold.
//
//  3. Line breaking. Words fill each line in turn until the next one does not fit (greedy
//     breaking). A word that does not fit may be hyphenated, at the points that package hyphen
//     finds; a word wider than a whole line is cut between two characters.
//
//  4. Line placement. Each line is aligned: left, right, centred, or justified, which spreads
//     the room left over the spaces between its words. The words share one baseline; the line is
//     as high as its largest font or image. Right-to-left text is put in visual order.
//
//  5. Pagination. Blocks go down the page one after the other, separated by their margins, and a
//     new page starts when the next line does not fit. A paragraph keeps its first two lines
//     together and a heading stays with what follows it. Images are fitted in the page; border
//     boxes are drawn around their blocks; floats are pushed to a side with the text flowing
//     around them; tables are laid out as grids whose rows may be cut across pages.
//
//  6. Drawing. A Page holds everything at its place, in pixels; Page.Draw draws it with Faces,
//     which open fonts as they are needed and fall back to other fonts for the characters a font
//     lacks.
//
// Positions in a document are offsets in runes: each block's text counts its runes, spaces
// included, and an image or a rule counts as one. Layout never changes them, and every page
// knows the offsets it shows, so that a reading position survives a new layout at another font
// size.
package layout
