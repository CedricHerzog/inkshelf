// Package mathml lays out and draws the formulas that books write in presentation MathML.
//
// Presentation MathML describes how a formula looks, as a tree of elements: tokens hold text
// (<mi> an identifier, <mn> a number, <mo> an operator, <mtext> text), and layout elements
// arrange their children (<mrow> a row, <mfrac> a fraction, <msqrt> a square root, <msup> a
// superscript, <munderover> limits under and over, <mtable> a matrix...). Read and Parse turn
// the markup into a tree of Nodes, refusing formulas past a size that would take too long.
//
// Layout follows TeX's model, in a simplified form. Every node is laid out as a box: a width,
// an ascent above its baseline and a descent below it, holding the items that draw it (text,
// rules, strokes, stretched glyphs), placed relative to the box's left end and baseline. A
// layout element places its children's boxes in its own: a row side by side on one baseline, a
// fraction's parts above and below the math axis (the height of a minus sign, where fraction
// bars go), a script raised or lowered next to its base.
//
// The style of a node is what the elements around it make of it: display style (a formula on
// a line of its own, with larger operators and limits under and over them) or text style, and
// the script level, each level drawn smaller (scriptScale). Operators get their spacing, and
// whether they stretch, from a small operator dictionary.
//
// The package knows nothing of EPUB or of the page: Fonts measures and draws text, and the
// caller gives the size of the text around the formula. A formula travels in a document as the
// path of an image (Path, which is Prefix and its markup), since the document is built before
// the text's size is known; the typesetter lays it out at that size when it places it.
package mathml
