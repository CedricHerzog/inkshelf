// Package container opens an EPUB book (versions 2 and 3) and finds what is in it. Package
// epub then turns the chapters it finds into documents.
//
// An EPUB is a zip archive laid out by the Open Container Format (OCF):
//
//   - META-INF/container.xml names the package document (the OPF file), the book's
//     description. A book may name several, for other renditions: the first one is the book.
//   - META-INF/encryption.xml, if present, lists encrypted files. Books protected by DRM
//     encrypt their content and are refused with ErrDRM. Books that only obfuscate their fonts
//     (to keep them from being copied out) are fine: the fonts are de-obfuscated with the
//     book's identifier as the key.
//
// The package document has three parts that matter here:
//
//   - The metadata: title, language, creators, identifiers, and rendition:layout, which says
//     whether the book is reflowable text or pre-paginated pages.
//   - The manifest: every file of the book, by id, with its media type. An item the app cannot
//     show (a scripted page, an AVIF image) may name a fallback item to show instead.
//   - The spine: the reading order, as references to manifest items. The linear items are the
//     chapters, read one after the other; the others (notes, answers) are shown only when a
//     link leads to them.
//
// The navigation document (EPUB 3), or the NCX (EPUB 2), holds the table of contents, the
// pages of the printed book and the landmarks (the cover, the start of the text...), as links
// into the spine's documents.
//
// Paths in this package are paths in the zip: links in the book's files are resolved against
// the file they are in.
package container
