// Package comicarchive reads the pages of a comic book archive. A CBZ is a zip of page images,
// one file per page; the pages are in the natural order of their file names, so that page 10
// comes after page 9 however the archive lists them. A CBR is the same in a RAR archive, which
// the standard library cannot read: it is recognized and refused with ErrRAR.
package comicarchive
