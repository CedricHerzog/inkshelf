// Package uri reads the URIs found in a book's markup.
//
// A link in a book's file (an <img src>, a stylesheet's url(), an SVG's href) is relative to
// the file it is in: Resolve turns it into the path of the file it leads to in the book's
// archive. An image may also be embedded in the markup itself, as a data: URI (RFC 2397):
// IsData recognizes one and DecodeData returns its bytes.
package uri
