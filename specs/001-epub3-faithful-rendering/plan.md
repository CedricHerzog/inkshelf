# Implementation Plan: EPUB 3 books shown as their publisher made them

**Branch**: `001-epub3-faithful-rendering` | **Date**: 2026-11-01 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/001-epub3-faithful-rendering/spec.md`

## Summary

Close the remaining gaps between Inkshelf and EPUB 3.3, without running or fetching anything a
book carries. Today 109 of 121 in-scope W3C reading-system tests pass; the gaps are vertical
writing, emphasis-mark shapes, SVG stroke widths, tiled and repeating backgrounds, AVIF, roll
layout, and two-page spreads, plus CJK glyphs on the reader and a bound on slow elements.

The approach, from [research.md](research.md):
- **Spreads**: parse every rendition property. At download, also draw each pair of fixed-layout
  pages as one landscape image (`spread-NNNN.pg4`, listed in `book.json`). Pair image-only
  reflowable items in the index. The reader screen switches the device to landscape for a
  spread, through a new orientation in `device/pocketbook`.
- **Vertical writing**: lay out in logical coordinates with the existing line breaker. Turn the
  lines into columns at placement, and shape upright runs top to bottom with `vert` (go-text,
  already a dependency).
- **CJK fonts**: ship Noto Serif CJK and Noto Sans CJK, Brotli-compressed in the binary,
  decompressed once into a cache folder.
- **AVIF**: decode with the pure-Go `gen2brain/gav1d`, behind the same guards as JPEG XL.
- **Slow elements**: long-lived workers (a reading pool and a download pool) bound each page to
  10 seconds. Elements not ready by then show their alt text.
- **Backgrounds and SVG**: one background-layer painter shared by reflowable and fixed layout;
  SVG strokes scaled into pixels.

## Technical Context

**Language/Version**: Go 1.27.2, `CGO_ENABLED=0`, `GOOS=linux GOARCH=arm GOARM=7` for the reader

**Primary Dependencies**: existing: go-text/typesetting (shaping, vertical included),
oksvg/rasterx, gen2brain/jxl, x/image, tdewolff/font. New: `gen2brain/gav1d` (AVIF, pure Go,
BSD-2-Clause, pinned commit). `andybalholm/brotli` moves from indirect to direct. Noto Serif
CJK JP and Noto Sans CJK JP, Regular (SIL OFL 1.1), embedded.

**Storage**: files on the reader. New: `spread-NNNN.pg4`, `book.json` `spreads`,
`index.json` v8 `spreads`, and `inkshelf/fonts/` (a cache). See
[contracts/on-disk.md](contracts/on-disk.md).

**Testing**: `go test` (unit, golden, fuzz), `make shots-all` and `shots-compare` (emulator),
`make bench` with benchstat. New `cmd/epubtests` runs the W3C suite. Reader scripts run over
remote control.

**Target Platform**: PocketBook InkPad 3 (PB740, B288 dual Cortex-A7, 512 MB, 1404x1872 16-gray
e-ink), firmware 6.x; the emulator on the host.

**Project Type**: embedded e-reader application (one static binary).

**Performance Goals**:
- A horizontal page turn takes no longer than before, by benchstat and on the reader (SC-005).
- A vertical or shaped page appears in under 2 s on the reader.
- The first CJK page pays a one-time font decompression of about 1 s.

**Constraints**:
- No code from a book runs, and nothing is fetched (FR-011–013).
- A page waits at most 10 s in all; elements not ready by then show their fallback (FR-014a).
- AVIF falls back to the book's manifest fallback if `gav1d` cannot run on the reader (FR-008).
- On-disk formats only grow (principle III).
- Hot paths keep their speed and inlining (principle IV).
- The binary grows by about 16 MB for the fonts.
- Memory stays within the reader's 512 MB with a 24.6 MB serif font parsed once.

**Scale/Scope**: about 200 W3C tests and the 60-book library of the previous run. The work
touches 6 domains (device, graphics, web, typesetting, ebook, shelf) plus app/openbook and the
reader screen. Vertical writing is the largest piece (weeks).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
|---|---|---|
| I. Pure Go, static | `gav1d` and `brotli` are pure Go. A first task builds `gav1d` with `CGO_ENABLED=0 GOARCH=arm GOARM=7` and checks `go.mod` for cgo. Fonts are data. | Pass (to confirm in that task) |
| II. Domain layering | Each new piece sits in an existing domain: orientation in device; CJK fonts and turned glyphs in graphics/fonts; backgrounds and gradients in web/css then typesetting; spreads in ebook/epub/container, ebook/epub and shelf. The one new package, `internal/deadline`, is a utility beside `timing`, and `cmd/epubtests` is a program. `app` and `shelf` import no server. The device knows nothing of spreads. | Pass |
| III. Files stay readable | Only new optional JSON fields and new files. `IndexVersion` goes 7 → 8 because the index gains `spreads`; rune counts are unchanged. The existing golden files pass unchanged, and new golden cases are added. | Pass |
| IV. Hot paths | `copyRotated` is untouched; landscape adds `copyStraight` with a benchmark. Glyph drawing in `fonts/draw.go` gets a separate path for turned glyphs; `make bench-baseline`/`bench-compare` and the `-gcflags=-m` inlining list are compared before and after. `fit`, `ditherAndPack`, `rgbaToGray` and `Unpack` are untouched. New costly code (vertical layout, background painting, spread composition, AVIF, font decompression) gets benchmarks and `timing.Start` spans. | Pass |
| V. One thread draws | See the goroutine and lock list below. | Pass |
| VI. Done means verified | `tasks.md` carries a script step and a shot for each UI change, `make verify`, and reader checks for what the emulator cannot show. | Pass (enforced in tasks) |
| VII. Docs follow code | See the docs list below. | Pass |

### Goroutines and locks (principle V)

- **Orientation switch**: on the main thread, in the reader screen's `DrawAndRefreshLocked`, with
  the state's mutex held, as today's drawing. No new lock.
- **Spread drawing at download**: on the existing downloader and preparer goroutines; no state
  lock while drawing; `book.json` is saved as today.
- **Reflowable image spreads**: drawn where text pages are, on the main thread or the preloader,
  under the renderer lock.
- **Element workers**: two goroutines started at `app.Init` with `state.GoLoop`. They never
  touch the state or draw to the screen; they compute and hand back a result.
  `layout.Page.Draw` waits on them while holding the renderer lock, which they never take. The
  lock order is unchanged.
- **CJK font decompression**: inside a `sync.Once` on the goroutine that first needs the font
  (main thread or preloader); it takes no app lock.

### Docs that change (principle VII)

- **Reference**:
  - [epub-conformance.md](../../docs/reference/epub-conformance.md): new results.
  - [epub-to-page.md](../../docs/reference/epub-to-page.md): rendition, spreads, vertical
    styles, backgrounds.
  - [pagination.md](../../docs/reference/pagination.md): vertical columns, marks, the deadline at
    draw.
  - [document-model.md](../../docs/reference/document-model.md): new style fields, layers,
    `Alt`.
  - [image-pipeline.md](../../docs/reference/image-pipeline.md): AVIF, spread images.
  - [on-disk-formats.md](../../docs/reference/on-disk-formats.md): `spreads`, `spread-NNNN.pg4`,
    index v8, `fonts/`.
  - [eink-and-firmware.md](../../docs/reference/eink-and-firmware.md): landscape.
  - [app-threading.md](../../docs/reference/app-threading.md): element workers.
  - [architecture.md](../../docs/reference/architecture.md) and CLAUDE.md's domain map:
    `deadline`, `cmd/epubtests`.
  - [emulator.md](../../docs/reference/emulator.md): landscape shots.
  - [usage.md](../../docs/reference/usage.md): spreads, vertical books, the 10-second rule.
- **Learn**:
  - `docs/learn/books/lines.mdx` and `pages.mdx`: vertical lines.
  - `fonts.mdx`: the CJK fallback.
  - `css.mdx`: writing modes and backgrounds.
  - `docs/learn/screen/eink.mdx`: orientation.

## Project Structure

### Documentation (this feature)

```text
specs/001-epub3-faithful-rendering/
├── spec.md
├── plan.md              # this file
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   ├── on-disk.md
│   ├── device-orientation.md
│   └── element-deadline.md
├── checklists/requirements.md
└── tasks.md             # /speckit-tasks, not created here
```

### Source Code (repository root)

```text
internal/
  device/pocketbook/          orientation, landscape canvas, copyStraight (+bench), touch mapping; emulator panel
  graphics/fonts/             embedded Brotli CJK fonts + license, cache, parse once per file, turned-glyph drawing,
                              vertical shaping (go-text TTB), synthesized bold
  graphics/pageimage/         AVIF in Decode (gav1d), pixel cap, panic recovery
  web/css/                    repeating/radial gradients, background shorthand and longhands, layers
  web/svgimage/               stroke width, dashes in pixels (serialize.go, rasterize.go)
  typesetting/document/       WritingMode, TextOrientation, TextCombineUpright, EmphasisPosition, BackgroundLayer, Image.Alt
  typesetting/layout/         vertical columns, orthogonal blocks, Vertical_Orientation table (go generate),
                              emphasis shapes, background painter, deadline at Draw
  ebook/epub/container/       Rendition (all properties), image/avif as a core type
  ebook/epub/                 styles above, Spreads (fixed and reflowable), IndexVersion 8, alt and alttext, roll
  ebook/fixedlayout/          background layers through layout's painter, per-element deadline
  shelf/                      spread-NNNN.pg4 at download, Book.Spreads, golden cases
  reading/                    reflowable image spreads in page turns
  deadline/                   new utility: worker pool with a per-job wait limit
  app/openbook/               spreads in TurnPage and positions, workers in TextSettings
  app/screens/reader/         landscape for spreads, menus and gestures in both orientations
  app/                        workers started in Init; portrait on leaving the reader
cmd/epubtests/                new: runs the W3C suite into build/epub-tests/
cmd/emulator/                 landscape in shots and the web view
emulator/scripts/             calibreweb-vertical.txt (new), fxl, showcase, mathml extended; reader/spreads.txt
internal/servers/calibreweb/fakecalibreweb/   fake books: vertical, spreads, AVIF, tiled backgrounds
```

**Structure Decision**: the existing single-module layout. Each change goes in the domain that
owns the concept (see the Constitution Check, II). There is one new utility package
(`internal/deadline`) and one new program (`cmd/epubtests`).

## Phasing (for /speckit-tasks)

The phases follow the spec's priorities. Each phase ends with `make verify` and its shots.

1. **Foundations**:
   - Parse rendition properties.
   - Set up `cmd/epubtests` and record today's results as the baseline.
   - Add `internal/deadline`.
   - Spikes, each a go/no-go:
     - Noto CJK through `sfnt` and go-text.
     - `gav1d` on ARMv7.
     - Landscape direction on the reader.
2. **US5, security**: no fetch, scripted fallbacks, and the 10-second limit with `Alt`. Mostly
   in place today; this phase locks it in with tests.
3. **US2, writing systems**: CJK fonts, then vertical layout and the page direction for vertical
   books.
4. **US1, styled books**: emphasis shapes, backgrounds, SVG strokes, roll.
5. **US3, spreads**: device orientation, fixed-layout spreads at download, reflowable image
   spreads, then the reader screen.
6. **US4, media**: AVIF.
7. **US6, media out of scope**: posters and fallbacks checked.
8. **Close**: run the conformance suite and the 60-book library, update the docs, run the
   reader checks.

## Complexity Tracking

No principle is broken; nothing to justify.
