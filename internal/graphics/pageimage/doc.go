// Package pageimage prepares page images for the e-ink panel, once, when a book is
// downloaded, so that turning a page on the reader only has to read a file.
//
// The panel shows 16 grays, and e-ink renders mid grays lighter than a computer screen does.
// A comic page scanned in color, at any size, must therefore be turned into the panel's own
// terms: the screen's size, its grays and its tone. That is slow on the reader's processor,
// so it is done while downloading, not while reading.
//
// Prepare runs every stage in turn:
//
//  1. DecodeGray decodes a JPEG, PNG, WebP or JPEG XL image straight to gray, what is
//     transparent composited over white (decode.go).
//  2. fit scales it down to the screen with a box filter and counts its grays (fit.go).
//  3. toneTable builds a tone table that stretches the grays to full black and white and
//     darkens the mid grays for e-ink (tone.go).
//  4. ditherAndPack maps the grays through that table to the panel's 16 grays with
//     Floyd–Steinberg dithering and packs them into a .pg4 page (dither.go, pg4.go).
//
// PrepareDecoded runs stages 2 to 4 on an image already decoded, and Fit stage 2 alone.
//
// Unpack reads a .pg4 page back for drawing.
package pageimage
