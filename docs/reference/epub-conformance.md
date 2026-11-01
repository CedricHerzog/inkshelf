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

That program is now [cmd/epubtests](../../cmd/epubtests): `make epub-tests
EPUB_TESTS=<checkout>` draws every test into `build/epub-tests/`, lists the tests whose output
changed since the previous run, and prints this page's table from the verdicts kept in
`cmd/epubtests/judgments.tsv`. Judging stays a person's job: the program only shows.

Two judgement calls:
- Metadata tests (title and creator order, whitespace) are judged on `epub.Book`: the app's lists
  show the server's metadata, which Calibre-Web reads from the same package document.
- The ICB tests of fixed layout are judged on the page size from the viewport and the clipping,
  and on their grid: two gradients tiled by `background-size`, drawn as a grid.
- Tests 8 to 11 of `css-epub-text-emphasis` ask for hollow marks that their CSS does not give
  (`filled double-circle`, `filled triangle`): the marks the CSS gives count.

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
| CSS (`-epub-` properties) | 10 | 5 | 5 | 0 | Vertical writing (`writing-mode`, `text-orientation`, `text-combine-horizontal`, the vertical halves of `text-underline-position` and `text-emphasis`). |
| SVG | 5 | 5 | 0 | 0 | A pattern on a stroke or on text keeps one color. |
| MathML | 1 | 1 | 0 | 0 | |
| Navigation | 9 | 9 | 0 | 0 | |
| Structural semantics (footnotes) | 2 | 2 | 0 | 0 | |
| Fixed layout | 29 | 24 | 0 | 5 | No synthetic spreads on a portrait panel; one test is EPUBCheck's. |
| Reflowable layout (`rendition:flow`, roll) | 8 | 5 | 0 | 3 | Scrolled flows are shown paged, and `align-x-center` doesn't apply to paged e-ink. |
| Scripting | 16 | 1 | 0 | 15 | No scripts; a scripted document's manifest fallback is shown. |
| Media overlays | 21 | 1 | 0 | 20 | No audio; overlays are ignored, as required. |
| **Total** | **179** | **113** | **8** | **58** | |

Of the 121 tests in scope, 113 pass (93%). The run before this one gave 109, and 101 before
that. The verdict of each test, and why, is in `cmd/epubtests/judgments.tsv`.

## Done since the previous run

- **Backgrounds** (`web/css`, `layout.PaintBackground`): the `background` shorthand expanded,
  several layers, `background-size`, `-position` and `-repeat`, and gradients linear or radial,
  repeating or not, with stops in pixels. One painter for flowing and fixed-layout pages, so the
  ICB tests' grids are drawn; `html`'s background is placed once its head's stylesheets are read.
- **Emphasis marks** as shapes: dots, circles, double circles, triangles and sesames, filled or
  open, whatever the fonts have (`css-epub-text-emphasis`, but for its vertical text).
- **SVG strokes** as thick as the drawing says: widths, dashes and dash offsets scaled from user
  units to pixels (`cnt-svg-css`). Joins stay bevelled when a drawing gives none, as oksvg
  draws them: SVG's initial join is a miter, but rasterx's miters break some corners.
- **Rolls** (`rendition:layout` roll): drawn at download as one strip of pictures as wide as the
  screen, cut into pages (`lay-roll-*`). Scrolled flows are shown paged.

## Remaining work, by value for reading books on this reader

1. **CJK and Arabic fonts on the reader**: the layout is done, but whether the firmware's fonts
   have the glyphs is unchecked. DejaVu Sans, a fallback, has Arabic and Hebrew; CJK may need a
   font added.
2. **Vertical writing** (`writing-mode: vertical-rl`, `text-orientation`, `text-combine-upright`):
   a separate, larger job (weeks): columns, rotated glyphs, pages turning the other way.
3. AVIF: `gen2brain/gav1d`, a pure-Go port of dav1d, builds for the reader; not yet wired in.
4. On the reader: the time of a shaped page, of a page of formulas, and of a JPEG XL decode
   (about 270 ms on the Mac in pure Go for a full page, several seconds expected on the ARM).

## A real library

The 60 EPUBs of a Calibre library (French and English novels, EPUB 3 after conversion) were read
as the app does, before and after this work: every chapter laid out and every page drawn, every
link followed. 58 read and 2 are refused for their LCP DRM, as they should be. Both runs agree
on every book: 37,279 pages, the same characters, notes and links, no failed image, no missing
glyph, no text left out (the pages cover each chapter but its hidden footnotes), and 244 sample
pages identical to the pixel. The only defects are four links, in three books, to a cover page
the conversion left out of the spine.
