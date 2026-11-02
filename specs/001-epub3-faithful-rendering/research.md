# Research: EPUB 3 books shown as their publisher made them

Phase 0 of [plan.md](plan.md). Each section answers one unknown of the Technical Context with a
decision, why, and what was set aside. Facts about today's code carry their paths; facts from
outside carry their source.

## Where the 12 failing tests are

From [epub-conformance.md](../../docs/reference/epub-conformance.md): CSS 5 (vertical writing,
sesame marks), reflowable layout 4 (`rendition:layout` roll), core media types 1 (AVIF), package
metadata 1 and XHTML 1 (the two deliberate departures, which stay). The 5 fixed-layout tests out
of scope for "no synthetic spreads" come into scope with spreads, and the 7 internationalization
tests out of scope are judged again on `epub.Book`, as the spec's assumptions say. SC-001's
denominator therefore grows; the plan's last task re-runs the whole suite and rewrites the
results table.

## R1. Landscape on a panel mounted sideways

**Facts.** The framebuffer is natively landscape, 1872x1404; the app draws a portrait canvas
and `copyRotated` (`internal/device/pocketbook/framebuffer_copy.go:34`, HOT PATH) turns it a
quarter turn. Touch is mapped the opposite way in `touchPoint` (`touch.go:37`). The canvas is the
firmware's task shared memory, accepted only at portrait size (`firmware_linux_arm.go:102-124`).
Screens read `pocketbook.ScreenSize()` on each draw; `widgets.Px` scales from the shorter side
(`internal/app/widgets/scale.go:14-24`), so sizes do not change in landscape. No orientation
setting, sensor or ioctl is used anywhere.

**Decision.** The device package gets an orientation, portrait (today) or landscape. In
landscape the app draws into a second canvas owned by the app, 1872x1404, copied to the
framebuffer without rotation (a straight row copy, a new function with its own benchmark;
`copyRotated` is not touched), and touch is mapped without the quarter turn. The firmware's
shared canvas stays portrait: on a switch to the background in landscape, the task thumbnail
keeps the last portrait frame. Switching orientation is a full flash refresh. Which way the
reader must be held for landscape (a clockwise or anticlockwise turn) follows from the panel's
mounting and is fixed on the reader during implementation; the emulator shows the landscape
canvas as it is.

**Alternatives.** Setting `rotate` with `FBIOPUT_VSCREENINFO`: the firmware's own apps and the
B288 driver may not honor it, and it would change the hot path's assumptions for portrait too.
Drawing landscape into the portrait canvas rotated: doubles the work of every landscape frame
for nothing.

## R2. Spreads: what is paired, and where it is stored

**Facts.** A fixed-layout book is rendered at download, page by page, at the portrait screen
size, into `%04d.pg4` files; the EPUB is then deleted and the book saved as
`servers.ComicArchive` (`internal/shelf/prepare_fixed_layout.go:20-44`). `rendition:spread`,
`rendition:orientation`, `rendition:flow` and `page-spread-*` are not parsed
(`internal/ebook/epub/container/package_document.go:160-168`). The position is a page number.

**Decision.**
- The container parses all rendition properties, package-wide and per itemref (data model:
  `Rendition`).
- `epub` computes the pairs (`Spreads`) by EPUB 3.3's rules: a spread is two consecutive spine
  items in a book or item whose `rendition:spread` is not `none`; `page-spread-left` and
  `page-spread-right` pin a page's side; unmarked pages alternate, the first page alone on the
  side opposite to the binding (as in print); `page-spread-center`, a portrait
  `rendition:orientation`, or a reflowable item in a fixed-layout book breaks the alternation and
  stands alone.
- At download, each pair is also drawn once as one landscape image: each page fitted into half
  of 1872x1404 (936x1404) and placed against the gutter, saved as `spread-%04d.pg4` (the number of
  its first page in reading order). `book.json` lists the pairs' first pages in a new
  `spreads` field, omitted when empty, so old files read and write back byte for byte.
- A book downloaded before has no `spreads`: it reads in portrait as today (FR-015).
- The reading position stays a page number; on a spread it is the spread's first page, and
  moving forward goes past both. Progress sent to the server is that number, as today.

**Alternatives.** Composing the spread at read time from the two portrait `.pg4` files: they are
already fitted and dithered to 1404x1872, so halving them would dither twice and lose detail;
the EPUB is gone, so they cannot be re-rendered. Storing landscape pages only: single pages
must still fill a portrait screen.

## R3. Image spreads in reflowable books

**Decision.** `epub.BuildIndex` finds pairs of consecutive spine items that each hold a single
image and nothing else visible (an `image/*` item, or an XHTML wrapper whose document is one
image block), marked left and right as in R2. The index records them in a new `spreads` field;
`epub.IndexVersion` goes up one (8, or 9 after US2's bump for vertical books) so existing indexes are rebuilt (rune counts are
unchanged, so saved locations stay valid). At read time the two images are decoded from
`book.epub`, fitted into the halves and shown in landscape; the reading position is the first
item's chapter at offset 0.

**Alternatives.** Rendering them at download: the reflowable path keeps the EPUB and decodes
images at draw time already; one more decode per spread costs less than new files.

## R4. Vertical writing

**Facts.** Layout is horizontal throughout (`internal/typesetting/layout`: one cursor `y`,
widths from the measurer, left/right floats). `writing-mode`, `text-orientation` and
`text-combine-upright` are dropped by `ebook/epub/style.go`. go-text v0.3.5 shapes top to bottom
(`di.DirectionTTB`), reads `vmtx`/`vhea`/`VORG`, and applies `vert` for vertical directions; the
app never asks it to.

**Decision.** Lay out in logical coordinates and turn at placement:
- The style gets `WritingMode`, `TextOrientation`, `TextCombineUpright` (the `-epub-` and
  `-webkit-` forms read as today).
- For a vertical chapter the paginator runs the same line breaker with the inline size equal to
  the content height, and stacks lines as columns, right to left for `vertical-rl`, left to
  right for `vertical-lr`; a page ends when the columns fill the content width.
- Each glyph's orientation follows `text-orientation` and Unicode's Vertical_Orientation
  property (UAX #50): upright runs are shaped top to bottom with `vert`/`vrt2`; sideways runs
  are shaped horizontally and drawn turned a quarter turn clockwise. The Vertical_Orientation
  table is generated from the UCD file by `go generate` into a compact range table (data, not
  copied code).
- `text-combine-upright: all` (and `digits N`) draws the run horizontally, compressed into one
  em cell.
- Underline and emphasis marks go to the right (or the side `text-underline-position` and
  `text-emphasis-position` say).
- A block whose writing mode differs from its parent's (a table in a vertical book) is laid out
  in its own mode as one box, at a width that fits the page, and placed as one column.
- When the spine has no `page-progression-direction` and the first chapter's root is
  `vertical-rl`, pages turn right to left.
- Rune offsets do not change, so saved positions stay valid; no `IndexVersion` bump for this.
- Glyph drawing (`graphics/fonts/draw.go`, HOT PATH) gets a separate entry for turned glyphs;
  the horizontal path keeps its benchmark and inlining.

**Alternatives.** Laying out horizontally then rotating the whole page: gives sideways CJK, the
opposite of what the book asks. A separate vertical layout engine: duplicates line breaking,
justification, bidi and hyphenation.

## R5. Emphasis marks of every shape

**Decision.** Marks are drawn as shapes, not glyphs: dot, circle, double circle, triangle and
sesame, filled or open, at half the em size, centred over each character (to the right in
vertical text), skipping spaces and punctuation as today. A custom string mark keeps being
drawn as text. `document.Emphasis.Mark` stays a rune so documents and their tests stay the
same; the drawing maps the CSS keyword runes to shapes.

**Alternatives.** Shipping a font with U+FE45/U+FE46: the CJK fonts of R6 have them, but marks
must work whatever the book's font, and the shapes are a few lines of path drawing.

## R6. CJK fonts, compressed

**Facts.** No CJK font ships and the firmware likely has none (a 2018 report of Japanese file
names as boxes on the InkPad 3; to be confirmed by listing `/ebrmain/fonts` on the reader). Fonts
are read and parsed again for each size (`internal/graphics/fonts/open.go:156,175`). The app is
one static file, 16 MB, which `deploy.sh` copies alone.

Sizes (notofonts/noto-cjk): NotoSansCJKjp-Regular.otf 16.5 MB, NotoSerifCJKjp-Regular.otf
24.6 MB (full Chinese, Japanese and Korean coverage, Japanese glyph forms by default); the
Japan-only subsets (4.5 and 6.2 MB) lack Hangul and much Hanzi. SIL OFL 1.1: redistribution with
software is allowed with the copyright notice and the license.

**Decision.**
- Ship Noto Serif CJK JP and Noto Sans CJK JP, Regular, compressed with Brotli at the highest
  quality, embedded in the binary (`go:embed` in `graphics/fonts`), with their license. Brotli
  (`andybalholm/brotli`, pure Go, already in the module graph) beats zstd on ratio and its
  decompression speed only matters once.
- On the first glyph lookup that needs one, the font is decompressed into
  `/mnt/ext1/applications/inkshelf/fonts/<name>.otf` (written atomically, then reused), under a
  `sync.Once`, on whatever goroutine lays out; the cache folder is given by `cmd`, as `AddDir` is
  today. Expected cost on the B288: about a second, once per install (measured on the reader).
- A font file is parsed once and shared by all its sizes (a change also worth it for today's
  fonts).
- Fallback order: reader typeface → DejaVu Sans → Noto CJK (serif for a serif family, sans
  otherwise) → Go fonts. Bold is synthesized (emboldened) rather than shipping two more fonts.
- Risk to clear first: `x/image/font/sfnt` caps cmap segments at 20000; Noto CJK's format 12
  cmap may exceed it. A first task parses both fonts with `sfnt` and go-text; if `sfnt` refuses
  them, CJK glyphs are drawn from go-text's parsed font through the shaping path (which already
  draws Arabic).
- Binary size: about 16 MB more with Brotli (measured in the task; estimated from WOFF2 sizes of
  the same fonts, about 11.6 MB for the sans).

**Spike outcome (T005, notofonts/noto-cjk commit f8d1575, 2026-11-01).** `sfnt` accepts the
cmap: both fonts parse with `x/image/font/sfnt` (0.25 ms) and with go-text `font.ParseTTF`
(2.5 ms), and both find 永, 한, 中 and ﹅ and agree on their glyph indices; `sfnt` loads their CFF
outlines. Both fonts have the GSUB `vert` feature and a `vmtx` table. So CJK glyphs are drawn
by the usual `sfnt` path; the go-text fallback above is not needed. Brotli at quality 11
(`andybalholm/brotli`) only saves 30%, as CFF outlines are already compact: sans 16.5 → 11.5 MB,
serif 24.6 → 16.8 MB, so the binary grows by about 28 MB, not 16. Decompressing takes 0.2 and
0.3 s on the Mac (compressing, done once by `make cjk-fonts`, about a minute each).

**Implementation (T034–T038, 2026-11-01).** With a 16 MB window (`LGWin` 24) the compression
does better than the spike's: sans 16.5 → 11.2 MB, serif 24.6 → 16.4 MB, 27.5 MB for both (`make
cjk-fonts` compresses them in Go, as there is no `brotli` command here). The stripped app grows
from 16,187,552 to 43,712,672 bytes. Decompressing in the emulator (arm64 host) takes 0.2 s for
the sans and 0.3 s for the serif. Parsing each file once makes opening a second size 3 times
quicker on the host (4.8 → 1.6 µs for Go's fonts, which are compiled in; on the reader the read
of the file is saved too). Times on the reader (first CJK page, a vertical page turn) are still
to measure: the reader under remote control was not reachable from the build machine (T050).

**Alternatives.** Separate font files copied by `deploy.sh`: users install the app by copying
one file, and a missing font would fail silently. Japan-only subsets: fail FR-005 for Korean and
much Chinese. The variable fonts: larger, and lose `VORG`.

## R7. AVIF

**Facts.** `gen2brain/avif` runs libavif as WebAssembly (or through purego); not allowed.
`gen2brain/gav1d` (BSD-2-Clause) is a pure-Go port of dav1d with AVIF support (8/10/12-bit,
4:2:0 to 4:4:4, alpha, grain, grids, irot/imir), bit-exact against dav1d by its author; created
2026-08, no release yet, SIMD only on 64-bit (plain Go on ARMv7).

**Decision.** Use `gen2brain/gav1d` pinned to a commit, behind `pageimage.Decode` with the same
guards as JPEG XL: header check against a pixel cap, panics recovered, one thread. Accept
`image/avif` as a core media type in `container`. First task: build for `GOARCH=arm GOARM=7`,
fuzz it with the existing harness, and time a full-page AVIF on the reader. If it fails those,
AVIF stays on its manifest fallback, as FR-008 allows, and `pub-cmt-avif` is recorded as a gap.

**Spike outcome (T006, 2026-11-01).** `gen2brain/gav1d` v0.2.6 (commit c8ebd66, tagged since;
53k lines of Go, no dependencies, needs Go 1.26.4) builds with
`CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build ./...`. It grows the stripped app by
1.2 MB (16.06 → 17.30 MB). It decodes `pub-cmt-avif`'s image (600x400, 21 KB) in 5 ms on the
host (an Apple-silicon Mac, arm64), and a 1404x1872 AVIF made with its own encoder (quality
70, noise added so it stays 544 KB) in 0.11 s, the same with `-tags noasm` (0.11 s): the
assembly barely matters here. The same test built for ARMv7 ran under the container's qemu
emulation in 1.45 s per full page; that is not the B288's speed, which T091 measures on the
reader. Go: T085–T091 go ahead.

**Alternatives.** Writing an AV1 intra decoder: an estimated 15-25k lines. The `wasm2go`
transpile of libavif: machine-translated WebAssembly, large and slow, and against the rule's
spirit.

## R8. Per-element 10-second limit

**Facts.** Nothing in decoding, layout or drawing takes a context or can be stopped. Images of
reflowable pages are decoded at draw time in `layout.Page.Draw` (`internal/typesetting/layout/draw.go:30-44`),
under the renderer lock, on the main thread or the preloader. A formula is laid out again at
each draw (`layout/formulas.go:56`). `<img alt>` and MathML `alttext` are not kept after the
chapter build.

**Decision.**
- A small utility package, `internal/deadline`, runs a job on a worker and waits for it at most
  a given time. Its workers are long-lived goroutines started once by the main thread
  (`state.GoLoop` at `app.Init`) and handed down in `openbook.TextSettings`, so no goroutine is
  started outside the main thread (principle V).
- `layout.Page.Draw` prepares each image, SVG and formula through it within the page's
  10-second budget (FR-014a): each element waits min(10 s, what is left of the page's 10 s).
  Past it, it draws the element's alternative text in its box (or leaves it empty), and
  remembers the element as given up for the rest of the session so the next page turn does not
  wait again. A late result is dropped.
- Two pools: reading (2 workers) and download (1 worker), so a download never makes a page
  wait. If every worker of a pool is still busy with given-up jobs, a new element is given up
  at once instead of queuing behind them.
- `document.Image` gains `Alt` (from `alt`, or MathML `alttext`), not counted in runes: no
  `IndexVersion` bump.
- Fixed-layout pages at download are already on the downloader's goroutines and not on screen;
  the same 10-second page budget applies there, through the download pool.
- Caps (pixels, nodes, depth) stay the first defence; the limit catches what passes them.

**Alternatives.** Cooperative cancellation (a context checked inside decoders): third-party
decoders (JPEG XL, AVIF, WebP) take none. Running each element on a fresh goroutine: breaks
principle V and leaves no bound on abandoned work.

## R9. Backgrounds, gradients, SVG strokes, roll

**Backgrounds.** `web/css` parses `repeating-linear-gradient`, `radial-gradient`,
`repeating-radial-gradient`, the `background` shorthand into longhands, `background-repeat`,
`background-size` (lengths, percentages, `cover`, `contain`), `background-position` and several
layers. The layers are a type of the document model (`typesetting/document`), and one routine
in `typesetting/layout` paints them into a box: reflowable border boxes use it instead of a flat
gray, and fixed layout (`ebook/fixedlayout`, a higher domain) uses it instead of
`fillGradient` and the stretched image. Fuzz tests extend `FuzzParseColor` and `FuzzFillGradient`; caps
on layers (8) and tiles per box.

**SVG strokes.** `svgimage` passes the accumulated user-to-pixel scale (`matrix.scale()`, as text
already does, `internal/web/svgimage/text.go:170`) to `addStrokeShape`, which writes
`stroke-width`, the dash array and the dash offset in pixels; for a non-uniform scale, the
square root of the determinant. No change to oksvg.

**Roll.** `rendition:layout` roll (EPUB 3.4 draft, tested by the suite) is shown as one
continuous strip: the images, each fitted to the page width, stacked and cut into screen-high
pages. The four failing tests are read again one by one in the task to confirm what each asks.

**The four roll tests, read again (T025, 2026-11-01).** All four are Kate Greenaway's *A Apple
Pie* (Library of Congress scans, 1404 to 1674 pixels wide) with `rendition:layout` roll, and none
says in words what passes: the roll layout does (EPUB 3.4 draft), one strip of the pictures,
each as wide as the screen, with no gap between spine items. Before this work each showed one
picture per page, fitted inside the margins (1224x1007 at 90,80), white above and below:
- `lay-roll-embedded-images`: XHTML chapters, each a `<figure>` with one `<img>` (styled
  `max-width: 100%; max-height: 100%`), after a cover page that is an image too.
- `lay-roll-embedded-images-svg`: the same, its plates SVG documents in the spine that link the
  PNGs with `<image>`; the title page has `page-spread-center`, which a strip ignores. (Its
  description says "Pre-paginated", copied from the pre-paginated test; its package says roll.)
- `lay-roll-images-in-spine`: the PNGs themselves in the spine, with XHTML fallbacks, and
  `rendition:spread` none.
- `lay-roll-images-mixed`: XHTML chapters and spine PNGs in turn.

So a roll is drawn as a strip at download, as fixed-layout pages are (`epub/roll.go`): every
chapter's images, fitted to the screen's width, stacked, and cut into screen-high page images.
A roll whose chapters hold text is read as a reflowable book. No chapter's rune count changes.

**Scrolled flows.** `rendition:flow` scrolled values are read and shown paged with the same
content, which the spec counts as honoring them.
