# EPUB 3.3 conformance

Measured with the W3C's reading-system tests ([w3c/epub-tests](https://github.com/w3c/epub-tests),
commit 54092b4, 2026-09-29). Each test is a small EPUB that says what passes ("you see X"). The 27
tests marked deprecated are left out, which leaves 179.

How they were run: a throwaway program zipped each test, opened it with `epub.Open`, built its
`epub.BuildIndex`, laid out its first chapters with `layout.Paginate` and drew their first pages
with the reader's settings (`openbook.TextSettings` in `internal/app/openbook`: 1404×1872, the default text
size); a fixed-layout book's pages went through `Book.RenderPage`, as at download time. Visible-text tests were
checked on the laid-out words, the others on the PNGs, each against the same run on the previous
version, so that a test that passed and changed was looked at again. A test that does not apply to
an offline e-ink reader without scripts, audio or a browser counts as out of scope.

Two judgement calls:
- Metadata tests (title and creator order, whitespace) are judged on `epub.Book`: the app's lists
  show the server's metadata, which Calibre-Web reads from the same package document.
- The ICB tests of fixed layout are judged on the page size from the viewport and the clipping.
  Their red boxes are drawn now (background colors), their grids are not: a grid is several
  gradients tiled by `background-size`, and such a pattern keeps only its color.

Nothing in a book is executed or fetched: no scripts (a `scripted` item shows its manifest
fallback), no WebAssembly, no remote resource (an `http:` image is a missing file). The parsers
added for this work (CSS colors and gradients, the SVG cascade, MathML, JPEG XL through
`pageimage.Decode`) have limits on their input and fuzz tests.

## Results

| Area | Tests | Pass | Fail | Out of scope | Main gaps |
|---|---|---|---|---|---|
| OCF (container, URLs, ZIP) | 14 | 11 | 0 | 3 | The 3 others need scripts to parse URLs. |
| Package metadata | 12 | 11 | 1 | 0 | An image missing from the manifest is still shown (kept: real books forget them). |
| Internationalization | 9 | 2 | 0 | 7 | Titles shown are the server's. Text in the book is laid out bidirectionally now. |
| Spine | 10 | 10 | 0 | 0 | |
| Manifest fallbacks | 5 | 5 | 0 | 0 | |
| Core media types | 10 | 6 | 1 | 3 | No AVIF decoder (its fallback is used); audio is out of scope. |
| XHTML and resources | 12 | 9 | 1 | 2 | Malformed XML is read leniently, not refused (kept: real books have broken markup); no browser for web links. |
| Fonts (`@font-face`, obfuscation) | 6 | 6 | 0 | 0 | |
| CSS (`-epub-` properties) | 10 | 5 | 5 | 0 | Vertical writing (`writing-mode`, `text-orientation`, `text-combine-horizontal`, the vertical half of `text-underline-position`); sesame emphasis marks, which no font has, drawn as dots. |
| SVG | 5 | 5 | 0 | 0 | Stroke widths are not scaled with the image (oksvg draws them in pixels); a pattern on a stroke or on text keeps one color. |
| MathML | 1 | 1 | 0 | 0 | |
| Navigation | 9 | 9 | 0 | 0 | |
| Structural semantics (footnotes) | 2 | 2 | 0 | 0 | |
| Fixed layout | 29 | 24 | 0 | 5 | No synthetic spreads on a portrait panel; one test is EPUBCheck's. |
| Reflowable layout (`rendition:flow`, roll) | 8 | 1 | 4 | 3 | `rendition:layout` roll (EPUB 3.4) is shown a page per image; scrolled flows and `align-x-center` don't apply to paged e-ink. |
| Scripting | 16 | 1 | 0 | 15 | No scripts; a scripted document's manifest fallback is shown. |
| Media overlays | 21 | 1 | 0 | 20 | No audio; overlays are ignored, as required. |
| **Total** | **179** | **109** | **12** | **58** | |

Of the 121 tests in scope, 109 pass (90%). The run before this one gave 101 (and 93 before
its fixes).

## Done since the previous run

- **MathML** (`internal/web/mathml`): presentation MathML laid out with the page, at the text's
  size, on its own baseline: fractions, scripts, roots, limits, stretched fences, tables,
  `mfenced`, `menclose`… Past its limits, `altimg`, then `alttext`, then the tokens
  (`cnt-mathml-support`).
- **SVG**: the SVG's `<style>` and the chapter's CSS applied before rasterizing; `<text>` drawn in
  the reader's fonts; colors oksvg refused (`rgba()`, `currentColor`) no longer fail the whole
  SVG; pattern fills tiled (`cnt-svg-css`, `cnt-svg-css-inclusion`).
- **Fixed layout**: background colors and simple linear gradients drawn under the boxes, a
  transparent background image over them.
- **CSS Text**: `-epub-`/`-webkit-` prefixes read, `text-align-last`, `text-underline-position:
  under`, `text-emphasis`, `word-break`, `line-break`, `text-transform: full-width`
  (`css-epub-text-align-last`, `-word-break`, `-line-break`, `-text-transform`).
- **CJK line breaking**: UAX #14 (`rivo/uniseg`) between ideographs, with kinsoku.
- **Bidi and shaping**: `dir`, `direction`, `unicode-bidi`, `<bdi>`, `<bdo>`; lines reordered
  (rule L2) and right-to-left paragraphs aligned from the right; Arabic and other joining scripts
  shaped (`go-text/typesetting`, a pure-Go HarfBuzz port).
- **JPEG XL** (`gen2brain/jxl`, pure Go): decoded with a size cap and its panics recovered
  (`pub-cmt-jxl`).

## Remaining work, by value for reading books on this reader

1. **CJK and Arabic fonts on the reader**: the layout is done, but whether the firmware's fonts
   have the glyphs is unchecked. DejaVu Sans, a fallback, has Arabic and Hebrew; CJK may need a
   font added.
2. **Vertical writing** (`writing-mode: vertical-rl`, `text-orientation`, `text-combine-upright`):
   a separate, larger job (weeks): columns, rotated glyphs, pages turning the other way.
3. SVG stroke widths in user units: oksvg draws `stroke-width` in output pixels, so a line is too
   thick in a large viewBox and too thin in a small one.
4. Repeated gradients and tiled `background-size` in fixed layout (the ICB tests' grids).
5. Sesame emphasis marks need a font that has U+FE45/U+FE46.
6. AVIF: no decoder that runs neither cgo nor WebAssembly; well-made books give a fallback.
7. On the reader: the time of a shaped page, of a page of formulas, and of a JPEG XL decode
   (about 270 ms on the Mac in pure Go for a full page, several seconds expected on the ARM).

## A real library

The 60 EPUBs of a Calibre library (French and English novels, EPUB 3 after conversion) were read
as the app does, before and after this work: every chapter laid out and every page drawn, every
link followed. 58 read and 2 are refused for their LCP DRM, as they should be. Both runs agree
on every book: 37,279 pages, the same characters, notes and links, no failed image, no missing
glyph, no text left out (the pages cover each chapter but its hidden footnotes), and 244 sample
pages identical to the pixel. The only defects are four links, in three books, to a cover page
the conversion left out of the spine.
