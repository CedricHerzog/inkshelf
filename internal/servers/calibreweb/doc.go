// Package calibreweb reads a Calibre-Web server, a server of ebooks, through its OPDS catalog.
//
// OPDS is a set of Atom feeds: a feed lists entries (books, or links to other feeds), a page
// at a time, each page linking to the next. Every request logs in with HTTP Basic auth (the
// user's username and password). Calibre-Web also has a JSON API under /ajax, but it needs a
// login session, so the app only uses OPDS.
//
// The catalog is read whole: every book from /opds/new, every series from the series index,
// then each series' feed for its books in order. A book's Calibre id is found in its download
// and cover links, and its place in its series in the text of its entry ("SERIES: Eragon
// [2.5]"). Books in no series are gathered into a series of their own. The app reads EPUB and
// CBZ files, preferring the EPUB when a book has both, and skips the other formats. OPDS has
// no reading progress: the reader keeps it alone.
//
// Client reads the feeds and downloads the files; Server turns the catalog into a
// servers.Server.
package calibreweb
