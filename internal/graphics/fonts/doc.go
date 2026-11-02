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
// and Measure tell): DejaVu Sans, the firmware's font with the most characters, its Arabic
// font, then a CJK font, then Go's. Arabic letters take their form from their neighbours, by
// rules in the font; ShapesScriptOf tells whether a font has them, so that a font with the
// letters but not the rules (the firmware's Liberation Sans) gives way to one with both.
//
// Chinese, Japanese and Korean take Noto Serif CJK or Noto Sans CJK: the firmware's, the Japanese
// font of its collections (.ttc files), or else the program's own, compressed with Brotli
// (cjk.go). A font of the program's is decompressed the first time a character needs it, into the
// folder SetCacheDir gives, where later starts find it. They have one weight: a bold CJK font is
// the regular emboldened, its glyphs widened as they are rasterized. Every font file is parsed
// once, whatever the sizes it is opened at.
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
//
// # Vertical text
//
// Chinese and Japanese set in vertical lines go down a column. Their ideographs stay upright:
// ShapeVertical shapes them top to bottom, which gives brackets and the long vowel mark their
// vertical forms, and DrawGlyphsVertical draws them centred on the column. Latin words lie on
// their side: DrawTurned and DrawGlyphsTurned draw a horizontal line turned a quarter turn
// clockwise, from glyphs turned once and cached.
package fonts
