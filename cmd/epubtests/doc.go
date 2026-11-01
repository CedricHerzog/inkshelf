// Command epubtests reads the W3C's EPUB 3 reading-system tests as the reader would, to measure
// how much of EPUB 3.3 Inkshelf shows (docs/reference/epub-conformance.md).
//
// Each test of w3c/epub-tests is a small EPUB, unzipped in a folder, whose text says what a
// reader that passes shows: "you see a green square", "the second paragraph is in italics".
// Whether a test passes is judged by looking, so this program does not judge: it lays out and
// draws each test as the app does and writes what it got, for a person to look at.
//
// For each test it zips the folder into an EPUB and opens it. A fixed-layout book has its
// pages drawn as at download time; a reflowable one is laid out and drawn with the reader's
// settings (1404x1872, the default text size). It writes, in -out/<test>/, words.txt (the text
// of each page, line by line, as laid out) and page-N.png (the first pages).
//
// Two things save looking at every test on every run. With -previous, the output of the last
// run, it lists the tests whose words or pages changed: only those need looking at again. And
// with -judgments, the verdicts written down so far (test, area, verdict, reason, tab-separated),
// it prints the table of results by area that epub-conformance.md shows, and lists the tests
// that have no verdict yet.
//
//	go run ./cmd/epubtests -tests ~/src/epub-tests -previous build/epub-tests-previous
//
// make epub-tests does that, keeping the previous run's output.
//
// Nothing a test holds is executed or fetched, as in the app: the program only opens EPUBs.
package main
