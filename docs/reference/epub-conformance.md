# EPUB 3.3 conformance

Measured with the W3C's reading-system tests ([w3c/epub-tests](https://github.com/w3c/epub-tests),
commit 54092b4, 2026-09-29), last run on 2026-11-02. Each test is a small EPUB that says what passes
("you see X"). The 27 tests marked deprecated are left out, which leaves 179.

How they were run: a throwaway program zipped each test, opened it with `epub.Open`, built its
`epub.BuildIndex`, laid out its first chapters with `layout.Paginate` and drew their first pages
with the reader's settings (`openbook.TextSettings` in `internal/app/openbook`: 1404×1872, the default text
size); a fixed-layout book's pages went through `Book.RenderPage`, as at download time, and
its spreads through `Book.RenderSpread`. Visible-text tests were
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
added for this work (CSS colors and gradients, the SVG cascade, MathML, JPEG XL and AVIF
through `pageimage.Decode`) have limits on their input and fuzz tests.

## Results

| Area | Tests | Pass | Fail | Out of scope | Main gaps |
|---|---|---|---|---|---|
| OCF (container, URLs, ZIP) | 14 | 11 | 0 | 3 | The 3 others need scripts to parse URLs. |
| Package metadata | 12 | 11 | 1 | 0 | An image missing from the manifest is still shown (kept: real books forget them). |
| Internationalization | 9 | 2 | 0 | 7 | Titles shown are the server's. Text in the book is laid out bidirectionally now. |
| Spine | 10 | 10 | 0 | 0 | |
| Manifest fallbacks | 5 | 5 | 0 | 0 | |
| Core media types | 10 | 7 | 0 | 3 | Audio is out of scope. |
| XHTML and resources | 12 | 9 | 1 | 2 | Malformed XML is read leniently, not refused (kept: real books have broken markup); no browser for web links. |
| Fonts (`@font-face`, obfuscation) | 6 | 6 | 0 | 0 | |
| CSS (`-epub-` properties) | 10 | 10 | 0 | 0 | |
| SVG | 5 | 5 | 0 | 0 | A pattern on a stroke or on text keeps one color. |
| MathML | 1 | 1 | 0 | 0 | |
| Navigation | 9 | 9 | 0 | 0 | |
| Structural semantics (footnotes) | 2 | 2 | 0 | 0 | |
| Fixed layout | 29 | 27 | 0 | 2 | A page of flowing text is not set in a spread (only image pages of a reflowable book pair); one test is EPUBCheck's. |
| Reflowable layout (`rendition:flow`, roll) | 8 | 5 | 0 | 3 | Scrolled flows are shown paged, and `align-x-center` doesn't apply to paged e-ink. |
| Scripting | 16 | 1 | 0 | 15 | No scripts; a scripted document's manifest fallback is shown. |
| Media overlays | 21 | 1 | 0 | 20 | No audio; overlays are ignored, as required. |
| **Total** | **179** | **122** | **2** | **55** | |

Of the 124 tests in scope, 122 pass (98%). The runs before this one gave 118, 113, 109 and 101.
The verdict of each test, and why, is in `cmd/epubtests/judgments.tsv`. The two that fail are
the deliberate departures.

Of the 55 out of scope, 43 need scripts, the network, a browser or audio. The other 12 are: 7
titles and creators that the app's lists show from the server, 3 scrolled flows and
`align-x-center`, which a paged e-ink screen has no use for, a test of EPUBCheck, and a page of
flowing text set in a spread, which this reader leaves alone (only image pages of a reflowable
book pair).

## Done since the previous run

- **Spreads** (`epub.Book.Spreads`, the device's landscape): two pages of a fixed-layout book
  that EPUB 3.3 pairs show side by side in landscape, drawn at download as one image; a centred
  page, a page meant for portrait, and the first page stand alone (`lay-pp-page-spread-left`,
  `-right`, `-combined`, which were out of scope on a portrait panel). Two image pages of a
  reflowable book pinned left and right read as one spread too.
- **AVIF** (`pageimage.decodeAVIF`): decoded by `gen2brain/gav1d`, a pure-Go port of dav1d,
  behind a pixel cap and a frame size limit, its panics on the calling goroutine recovered
  (`pub-cmt-avif`). AVIF is a core media type now: the fallback chain stops at it.
- **Vertical writing** (`layout`, `vertical.go`): `writing-mode` `vertical-rl` and `vertical-lr`
  laid out in columns with the horizontal line breaker, turned onto the page; ideographs upright
  and shaped top to bottom (vertical forms of brackets and the long vowel), Latin turned, as
  `text-orientation` and Unicode's `Vertical_Orientation` say; `text-combine-upright` (and its
  old names) across one cell; emphasis marks and underlines to the right, or to the left
  (`css-epub-writing-mode`, `-text-orientation`, `-text-combine-horizontal`,
  `-text-underline-position`, the vertical half of `-text-emphasis`). A block in the other
  direction (a horizontal table) is laid out apart, as one box. A vertical-rl book without a
  page progression turns its pages from right to left.
- **CJK fonts**: Noto Serif and Sans CJK JP ship in the program, compressed, as the firmware has
  no font for Chinese, Japanese or Korean.
- **`html`'s style**: what a stylesheet of the head sets on `html` (its writing mode, its
  underline position) now reaches the body, as in a browser.
- **Backgrounds** (`web/css`, `layout.PaintBackground`): the `background` shorthand expanded,
  several layers, `background-size`, `-position` and `-repeat`, and gradients linear or radial,
  repeating or not, with stops in pixels. One painter for flowing and fixed-layout pages, so the
  ICB tests' grids are drawn; `html`'s background is placed once its head's stylesheets are read.
- **Emphasis marks** as shapes: dots, circles, double circles, triangles and sesames, filled or
  open, whatever the fonts have (`css-epub-text-emphasis`).
- **SVG strokes** as thick as the drawing says: widths, dashes and dash offsets scaled from user
  units to pixels (`cnt-svg-css`). Joins stay bevelled when a drawing gives none, as oksvg
  draws them: SVG's initial join is a miter, but rasterx's miters break some corners.
- **Rolls** (`rendition:layout` roll): drawn at download as one strip of pictures as wide as the
  screen, cut into pages (`lay-roll-*`). Scrolled flows are shown paged.

## Remaining work, by value for reading books on this reader

1. To re-time on the reader: a page's 10 seconds now count from the start of its layout, and
   stylesheets are bounded (a quarter megabyte each, a megabyte per book, 10,000 rules). Before,
   a hostile SVG page appeared after 11.45 s (1.42 s of layout, then its 10 s), and a stylesheet
   of 20,000 rules took 6.56 s to lay out, 212 MB allocated; that chapter now builds in 22 ms
   with 28 MB on the host, against 65 ms. Measured on the reader on 2 November 2026,
   otherwise: the firmware's CJK fonts read in 0.08 to 0.1 s, so a vertical book opens in 0.9 s;
   vertical page turns 0.11 to 0.94 s; horizontal ones 111 ms, as before this work (112 ms); a
   page of formulas 177 ms; a full-page AVIF 5.75 s and a full-page JPEG XL 6.97 s, both under
   the limit.
2. AVIF's decoder runs an image's tiles on goroutines of its own, where a panic cannot be
   recovered: an option of gav1d to decode on the caller would close that.
3. Vertical text keeps the horizontal meaning of CSS margins (`margin-top` is the space before
   a block), and lays a formula out across.

## Whole books

A test of the suite checks one feature on a few pages. `go run ./cmd/epubtests -whole -books
a.epub:b.epub` reads real books whole instead, every chapter laid out and every page drawn with
footnotes kept in the text, and reports per book: the characters drawn as the missing-glyph box
(`layout.Faces.MissingCharacters`), the runes that no page shows, and which way the pages turn
against the spine's `page-progression-direction` and the language. The books stay where they
are; nothing is committed. `TestReadWholeBooks` runs it on a vertical Japanese and an Arabic book
made in code: no missing glyph, no rune left out, pages turning right to left.

Two real novels were read this way on 2 November 2026 (SC-003): Natsume Sōseki's *Kusamakura*
in vertical Japanese (the IDPF's `kusamakura-japanese-vertical-writing.epub`, 15 chapters, 334
pages) and Gibran's *The Broken Wings* in Arabic (Wikisource's EPUB 3 export of «الأجنحة
المتكسرة», 14 chapters, 87 pages). Neither draws a missing glyph or leaves a rune off its pages,
and both turn right to left: Sōseki from the spine, Gibran, whose spine gives no direction, from
the language. Read on the reader, Gibran's letters first had their dots beside their place: the
book sets its text in sans-serif, and the firmware's Liberation Sans has Arabic letters but no
rules to join them. Arabic now goes to the firmware's Noto Sans Arabic UI, which has both. The
lists showed the titles of both books wrong, Gibran's letters apart and in logical order, Sōseki's
as boxes; the interface now sets such a title as a line of a book (`layout.Label`).

## A real library

The EPUBs of a Calibre library (French and English novels, EPUB 3 after conversion; 60 at the
previous run, 61 now) were read as the app does on 2 November 2026, with the build before this
work and the build after it: through `reading.TextBook` at the default text size, every page
turned to and drawn, every link on it followed. 59 read and 2 are refused for their LCP DRM, as
they should be. Both builds agree on every book in pages (40,927, each with the same first and
last rune), characters, notes (288) and links (3,429, each to the same place or the same popup).

Drawn, 40,920 pages are identical to the pixel. The other 7, in four books, show a character
that the build before drew as the missing-glyph box: an en space (U+2002) or an em space
(U+2003) after a note's number or before a list's dash, and a row of black diamonds (U+25C6) as
a section break. They are drawn now; in one note the en space, narrower than the box, lets a
word up onto the line before, within the same page. The only other defects, in both builds, are links in four
books to a cover or title page that the conversion left out of the spine.
