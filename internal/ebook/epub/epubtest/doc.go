// Package epubtest makes EPUB books for the tests and benchmarks of the packages that read
// them: package epub and its container, the shelf that downloads them, the reading engine.
//
// An EPUB is a zip whose first file, stored uncompressed, is "mimetype"; then
// META-INF/container.xml names the package document (the OPF), which lists the book's files
// (its manifest) and its reading order (its spine). Zip and Write turn a map of paths to
// contents into such a zip, so that a test can write exactly the files it is about, broken
// ones too.
//
// Files writes a whole book instead, of the size Options asks for: chapters of made-up but
// plausible prose (paragraphs of varied words, italics, a footnote, links between chapters),
// a stylesheet, a cover, a raster illustration, an SVG figure and a MathML formula, an EPUB 3
// navigation document and an EPUB 2 NCX. A benchmark that opens, indexes, lays out or
// downloads it measures the work a real novel costs. With Options.FixedLayout, each chapter is
// a pre-paginated page instead: a picture over the whole page and text placed on it, as in a
// comic or a children's book.
//
// The books are deterministic: the same Options always give the same files, so that two runs
// of a benchmark measure the same work.
//
// The package imports only the standard library, so that any package of the ebook domain or
// above can use it.
package epubtest
