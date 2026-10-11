// Package css reads the stylesheets of EPUBs and works out which declarations apply to each
// element.
//
// A stylesheet is a list of rules: a selector (p.note, h1 + p, nav a...) and the declarations
// it gives the elements it matches (font-size: 1.2em). Several rules may give an element the
// same property; the cascade decides which wins: an !important declaration over a normal one,
// then the more specific selector (an id over a class over an element name), then the later
// rule. Parse reads a stylesheet as CSS Syntax 3 does: what cannot be parsed is skipped, and
// nothing fails, since books are full of CSS meant for other readers. It keeps the rules of
// the @media blocks that match the reader's Media, and lists the @import and @font-face
// rules for the caller to follow.
//
// A Cascade gathers stylesheets in order, the user agent's first, and Compute returns the
// declared values of an element: for each property, the value that wins. The element is given
// through the Element interface, which only looks at the element, its parent and its previous
// siblings. That lets a builder that reads XHTML as a stream, with a stack of the elements open,
// compute each element's style as soon as it opens. Selectors that need the elements after one
// (:last-child, :empty) are dropped for that reason.
//
// Inheritance is left to the caller, which follows only some properties: a computed value is
// the caller's business. The package also reads the values the callers need: lengths, font
// families, URLs, colors (as grays, for the e-ink panel) and linear gradients.
package css
