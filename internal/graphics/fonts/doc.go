// Package fonts measures and draws text in gray, for the app's screens and for book pages.
//
// # Font files and fonts
//
// A TrueType or OpenType file describes each character's glyph as an outline, curves that can
// be drawn at any size, together with its advance (how far the pen, or "dot", moves after it)
// and kerning (adjustments between pairs, such as "AV"). This package keeps two things apart:
//
//   - A File is a font file parsed once: say a font embedded in a book. Parse makes one.
//   - A Font is a File at one size in pixels, ready to measure and draw. File.Open makes one.
//
// The package function Open opens one of the reader's own fonts by family and style: it looks
// for the firmware's Liberation and DejaVu files in /ebrmain/fonts (or a directory given to
// AddDir, off the reader), and falls back to the Go fonts compiled into the program.
//
// Fonts are hinted fully: their outlines are snapped to the pixel grid, so advances are whole
// pixels and every glyph of a character looks the same wherever it is drawn. A Font therefore
// rasterizes each glyph once, into an alpha mask it caches, and later draws only blend that
// mask into the page. Text is placed by the top-left corner of its line, the baseline the
// font's ascent below it, as libinkview placed it.
//
// # Fallbacks
//
// No font has every character. A book in French may quote Greek, or a title may hold an
// arrow. OpenFallbacks opens the fonts to try for the characters a font lacks (Has, Covers
// and Measure tell): DejaVu Sans, the firmware's font with the most characters, then Go's.
//
// # Shaping
//
// Drawing one glyph per character is enough for Latin text, but not for scripts whose letters
// change shape with their neighbours (Arabic joins its letters), stack marks (Hebrew points),
// or reorder (Indic scripts). Shaping turns a run of characters into the right glyphs at the
// right places, following rules in the font's own tables. Font.Shape does it with go-text's
// pure-Go port of HarfBuzz, the shaping engine of Chrome and Firefox, and Font.DrawGlyphs draws the
// result. Shaping costs more than looking up characters, so callers shape only the text that
// needs it.
package fonts
