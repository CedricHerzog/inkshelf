// Package shelf keeps the books downloaded on the reader: it downloads them from a
// servers.Server, prepares them for the e-ink screen once and for all, and remembers how far
// each has been read. It also keeps the covers shown in the lists.
//
// # What lives on the reader
//
// A Store keeps each book in a folder of its own, named after the book's id:
//
//	book.json     the Book: its titles, page count, format and reading progress
//	0001.pg4 …    a book of pages: each page prepared for the screen (see pageimage)
//	book.epub     a Reflowable book: its EPUB, laid out while it is read,
//	index.json    with its chapters and table of contents (epub.Index)
//	cover.pg4     and its cover, prepared for the screen
//
// book.json is always written last. A folder without it is a download that was interrupted
// (the reader went to sleep, Wi-Fi dropped), and the next Download picks it up where it
// stopped, skipping the pages already on disk. Every file is written to a temporary file then
// renamed, so that a crash or a full disk never leaves half a file behind.
//
// # Downloading a book of pages
//
// Page images are decoded, fitted, toned and dithered at download time, not at read time,
// since a page turn on the reader must be fast. Download runs two groups of workers joined by
// a small queue: fetchers ask the server for pages (only a few at once, as more only slows the
// server down), and preparers, one per core, turn each into a .pg4 file. A CBZ is fetched whole
// first, and its pages are then taken out of it by the same workers.
//
// # Downloading an EPUB
//
// An EPUB is fetched whole, to a ".part" file renamed once complete. A watchdog gives the
// download up when nothing arrives for 30 seconds, since a dropped Wi-Fi connection otherwise
// hangs forever. The book is then read through once to build its index, from which its page
// count is estimated (CharsPerPage runes to a page: its real pages depend on the font size
// chosen while reading). A fixed-layout EPUB, whose pages are drawn like pictures, is instead
// drawn page by page into .pg4 files and kept as a book of pages.
//
// # Reading progress
//
// A Book remembers the last page read. Unsynced marks progress the server has not been told
// about yet (package progress sends it), and PendingRemoval a book deleted on the reader whose
// progress must still be sent before its folder goes.
package shelf
