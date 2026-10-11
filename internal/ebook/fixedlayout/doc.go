// Package fixedlayout draws the pages of fixed-layout EPUBs (rendition:layout
// pre-paginated): comics, children's books, magazines, where each spine item is one page
// designed at a fixed size and meant to be seen whole.
//
// The size of a page is its initial containing block, in CSS pixels, which the page's
// viewport <meta> gives (ParseViewport). Package epub reads the page's XHTML into a Page made
// of regions: the page's own flow of content, and one region for each element positioned
// with position: absolute or fixed, or that has a background. Render draws the regions in
// order, the page scaled to fit the screen: each region's background (a color, a linear
// gradient, an image), then its content: a lone image fitted in the region, or text laid out
// by the typesetter inside it.
//
// The package knows nothing of EPUB: it reads the book's files through Files.
package fixedlayout
