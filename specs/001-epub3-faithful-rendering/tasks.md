---

description: "Tasks for EPUB 3 books shown as their publisher made them"
---

# Tasks: EPUB 3 books shown as their publisher made them

**Input**: Design documents from `specs/001-epub3-faithful-rendering/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md),
[data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: included. The constitution (VI) and CLAUDE.md's definition of done require them:
- golden tests for on-disk formats;
- fuzz tests for parsers of book content;
- benchmarks for costly code and hot paths;
- a script step and a `shot` for each UI change;
- `make verify` before each commit.

**Organization**: by user story, in the spec's priority order (US1, US2 and US5 are P1; US3 and
US4 are P2; US6 is P3). Each story ends with `make verify` and its shots.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an unfinished task)
- **[Story]**: the user story of spec.md the task serves
- Paths are from the repository root. Follow CLAUDE.md's conventions throughout:
  - descriptive names;
  - comments that say why;
  - `timing.Start` spans around slow work;
  - benchmarks written as `for b.Loop()` with `b.ReportAllocs()`;
  - `Locked` suffixes;
  - one change per commit.

---

## Phase 1: Setup (shared infrastructure)

**Purpose**: baselines, a repeatable conformance run, and the go/no-go spikes the plan calls for.

- [X] T001 Record baselines before any change: run `make baseline` (build/baseline/shots) and `make bench-baseline` (build/bench), and save `GOOS=linux GOARCH=arm GOARM=7 go build -gcflags=-m ./internal/graphics/fonts ./internal/device/pocketbook 2>&1 | grep -i inlin | sort` to build/bench/inlining-before.txt
- [X] T002 Create the conformance runner `cmd/epubtests/main.go` (with `cmd/epubtests/doc.go`):
  - For each test folder of a local w3c/epub-tests checkout (flag `-tests`), zip it in memory and open it with `epub.Open`, then `epub.BuildIndex`.
  - For a reflowable book, lay out its first chapters with `layout.Paginate`, using the reader's settings (`openbook.TextSettings`: 1404x1872, default text size).
  - For a fixed-layout book, render its pages with `Book.RenderPage`.
  - Write `build/epub-tests/<test>/words.txt` and `page-N.png`.
  - Diff those against the previous run's folder (flag `-previous`) and list the tests whose output changed.
  - Read verdicts from `cmd/epubtests/judgments.tsv` (columns: test, area, verdict pass|fail|out-of-scope, reason).
  - Print the per-area table of docs/reference/epub-conformance.md.
- [X] T003 Add `epub-tests` to the Makefile (`make epub-tests EPUB_TESTS=~/src/epub-tests`): runs `go run ./cmd/epubtests -tests $(EPUB_TESTS) -previous build/epub-tests-previous`, after moving build/epub-tests to build/epub-tests-previous
- [X] T004 Run `make epub-tests` against w3c/epub-tests commit 54092b4. Fill `cmd/epubtests/judgments.tsv` with today's verdicts (109 pass, 12 fail, 58 out of scope, 27 deprecated left out), copying each reason from docs/reference/epub-conformance.md.
- [X] T005 [P] Spike, CJK fonts (research R6):
  - Download NotoSansCJKjp-Regular.otf and NotoSerifCJKjp-Regular.otf (notofonts/noto-cjk) into build/cjk/.
  - Write a test `internal/graphics/fonts/cjk_spike_test.go`, skipped unless `INKSHELF_CJK_DIR` is set. It parses both fonts with `golang.org/x/image/font/sfnt` and with go-text `font.ParseTTF`, and checks:
    - the glyphs of 永, 한, 中 and ﹅;
    - the `vert` feature;
    - `vmtx`.
  - Record the outcome in research.md R6 ("sfnt accepts / refuses the cmap"), then delete the spike test.
- [X] T006 [P] Spike, AVIF (research R7):
  - `go get github.com/gen2brain/gav1d@<latest commit>` in a scratch branch.
  - Check the build: `CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build ./...`.
  - Decode the AVIF of the w3c test `pub-cmt-avif` and a 1404x1872 AVIF on the host.
  - Record in research.md R7: builds yes/no, host decode time, binary growth.
  - If it fails, AVIF stays on its manifest fallback, as FR-008 allows: mark T085–T089 as dropped, and in T091 record `pub-cmt-avif` as a gap in `cmd/epubtests/judgments.tsv` and docs/reference/image-pipeline.md.

---

## Phase 2: Foundational (blocking prerequisites)

**Purpose**: the pieces several stories use: the deadline pool, the rendition properties, and the
domain map.

**⚠️ CRITICAL**: no user-story work starts before this phase is done.

- [X] T007 Create the utility package `internal/deadline`, following contracts/element-deadline.md:
  - `doc.go` explains why a wait is bounded and not the work.
  - `deadline.go` provides:
    - `NewPool(workers int)`;
    - `(*Pool).Loop` (the body of one worker);
    - `(*Pool).Run(key string, limit time.Duration, job func() any) (any, bool)`.
  - States of a job: queued → running → done or given up.
  - Rules:
    - "A key in GivenUp is not prepared again until the app restarts".
    - When every worker is busy with given-up work, `Run` returns false at once.
    - A nil pool runs the job inline with no limit.
    - `Run` never starts a goroutine.
    - Each job runs in a `timing.Start` span named by the caller.
    - A given-up job logs one line with its key and duration.
  - It imports nothing of Inkshelf's but `timing`.
- [X] T008 [P] Tests for T007 in `internal/deadline/deadline_test.go`:
  - a job past a short limit returns false and its late result is dropped;
  - a given-up key returns false immediately;
  - two workers busy with given-up jobs make a third `Run` return false at once;
  - a nil pool runs inline;
  - no goroutine leak (count before and after with workers stopped).
- [X] T009 Start two pools in `internal/app/app.go` `Init`, each worker started by `state.GoLoop`: a reading pool (`deadline.NewPool(2)`) for page drawing and a download pool (`deadline.NewPool(1)`) for download-time rendering, so a download never makes a page wait. Keep both in the State (`internal/app/state/state.go`) so `openbook` and `downloadqueue` can reach them.
- [X] T010 Parse every rendition property in `internal/ebook/epub/container/package_document.go` and `internal/ebook/epub/container/container.go`, as data-model.md's `Rendition` says:
  - Fields and values (default in parentheses):
    - Layout: reflowable, pre-paginated, roll (reflowable)
    - Flow: auto, paginated, scrolled-continuous, scrolled-doc (auto)
    - Spread: none, landscape, both, auto (auto). `portrait` is read as `both` (deprecated).
    - Orientation: auto, landscape, portrait (auto)
    - PageSpread, itemref only: none, left, right, center (none). `rendition:page-spread-center` is read too.
  - Read them both package-wide (`<meta property="rendition:*">`) and per itemref (`properties`); the itemref overrides the package.
  - "an unknown value is ignored (the default stays)".
  - Replace `SpineItem.Layout` and `Book.Layout` with `Rendition.Layout` everywhere they are used (`grep -rn '\.Layout\b' internal/ebook`).
- [X] T011 [P] Tests for T010 in `internal/ebook/epub/container/package_document_test.go`: each property package-wide, per itemref, the override, unknown values ignored, `portrait` spread read as `both`, and both spellings of center
- [X] T012 [P] Add `internal/deadline` and `cmd/epubtests` to the domain map in CLAUDE.md and to the table of docs/reference/architecture.md (a utility beside `timing`; a program beside `cmd/readerctl`)

**Checkpoint**: `make verify` passes and shots-compare lists nothing (no visible change yet).

---

## Phase 3: User Story 1 - A styled novel looks as it was designed (Priority: P1) 🎯 MVP

**Goal**:
- emphasis marks of every shape;
- repeating, radial and tiled backgrounds, in reflowable and fixed-layout pages;
- SVG strokes as thick as the drawing says;
- `rendition:flow` and roll honored on a paged screen.

**Independent Test**: `make shots SCRIPT=emulator/scripts/calibreweb-showcase.txt`, then check
in build/shots:
- the marks in their shapes;
- the tiled grid visible;
- the strokes of two viewBoxes equal relative to their drawing.

Then `make epub-tests`: the CSS sesame tests, the ICB grid tests and the roll tests pass.

### Tests for User Story 1

- [X] T013 [P] [US1] Parser tests in `internal/web/css/color_test.go` and `internal/web/css/declarations_test.go`:
  - `repeating-linear-gradient`, `radial-gradient` and `repeating-radial-gradient`, with at most 32 stops;
  - the `background` shorthand expanded into longhands;
  - `background-repeat`, `background-size` (`cover`, `contain`, lengths, percentages), `background-position`;
  - several layers.
  - Extend `FuzzParseColor` to cover the new gradient functions.
- [X] T014 [P] [US1] Stroke test in `internal/web/svgimage/rasterize_test.go`: the same path with `stroke-width="1"` in a 10x10 viewBox and with `stroke-width="100"` in a 1000x1000 viewBox, both rasterized to 200x200, give the same stroke thickness in pixels (±1). Cover dash arrays and dash offsets the same way.
- [X] T015 [P] [US1] Emphasis test in `internal/typesetting/layout/emphasis_marks_test.go`: each keyword mark (•, ◦, ◉, ○, ▲, △, ﹅, ﹆) draws ink above each emphasized character with a font that lacks the glyph, and skips spaces and punctuation

### Implementation for User Story 1

- [X] T016 [P] [US1] Parse repeating and radial gradients in `internal/web/css/color.go`:
  - Generalize `ParseLinearGradient` into a `ParseGradient` returning a `Gradient` with its kind (linear, radial), repeating, stops ("≤ 32"), and either an angle or a shape and position.
  - Keep `ParseLinearGradient`'s callers working.
- [X] T017 [US1] Expand the `background` shorthand in `internal/web/css/declarations.go`, and parse `background-repeat` (repeat, repeat-x, repeat-y, no-repeat, space, round), `background-size` (auto, cover, contain, or two lengths/percentages), `background-position` (two lengths/percentages) and comma-separated layers in `internal/web/css/values.go`
- [X] T018 [US1] Add `BackgroundLayer` to `internal/typesetting/document/borders.go`:
  - Fields: Image (none, an image resource, or a gradient), Gradient, Size, Position, Repeat, as data-model.md says.
  - Add `Background []BackgroundLayer`, "at most 8", to the border box style in place of the gray-only `Background`.
  - Keep the color as the bottom layer.
- [X] T019 [US1] Compute background layers in `internal/ebook/epub/build_borders.go`, replacing `backgroundGray`. Images are resolved as `build_images.go` resolves them; a remote URL is a missing image.
- [X] T020 [US1] Write the layer painter `internal/typesetting/layout/backgrounds.go`, which paints a box's layers in gray:
  - color;
  - linear and radial gradients, repeating or not;
  - images sized, positioned and tiled.
  - Rule: "tiles drawn per layer are capped (4096); past the cap the layer keeps its color only".
  - Wrap it in a `timing.Start` span.
  - Call it from `drawBorderBoxes` in `internal/typesetting/layout/border_boxes.go`.
- [X] T021 [P] [US1] Benchmark and fuzz the painter in `internal/typesetting/layout/backgrounds_test.go`: `BenchmarkPaintBackground` (a 1404x1872 box with a tiled repeating gradient, built in code) and `FuzzPaintBackground` (random layers and sizes; no panic, bounded time)
- [X] T022 [US1] Use the painter for fixed layout:
  - In `internal/ebook/fixedlayout/page.go` and `internal/ebook/fixedlayout/background.go`, replace `fillColor`/`fillGradient` and the stretched background image with `layout`'s painter.
  - In `internal/ebook/epub/prepaginated.go`, drop the `layered`/`tiled` color-only fallback in `backgroundFill` and build `[]BackgroundLayer` in `placeOnPage`.
  - Adapt `FuzzFillGradient` in `internal/ebook/fixedlayout/fixedlayout_test.go` to the painter.
  - Check `BenchmarkPageRender` with benchstat.
- [X] T023 [US1] Draw emphasis marks as shapes in a new `internal/typesetting/layout/emphasis_marks.go`:
  - Shapes: dot, circle, double circle, triangle, sesame; filled or open.
  - Size: half the em, centred over each character.
  - Map the keyword runes of `document.Emphasis.Mark` to them.
  - Remove the dot fallback in `internal/typesetting/layout/text_preparation.go` (lines ~94–108).
  - In `addMarks` in `internal/typesetting/layout/line_placement.go`, emit shapes for keyword marks and keep text for custom string marks.
- [X] T024 [US1] Scale SVG strokes:
  - In `internal/web/svgimage/rasterize.go`, pass the user-to-pixel matrix into `restyle`.
  - In `internal/web/svgimage/serialize.go`, have `write` hand the accumulated transform's scale (`matrix.scale()`, as `text.go` does) to `style()` and `addStrokeShape`.
  - Multiply `stroke-width`, each dash length and `stroke-dashoffset` by it; for a non-uniform scale, use the square root of the determinant's absolute value.
  - Check `BenchmarkRasterize` with benchstat.
- [X] T025 [US1] Read again the 4 failing reflowable-layout tests (`make epub-tests`, area "Reflowable layout") and write down in research.md R9 what each one shows
- [X] T026 [US1] Lay out roll books:
  - When `Rendition.Layout` is roll, show the spine's images as one strip, each fitted to the page width, stacked and cut into screen-high pages.
  - Code: `internal/ebook/epub/roll.go`, chapter building in `internal/ebook/epub/chapter.go`.
  - Treat scrolled `Rendition.Flow` values as paginated, with the same content.
  - Compare each fake chapter's rune count (`document.Chars`) before and after. If it changes, bump `epub.IndexVersion` in this commit, and add a case to `internal/reading/textbook_test.go` showing that a `Location` saved before the change still reaches the same text (constitution III).
- [X] T027 [P] [US1] Tests for roll and flow in `internal/ebook/epub/roll_test.go`: a roll of three images of different heights gives ceil(total/screen) pages with no image left out, and a `scrolled-doc` book paginates as `paginated`
- [X] T028 [US1] Extend the fake showcase book `internal/servers/calibreweb/fakecalibreweb/showcase.go`:
  - a chapter with sesame, triangle and double-circle marks;
  - a box with a tiled `repeating-linear-gradient` grid and a radial gradient;
  - the same SVG stroke in a 10x10 and a 1000x1000 viewBox.
  - Add steps and `shot`s for those pages to `emulator/scripts/calibreweb-showcase.txt`.
- [X] T029 [US1] Update the docs this story makes wrong (constitution VII), following docs/WRITING.md:
  - docs/reference/epub-to-page.md: backgrounds, `background-*`, gradients, roll, scrolled flows;
  - docs/reference/pagination.md: emphasis shapes;
  - docs/reference/document-model.md: `BackgroundLayer`;
  - docs/learn/books/css.mdx: backgrounds.
  - Run `make docs-build`.
- [X] T030 [US1] Run `make verify` and `make epub-tests`:
  - Open each changed shot.
  - Update `cmd/epubtests/judgments.tsv` for the tests that now pass (CSS sesame, the ICB grids, roll).
  - Update docs/reference/epub-conformance.md in the same change (constitution VII): the CSS, Fixed layout and Reflowable layout rows (counts and main gaps), the Total row and the "Of the N tests in scope, M pass" sentence; remove sesame marks, the ICB grids, SVG stroke widths and roll from "Remaining work".
  - Run `make baseline`.

**Checkpoint**: US1 works alone; the showcase shots show the publisher's design.

---

## Phase 4: User Story 2 - A book in any writing system reads correctly (Priority: P1)

**Goal**:
- vertical writing (`writing-mode`, `text-orientation`, `text-combine-upright`, marks to the side);
- page direction for vertical books;
- a glyph for every CJK, Arabic and Hebrew character, from two shipped CJK fonts.

**Independent Test**: `make shots SCRIPT=emulator/scripts/calibreweb-vertical.txt`:
- columns from the right;
- Latin words turned;
- "2026" in one cell;
- marks to the right;
- no missing-glyph box;
- a tap on the left moves forward.

Then `make epub-tests`: the vertical-writing tests pass.

### Tests for User Story 2

- [X] T031 [P] [US2] Font fallback tests in `internal/graphics/fonts/cjk_test.go`: with the cache in `t.TempDir()`:
  - 永, 한 and 中 resolve to the CJK sans for a sans family and to the CJK serif for a serif family;
  - the cache file is written once and reused;
  - a cache file of the wrong size is rewritten;
  - a second size reuses the parsed font (no new read).
- [X] T032 [P] [US2] Vertical layout tests in `internal/typesetting/layout/vertical_test.go`:
  - a `vertical-rl` paragraph lays out as columns from right to left that fill the content width before the page ends;
  - `vertical-lr` goes from left to right;
  - an upright CJK glyph is not turned and a Latin word is turned a quarter turn clockwise (`text-orientation: mixed`);
  - `upright` keeps Latin upright;
  - `text-combine-upright: all` puts "2026" in one em cell;
  - emphasis marks sit to the right;
  - a horizontal table inside a vertical chapter keeps its own direction as one box.
- [X] T033 [P] [US2] Position test in `internal/reading/textbook_test.go`: the same chapter in horizontal and in vertical writing has the same rune count, and `GoTo(Location)` reaches the same text, so saved positions stay valid (no `IndexVersion` bump for vertical writing)

### Implementation for User Story 2

- [X] T034 [US2] Add the compressed CJK fonts:
  - A Makefile target `cjk-fonts` downloads NotoSerifCJKjp-Regular.otf and NotoSansCJKjp-Regular.otf (notofonts/noto-cjk, pinned release).
  - It compresses each with `brotli -q 11` into `internal/graphics/fonts/cjk/NotoSerifCJKjp-Regular.otf.br` and `internal/graphics/fonts/cjk/NotoSansCJKjp-Regular.otf.br`, and records each decompressed size in `internal/graphics/fonts/cjk/sizes.txt`.
  - Commit them with `internal/graphics/fonts/cjk/OFL.txt` (SIL OFL 1.1 and the copyright notice).
- [X] T035 [US2] Embed and unpack the fonts in a new `internal/graphics/fonts/cjk.go`:
  - `go:embed` the `.br` files.
  - Add `SetCacheDir(dir string)`.
  - On the first lookup that needs a CJK font, under a `sync.Once` per font, decompress with `github.com/andybalholm/brotli` (moved from indirect to direct in go.mod) into `<cache>/fonts/<name>.otf`, written atomically. "A file whose size differs from the embedded font's recorded size is written again".
  - Wrap the work in `timing.Start("unpack font")`.
- [X] T036 [US2] Set the font cache: in `cmd/inkshelf/main.go` call `fonts.SetCacheDir("/mnt/ext1/applications/inkshelf")`, and in `cmd/emulator/main.go` call `fonts.SetCacheDir("build/emulator-cache")`
- [X] T037 [US2] Parse each font file once in `internal/graphics/fonts/open.go`: keep the parsed `sfnt.Font` (or the go-text face, per T005's outcome) per file and build each size's face from it, instead of `os.ReadFile` + `Parse` per size (open.go:156,175). Benchmark the opening of a second size in `internal/graphics/fonts/open_test.go`.
- [X] T038 [US2] Extend the fallback chain:
  - In `internal/graphics/fonts/open.go` (`fallbackFiles`) and `internal/typesetting/layout/faces.go` (`fallbacksOf`), use the order reader typeface → DejaVu Sans → Noto CJK (serif for a serif family, sans otherwise) → Go fonts.
  - Synthesize bold for the CJK fonts (emboldened outline) instead of shipping a bold weight.
- [X] T039 [P] [US2] Generate the Unicode Vertical_Orientation table:
  - The generator `internal/typesetting/layout/gen_vertical_orientation.go` (`//go:build ignore`) reads VerticalOrientation.txt (UCD, pinned version, downloaded into build/) and writes the compact range table `internal/typesetting/layout/vertical_orientation.go`.
  - Add the `//go:generate` line and a lookup function with its test.
- [X] T040 [US2] Add the vertical style fields to `internal/typesetting/document/document.go`:
  - WritingMode: horizontal-tb, vertical-rl, vertical-lr ("`sideways-*` read as their vertical forms"); default horizontal-tb.
  - TextOrientation: mixed, upright, sideways; default mixed.
  - TextCombineUpright: none, all, digits(2..4); default none.
  - EmphasisPosition: over/under, right/left; default "over right".
  - All inherited as CSS says.
- [X] T041 [US2] Compute those properties in `internal/ebook/epub/style.go`:
  - Add cases in the setter switches (style.go ~180–260) for `writing-mode`, `text-orientation`, `text-combine-upright` and `text-emphasis-position`, plus the vertical values of `text-underline-position`.
  - Read the `-epub-` and `-webkit-` forms through `unprefixed`.
- [X] T042 [US2] Shape and draw vertical text in `internal/graphics/fonts`:
  - Shaping, in `shape.go`: shape upright runs top to bottom with go-text `di.DirectionTTB`, so `vert`/`vrt2` and `vmtx` apply.
  - Drawing, in `draw.go`: add a separate entry for glyphs turned a quarter turn clockwise. Leave the horizontal HOT PATH unchanged.
  - Run `make bench-compare` and compare the `-gcflags=-m` inlining list with build/bench/inlining-before.txt; both must be unchanged.
- [X] T043 [US2] Paginate vertical chapters in a new `internal/typesetting/layout/vertical.go`, with hooks in `internal/typesetting/layout/paginate.go` and `internal/typesetting/layout/line_placement.go`:
  - Run the same line breaker with the inline size set to the content height.
  - Stack lines as columns, right to left for `vertical-rl` and left to right for `vertical-lr`; the page ends when the columns fill the content width.
  - Map logical to physical coordinates when placing words.
  - Choose each glyph's orientation from `TextOrientation` and the Vertical_Orientation table.
  - Draw `text-combine-upright` runs horizontally, compressed into one em cell.
  - Put underlines and emphasis marks on the side `EmphasisPosition` and `text-underline-position` say.
  - Lay out a block whose writing mode differs from its parent's in its own mode, as one box at a width that fits the page, placed as one column.
  - Wrap the work in a `timing.Start("paginate vertical")` span.
- [X] T044 [P] [US2] Benchmark vertical pages in `internal/typesetting/layout/paginate_test.go`: `BenchmarkRenderPageVertical` (a page of Japanese text built in code, next to `BenchmarkRenderPage`)
- [X] T045 [US2] Turn vertical books right to left: in `internal/ebook/epub/index.go` (`BuildIndex`), when the spine has no `page-progression-direction` and the first chapter's root style is `vertical-rl`, set `Index.PageProgression` to `rtl`. Test it in `internal/ebook/epub/index_test.go`. Bump `epub.IndexVersion` from 7 to 8 in this commit, so that indexes already on the reader are rebuilt with the new direction; move `internal/shelf/testdata/golden/index.json` to version 8 and keep the version-7 file as a test that it is rebuilt.
- [X] T046 [US2] Add a fake vertical book in a new `internal/servers/calibreweb/fakecalibreweb/vertical.go`, listed in `catalog.go`:
  - a Japanese `vertical-rl` chapter with ruby-free text, a Latin word, the number 2026 under `text-combine-upright: all`, sesame marks and a horizontal table;
  - an Arabic chapter.
- [X] T047 [US2] Add the script `emulator/scripts/calibreweb-vertical.txt`: open the vertical book, `shot` the first page, tap left, `shot`, open the Arabic chapter, `shot`. Add it with its flags to `shots-all` in the Makefile.
- [X] T048 [US2] Update the docs this story makes wrong (constitution VII):
  - docs/reference/pagination.md: vertical columns, orthogonal blocks;
  - docs/reference/document-model.md: the writing-mode fields;
  - docs/reference/epub-to-page.md: vertical styles, the page direction of vertical books;
  - docs/reference/on-disk-formats.md: the `fonts/` cache, index version 8;
  - docs/learn/books/lines.mdx, docs/learn/books/pages.mdx, docs/learn/books/fonts.mdx.
  - Run `make docs-build`.
- [X] T049 [US2] Run `make verify` and `make epub-tests`:
  - Open each changed shot.
  - Update `cmd/epubtests/judgments.tsv` for the vertical-writing tests and the re-judged internationalization tests.
  - Update docs/reference/epub-conformance.md in the same change (constitution VII): the CSS and Internationalization rows, the Total row and the "Of the N tests in scope, M pass" sentence; remove vertical writing and the CJK fonts from "Remaining work".
  - Run `make baseline`.
- [X] T050 [US2] Reader checks (needs emulator/reader.json):
  - `make reader-deploy`.
  - Open a vertical Japanese book and an Arabic book, and read their logs with `go run ./cmd/readerctl log`: the first CJK page (font unpacking, about 1 s once), a vertical page turn (under 2 s, SC-005), a horizontal page turn (as before).
  - List `/ebrmain/fonts` (or confirm from the log that no firmware CJK font was found).
  - Note the times in research.md R6.
  - Without remote control: `make deploy`, ask the user to open both books, then read `applications/inkshelf.log` over USB.

**Checkpoint**: US2 works alone; a vertical Japanese and an Arabic book read with no missing glyph.

---

## Phase 5: User Story 5 - Nothing in a book runs or calls out (Priority: P1)

**Goal**:
- no code runs and nothing is fetched (locked in by tests);
- every image, SVG and formula bounded to 10 seconds, with its alt text shown past it.

**Independent Test**: `go test ./internal/deadline && go test ./internal/typesetting/layout -run 'TestDraw(ShowsAlt|Budget|WithoutPool)'`,
the scripting tests of `make epub-tests`, and the fake chapter with a script and an `https:`
image in the showcase script.

### Tests for User Story 5

- [X] T051 [P] [US5] Security tests in `internal/ebook/epub/epub_test.go`:
  - a `scripted` spine item with a manifest fallback shows the fallback;
  - a `<script>` and an `onclick` leave the static text around them and add no text of their own;
  - an `https:` image is a missing resource and its `Alt` is kept;
  - an `https:` font in `@font-face` falls back to the next font;
  - an `<iframe>`, `<form>` and `<object type="application/wasm">` are not loaded.
- [X] T052 [P] [US5] Deadline-at-draw test in `internal/typesetting/layout/draw_test.go`:
  - with a pool whose limit is 50 ms and an image loader that sleeps 200 ms, `Page.Draw` returns after about 50 ms with the image's `Alt` drawn in its box;
  - a second `Draw` does not wait;
  - a formula that fails to lay out draws its alt text;
  - the limit is per page: with a page limit of 250 ms and three images that each take 200 ms, `Draw` returns after about 250 ms, with the first image drawn and the alt text of the other two.

### Implementation for User Story 5

- [X] T053 [US5] Add `Alt` to `document.Image` in `internal/typesetting/document/document.go` ("Not counted in runes"; no `IndexVersion` bump). Fill it:
  - from `<img alt>` in `internal/ebook/epub/build_images.go`;
  - from an inline SVG's `<title>` in `internal/ebook/epub/build_svg.go`;
  - from MathML `alttext` in `internal/ebook/epub/build_math.go`.
- [X] T054 [US5] Bound each element in `internal/typesetting/layout/draw.go` (the per-image loop, ~30–44) and `internal/typesetting/layout/formulas.go`:
  - Run each image load (through the `webimage.Loader`), SVG rasterization and formula layout through the `deadline` pool given in the layout options.
  - The limit is per page (FR-014a): start a 10-second page budget when `Draw` begins, and give each element min(10 s, what is left of the budget). Once the budget is spent, every element left shows its fallback without a wait.
  - Use the key (book, resource, size).
  - Past the limit, draw `Alt` in the element's box in the text's font, or leave it empty when there is none.
- [X] T055 [US5] Hand the pool down:
  - Add the pool to `openbook.TextSettings` in `internal/app/openbook/text.go` and to the layout options it builds.
  - For download-time pages, pass the download pool from `internal/app/downloadqueue/downloadqueue.go` to `shelf.Store.Download` and on to `ebook/fixedlayout` (`internal/ebook/fixedlayout/page.go`), so each fixed-layout page has the same 10-second page budget.
- [X] T056 [US5] Review the input caps of every parser of book content and fix any that has none:
  - CSS: `internal/web/css`.
  - SVG: `internal/web/svgimage/tree.go`.
  - MathML: `internal/web/mathml/node.go`.
  - Fonts: `internal/web/webfont`.
  - Images: `internal/graphics/pageimage/decode.go`; add a pixel cap like `maxJXLPixels` for JPEG, PNG and WebP, checked with `DecodeConfig`.
  - Add a hostile-input set under `internal/ebook/epub/testdata/hostile/` (a 100k-element SVG, a 60 Mpx PNG header, nested CSS, deep MathML) and a test that opens and draws each within the limit without a panic (SC-006).
- [X] T057 [US5] Add a "Safety" chapter to the fake showcase book in `internal/servers/calibreweb/fakecalibreweb/showcase.go`, with a `<script>`, an `onclick`, an `https:` image with `alt`, and text around them. Add its step and `shot` to `emulator/scripts/calibreweb-showcase.txt`.
- [X] T058 [US5] Show the fallback in a shot (constitution VI):
  - Add an emulator flag `-element-limit` (default 10s) in `cmd/emulator/main.go`, used as the reading pool's page budget.
  - In the Safety chapter of the fake showcase book, add an image the fake server serves slowly (`internal/servers/calibreweb/fakecalibreweb/server.go`).
  - Add a script `emulator/scripts/calibreweb-slow.txt`, run with `-element-limit 50ms` (its flags in the Makefile's `shots-all`), that opens that page and `shot`s the alt text in the image's box.
- [X] T059 [US5] Update the docs this story makes wrong (constitution VII):
  - docs/reference/app-threading.md: the two pools, the wait under the renderer lock;
  - docs/reference/pagination.md: the page's 10-second budget at draw;
  - docs/reference/document-model.md: `Image.Alt`;
  - docs/reference/emulator.md: `-element-limit`.
  - Run `make docs-build`.
- [X] T060 [US5] Run `make verify` and `make epub-tests`:
  - Confirm that every scripting test shows its fallback or its static text.
  - Confirm that no network request leaves the emulator but to the fake server (its log).
  - Update `cmd/epubtests/judgments.tsv`, and in the same change the Scripting row of docs/reference/epub-conformance.md if any reason changed (constitution VII), with the Total row and the "Of the N tests in scope, M pass" sentence.
  - Run `make baseline`.

**Checkpoint**: US5 works alone; a hostile book can neither run code, fetch, crash nor stall a page.

---

## Phase 6: User Story 3 - Fixed-layout books and spreads look like the print edition (Priority: P2)

**Goal**:
- synthetic spreads side by side in landscape, the app switching orientation by itself;
- image spreads of reflowable books in landscape;
- old books unchanged.

**Independent Test**: `make shots SCRIPT=emulator/scripts/calibreweb-fxl.txt`:
- the cover alone in portrait;
- each pair in a 1872x1404 shot;
- a centred page alone in portrait;
- the menu over a spread;
- `spread-*.pg4` and `spreads` in the downloaded book's folder.

A book folder saved before the change reads in portrait.

### Tests for User Story 3

- [X] T061 [P] [US3] Pairing tests in `internal/ebook/epub/spreads_test.go`, following data-model.md's Spread rules:
  - "pair only where the effective `Spread` is not `none`";
  - "a pinned side is honored";
  - "unmarked items alternate, the first one alone";
  - "`center`, portrait `Orientation`, or a reflowable item stands alone";
  - "two items pinned to the same side are not paired";
  - right-to-left books put `Left` second.
  - For reflowable books: "both items are image-only and pinned to opposite sides, the first on the side reading starts from; anything else is not a spread".
- [X] T062 [P] [US3] Golden tests in `internal/shelf/golden_test.go`:
  - a new `internal/shelf/testdata/golden/books/fxl-9/book.json` with `"spreads": [2, 4]` reads and saves byte for byte;
  - the existing komga-42 and epub-7 files pass unchanged.
  - Move `internal/shelf/testdata/golden/index.json` up one version (8 to 9 if US2 shipped first) with `spreads` (`{"first": n, "left": n, "right": n}`), and keep the previous version's file as a test that it is rebuilt.
- [X] T063 [P] [US3] Device tests in `internal/device/pocketbook/framebuffer_copy_test.go` and `internal/device/pocketbook/touch_test.go`:
  - `copyStraight` copies a 1872x1404 canvas row for row;
  - `BenchmarkCopyStraight`;
  - in landscape, `touchPoint` maps raw touches without the quarter turn, and `rawTouch` is its inverse;
  - portrait mappings are unchanged.

### Implementation for User Story 3

- [X] T064 [US3] Add orientation to `internal/device/pocketbook/canvas.go`, following contracts/device-orientation.md:
  - `Orientation` (Portrait, Landscape), `SetOrientation`, `Orientation()`.
  - In landscape, an app-owned 1872x1404 canvas; `ScreenSize()` and `Screen()` return the current one.
  - "Only the main thread changes it".
  - Setting the current orientation does nothing.
  - The next refresh is a flash refresh.
- [X] T065 [US3] Copy and touch in landscape:
  - `internal/device/pocketbook/framebuffer_copy.go`: `copyStraight`, a row copy with no rotation. Leave `copyRotated` untouched.
  - `internal/device/pocketbook/framebuffer_linux_arm.go` `flush`: pick the copy by orientation.
  - `internal/device/pocketbook/touch.go`: map touches by orientation.
  - `internal/device/pocketbook/event_loop.go`: use the current screen size instead of the one captured at start.
- [X] T066 [US3] Return to portrait before the app goes to the background, so the firmware's task thumbnail and the next app see a portrait screen. Code: `internal/device/pocketbook/system_events.go` / `firmware_linux_arm.go` (the firmware's shared canvas stays portrait).
- [X] T067 [US3] Emulator in landscape:
  - `internal/device/pocketbook/emulated_firmware.go`: panel and canvas follow the orientation.
  - `cmd/emulator/script.go` and `cmd/emulator/server.go`: shots and the web view take the current size.
  - `internal/device/pocketbook/script/run.go`: check script points against the current size.
- [X] T068 [US3] Find the landscape direction on the reader:
  - Make a temporary reader script `emulator/scripts/reader/landscape-probe.txt` that opens a spread in a book already on the reader.
  - Run `make reader-deploy` and `make reader-shots`, and look at the panel.
  - If the image is upside down for the intended grip, fix the direction in `copyStraight` and `touch.go`.
  - Write the direction and its reason in docs/reference/eink-and-firmware.md, then delete the probe.
- [X] T069 [US3] Compute spreads in a new `internal/ebook/epub/spreads.go`: `Spreads(book) []Spread`, with `Spread{First, Left, Right}` as spine indexes, for fixed-layout books per T061's rules.
- [X] T070 [US3] Draw spreads at download in `internal/shelf/prepare_fixed_layout.go`:
  - For each spread, render both pages with `Book.RenderPage` fitted into 936x1404 each and place them against the gutter on a 1872x1404 canvas (left page on the left).
  - Pack it with `pageimage.PrepareDecoded` into `spread-%04d.pg4`, numbered by the spread's first page in reading order.
  - Write the files before `book.json`, wrapped in a `timing.Start("render spread")` span.
  - Benchmark the composition in `internal/shelf/prepare_fixed_layout_test.go`.
- [X] T071 [US3] Record and load spreads:
  - Add `Spreads []int` with the JSON tag `spreads,omitempty` to `shelf.Book` in `internal/shelf/book.go`. Rule: "every number n has `spread-%04d.pg4` for n on disk and n+1 ≤ Pages; spreads do not overlap".
  - Add `Store.LoadSpread(book, first)` to `internal/shelf/store.go`.
  - Document the new fields and files in docs/reference/on-disk-formats.md.
- [X] T072 [US3] Find reflowable image spreads:
  - Bump `epub.IndexVersion` by one (8 to 9 if US2 shipped first) in `internal/ebook/epub/index.go` and add `Spreads` (JSON tag `spreads,omitempty`) to `Index`.
  - In `BuildIndex`, find pairs of consecutive spine items that each hold one image and nothing else visible (an `image/*` item, or an XHTML wrapper whose document is one `document.Image` block), pinned left and right per T061's rules.
- [X] T073 [US3] Read reflowable image spreads:
  - In `internal/reading/textbook.go` and `internal/reading/renderer.go`, a page turn that reaches the first item of an index spread yields one spread page: both images decoded from `book.epub`, fitted into 936x1404 halves of 1872x1404, under the renderer lock.
  - The position is the first item's chapter at offset 0.
  - Moving forward goes past both items.
- [X] T074 [US3] Make page turns spread-aware in `internal/app/openbook/openbook.go` (`TurnPage`) and `internal/app/openbook/pages.go` (`CurrentPageLocked`, preloader):
  - On a page book with `Spreads`, the page in a spread loads `LoadSpread`.
  - Forward and back move past both pages.
  - "a position on the second page (from the server, or from an older version) is shown as that spread".
  - The saved and synced `Page` is the spread's first page.
  - A book without `Spreads` behaves as today (FR-015).
- [X] T075 [US3] Switch orientation in the reader screen, `internal/app/screens/reader/reader.go` (`DrawAndRefreshLocked`, `centered`, `drawPage`, tap zones):
  - Call `pocketbook.SetOrientation(Landscape)` before drawing a spread and `Portrait` before a single page, on the main thread with the state's mutex held.
  - Centre the image in the current screen size.
  - Compute tap and swipe zones from the current size.
- [X] T076 [US3] Toolbar, menus, popups and the end panel in landscape: `internal/app/screens/reader/popup.go`, `internal/app/screens/reader/endpanel.go`, `internal/app/screens/reader/deletequestion.go`. Replace sizes that assume portrait height (e.g. `bottom-Px(250)`) with the current screen size, so gestures and menus work in both orientations.
- [X] T077 [US3] Set portrait before drawing any screen other than the reader, in `internal/app/app.go` (`Draw`), so the library, lists and settings always draw in portrait
- [X] T078 [US3] Spreads in the fake fixed-layout book, `internal/servers/calibreweb/fakecalibreweb/fixedlayout.go`:
  - `rendition:spread` auto;
  - pages marked `page-spread-left`/`right`;
  - one `page-spread-center`;
  - one item with `rendition:orientation` portrait;
  - a tiled-grid background (ICB grid).
- [X] T079 [US3] Add a reflowable book to `internal/servers/calibreweb/fakecalibreweb/epub.go` with two image-only spine items pinned left and right between text chapters
- [X] T080 [US3] Extend `emulator/scripts/calibreweb-fxl.txt`:
  - download the book;
  - `shot` the cover (portrait);
  - turn and `shot` each spread (1872x1404);
  - `shot` the centred page (portrait);
  - open the menu over a spread and `shot`;
  - turn back and `shot`;
  - open the reflowable book's image spread and `shot`.
- [X] T081 [US3] Check that old books are kept: in `internal/shelf/download_test.go`, a book folder written without `spreads` (copy of a baseline-format folder in `internal/shelf/testdata/`) opens, reads page by page in portrait, and is never redrawn
- [X] T082 [US3] Add the reader script `emulator/scripts/reader/spreads.txt` (reads only; changes no server): open a fixed-layout book with spreads already on the reader, turn through two spreads and a single page, open the menu on a spread. Then:
  - `make reader-shots READER_SCRIPT=emulator/scripts/reader/spreads.txt`;
  - look at the ghosting at each orientation switch;
  - press the home button from a spread and check the task thumbnail is portrait;
  - read the log for the switch's refresh time.
  - Without remote control: `make deploy`, ask the user to go through the same steps, then read `applications/inkshelf.log` over USB.
- [X] T083 [US3] Update the docs this story makes wrong (constitution VII; T068 and T071 already cover eink-and-firmware.md and on-disk-formats.md):
  - docs/reference/epub-to-page.md: rendition, spreads;
  - docs/reference/image-pipeline.md: spread images;
  - docs/reference/app-threading.md: the orientation switch;
  - docs/reference/emulator.md: landscape shots;
  - docs/reference/usage.md: spreads; drop "two-page spreads show one page at a time";
  - docs/learn/screen/eink.mdx: orientation.
  - Run `make docs-build`.
- [X] T084 [US3] Run `make verify`, `make bench-compare` (`copyRotated` unchanged; `BenchmarkCopyStraight` new) and `make epub-tests`:
  - Update `cmd/epubtests/judgments.tsv` for the five fixed-layout tests that come into scope.
  - Update docs/reference/epub-conformance.md in the same change (constitution VII): the Fixed layout row (the five spread tests in scope; remove "No synthetic spreads" from its main gaps), the Total row and the "Of the N tests in scope, M pass" sentence.
  - Open each changed shot.
  - Run `make baseline`.

**Checkpoint**: US3 works alone; spreads show in landscape, single pages in portrait, old books unchanged.

---

## Phase 7: User Story 4 - Every core image format and formula shows (Priority: P2)

**Goal**: AVIF decoded without its fallback; formulas unchanged.

**Independent Test**: `make shots SCRIPT=emulator/scripts/calibreweb-mathml.txt` shows an AVIF
image with no fallback; `make epub-tests` passes `pub-cmt-avif`.

If T006 recorded that `gav1d` does not build or decode on ARMv7, skip T085–T089: AVIF stays on its manifest fallback, as FR-008 allows, and T091 only records the gap.

- [X] T085 [P] [US4] AVIF tests in `internal/graphics/pageimage/avif_test.go`:
  - decode a small AVIF built in code or from `internal/graphics/pageimage/testdata/` (8-bit 4:2:0, 10-bit, with alpha);
  - `FuzzDecodeAVIF` with no panic escaping;
  - `BenchmarkDecodeAVIF` (a 1404x1872 image).
- [X] T086 [US4] Add `github.com/gen2brain/gav1d` to go.mod, pinned to the commit T006 checked; confirm `CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build ./...`
- [X] T087 [US4] Decode AVIF in `internal/graphics/pageimage/decode.go`, with JPEG XL's guards:
  - check the header with `DecodeConfig` against a pixel cap (`maxAVIFPixels`, the same 8.4 Mpx);
  - recover panics with `recoverError`;
  - decode with one thread;
  - wrap the work in a `timing.Start("decode avif")` span.
- [X] T088 [US4] Accept AVIF:
  - add `image/avif` to the core image types in `internal/ebook/epub/container/package_document.go` (`SupportedImage`), so the fallback chain stops at it;
  - make `internal/web/webimage/webimage.go` route it through `pageimage.Decode`.
- [X] T089 [US4] Add an AVIF image with no fallback to the fake MathML book (`internal/servers/calibreweb/fakecalibreweb/chapters.go` or `images.go`), with its step and `shot` in `emulator/scripts/calibreweb-mathml.txt`
- [X] T090 [US4] Update docs/reference/image-pipeline.md (constitution VII): AVIF, its pixel cap and its guards; run `make docs-build`
- [X] T091 [US4] Run `make verify` and `make epub-tests` (update judgments for `pub-cmt-avif`). On the reader, time a full-page AVIF in the log (`go run ./cmd/readerctl log`) and note it in research.md R7. If it passes 10 s, the page budget of T054 shows its fallback: record that. If AVIF falls back (T006 or this timing), record `pub-cmt-avif` as a gap in `cmd/epubtests/judgments.tsv` and docs/reference/image-pipeline.md, as FR-008 allows. Update docs/reference/epub-conformance.md in the same change (constitution VII): the Core media types row (AVIF passes, or is recorded as a gap under FR-008), the Total row and the "Of the N tests in scope, M pass" sentence.

**Checkpoint**: US4 works alone.

---

## Phase 8: User Story 6 - Audio, video and read-aloud leave the text readable (Priority: P3)

**Goal**: media overlays ignored; a video shows its poster, audio and video their fallback
content.

**Independent Test**: the media-overlay tests of `make epub-tests` read as text; the fake chapter
shows a poster and a fallback.

- [X] T092 [P] [US6] Media tests in `internal/ebook/epub/epub_test.go`:
  - a book with a media overlay (`media-overlay` on items, SMIL in the manifest) builds the same documents as without it;
  - a `<video poster>` becomes an image of its poster;
  - a `<video>` without a poster shows its fallback content;
  - an `<audio>` with fallback content shows that content.
- [X] T093 [US6] Make the media elements show their poster or fallback in `internal/ebook/epub/build_images.go` (and the element dispatch in `internal/ebook/epub/build.go`):
  - `<video poster>` → `document.Image` of the poster;
  - otherwise the element's fallback children;
  - `<audio>` → its fallback children;
  - media overlays ignored.
  - Compare each fake chapter's rune count (`document.Chars`) before and after. If it changes, bump `epub.IndexVersion` in this commit, and add a case to `internal/reading/textbook_test.go` showing that a `Location` saved before the change still reaches the same text (constitution III).
- [X] T094 [US6] Add a chapter with a video (poster), a video (no poster, fallback text) and an audio element (fallback text) to the fake showcase book, `internal/servers/calibreweb/fakecalibreweb/showcase.go`, with a step and `shot` in `emulator/scripts/calibreweb-showcase.txt`. Update docs/reference/epub-to-page.md (posters, media fallbacks, overlays ignored; constitution VII). Then run `make verify` and `make baseline`.

**Checkpoint**: all stories work.

---

## Phase 9: Polish and cross-cutting concerns

- [X] T095 Run the whole conformance suite (`make epub-tests`) and finish docs/reference/epub-conformance.md (each story already updated its rows):
  - check the results table's totals against the run;
  - write "Done since the previous run";
  - record the run's w3c/epub-tests commit and date.
  - Expected: every in-scope test passes but the two deliberate departures, and `pub-cmt-avif` if AVIF falls back (SC-001); every out-of-scope line names scripts, network, audio or video (SC-002).
- [X] T096 Read the 60-book Calibre library of the previous run as the app does (every chapter laid out, every page drawn, every link followed) with the build before and after this feature. Compare pages, characters, notes, links and 244 sample pages to the pixel; explain each difference as a fixed defect (SC-004). Record the result in docs/reference/epub-conformance.md ("A real library").
- [X] T097 Check SC-003 on two real books:
  - Add a `-whole` mode to `cmd/epubtests/main.go`: for each EPUB given by `-books` (paths outside the repository; nothing is committed), lay out every chapter and draw every page.
  - Report missing glyphs (a .notdef drawn), runes left out (each chapter's coverage by its pages), and the page order against the spine's direction.
  - Run it on a real vertical Japanese novel and a real Arabic novel. Expected: zero of each. Record the result in docs/reference/epub-conformance.md.
- [X] T098 [P] Check that the reference docs are complete and consistent across stories (each story updated its own pages; fill what is missing), following docs/WRITING.md:
  - docs/reference/epub-to-page.md: rendition, spreads, vertical styles, backgrounds.
  - docs/reference/pagination.md: vertical columns, mark shapes, the deadline at draw.
  - docs/reference/document-model.md: style fields, `BackgroundLayer`, `Image.Alt`.
  - docs/reference/image-pipeline.md: AVIF, spread images.
  - docs/reference/app-threading.md: element workers, the orientation switch.
  - docs/reference/emulator.md: landscape shots.
  - docs/reference/usage.md: spreads, vertical books, the 10-second rule; drop "two-page spreads show one page at a time".
- [X] T099 [P] Check that the learn chapters are complete and consistent across stories:
  - docs/learn/books/lines.mdx and docs/learn/books/pages.mdx: vertical lines and columns.
  - docs/learn/books/fonts.mdx: the CJK fallback and its cache.
  - docs/learn/books/css.mdx: writing modes, backgrounds.
  - docs/learn/screen/eink.mdx: orientation.
  - Then run `make docs-build` (no broken link).
- [X] T100 [P] Update the package docs (`doc.go`) of the packages whose domain grew: `internal/device/pocketbook/doc.go` (orientation), `internal/graphics/fonts/doc.go` (CJK fonts, vertical shaping), `internal/typesetting/layout/doc.go` (writing modes), `internal/ebook/epub/doc.go` (rendition, spreads)
- [X] T101 Check the constitution's gates on the finished branch:
  - `make build` (static; record the binary's growth against 16,056,480 bytes);
  - `go list -deps ./internal/app/... ./internal/shelf | grep -E 'servers/(komga|mealie|calibreweb)'` prints nothing;
  - `go test ./internal/shelf ./internal/graphics/pageimage ./internal/servers ./internal/app/state -run Golden` passes with the old golden files unchanged;
  - `make bench-compare` and the `-gcflags=-m` inlining list match build/bench/inlining-before.txt for the hot paths.
- [X] T102 Walk through every section of [quickstart.md](quickstart.md) and the reader checks it lists, then run `make verify` and `make baseline` one last time

---

## Dependencies & Execution Order

### Phase dependencies

- **Setup (Phase 1)**: none. T002 → T003 → T004. T005 and T006 run in parallel with them.
- **Foundational (Phase 2)**: after Setup. T007 → T008, T009. T010 → T011. T012 any time.
- **US1 (Phase 3)**: after Phase 2; T026 uses T010.
- **US2 (Phase 4)**: after Phase 2 and T005's outcome (T037 depends on it). T034 → T035 → T036 → T038. T039, T040 → T041 → T042 → T043.
- **US5 (Phase 5)**: after Phase 2 (T007, T009). Independent of US1 and US2.
- **US3 (Phase 6)**: after Phase 2 (T010).
  - T064 → T065 → T066, T067 → T068 (needs the reader).
  - T069 → T070 → T071 → T074.
  - T072 → T073.
  - T075 after T064 and T074.
  - Its fake book's grid uses US1's painter (T020/T022).
- **US4 (Phase 7)**: after Phase 2 and T006's go. Independent.
- **US6 (Phase 8)**: after Phase 2. Independent.
- **Polish (Phase 9)**: after the stories delivered.

### Story independence

- US1, US2, US4, US5 and US6 do not need each other.
- US3's spreads do not need US1, but its ICB-grid page shows a tiled background only once US1's painter is in. Without it, that page keeps its color.
- US2's fonts help every story's CJK text but block none.

### Within each story

Tests first, written to fail; then model and parsing; then layout or drawing; then the app; then
the fake book, script and shots; then `make verify` and the judgments.

---

## Parallel opportunities

- Setup: T005 and T006 alongside T002–T004.
- Foundational: T008, T011 and T012 once their subject exists.
- US1: T013, T014 and T015 together. T016 is parallel to T024 (css vs svgimage) and to T023
  (layout emphasis, a different file from T020).
- US2: T031, T032 and T033 together. T039 is parallel to T034–T038.
- US3: T061, T062 and T063 together. The device track (T064–T068) runs in parallel with the
  epub/shelf track (T069–T072).
- US4 and US6 can run in parallel with any other story after Phase 2.

### Parallel example: User Story 1

```text
Task: "T013 Parser tests in internal/web/css/color_test.go and declarations_test.go"
Task: "T014 Stroke test in internal/web/svgimage/rasterize_test.go"
Task: "T015 Emphasis test in internal/typesetting/layout/emphasis_marks_test.go"
Then: "T016 gradients in internal/web/css/color.go" | "T023 emphasis_marks.go" | "T024 svgimage stroke scale"
```

### Parallel example: User Story 3

```text
Device track:  T063 → T064 → T065 → T066, T067 → T068
Book track:    T061, T062 → T069 → T070 → T071; T072 → T073
Then:          T074 → T075 → T076, T077 → T078–T084
```

---

## Implementation Strategy

### MVP first

1. Phase 1 and Phase 2: the conformance runner, the spikes, the deadline pool and rendition
   properties.
2. Phase 3 (US1): styled books, the most-read kind.
3. Stop and validate: the showcase shots, `make epub-tests`, `make verify`; deploy to the reader.

### Incremental delivery

1. US1, then US5. Both are small and lock in fidelity and safety.
2. US2: the largest piece (weeks).
3. US3: spreads, which need the reader to settle the landscape direction.
4. US4 and US6: small.
5. Each story ends with its shots, `make verify`, an updated judgments file and a baseline, so
   it ships alone.

---

## Notes

- One change per commit with a short imperative message; `make verify` before each.
- Every changed shot is opened and looked at before `make baseline`.
- Nothing in a book runs or is fetched at any step; a task that would need it is wrong.

## Phase 10: Convergence

- [X] T103 CRITICAL: Bring the learn chapter's font fallback widget in line with the code: `docs/src/components/fonts/FontFallback.tsx` (and `docs/src/lib/fontFallback`) list the fallbacks as `fonts.OpenFallbacks` opens them (DejaVu Sans, the firmware's Noto Sans Arabic UI, the CJK font, Go's), and the hint of "One line, several fonts" in `docs/learn/books/fonts.mdx` no longer says only the app's own CJK font has Kanji and Hangul (the firmware's comes first); run `make docs-build` per Constitution VII (contradicts)
- [X] T104 Time on the reader the heaviest pages of the W3C suite's books and a hostile-input set (deep nesting, huge CSS, large SVG and MathML, a 30-megapixel image): each page appears within 10 seconds and the app keeps responding; record the times in research.md R8 and docs/reference/epub-conformance.md per SC-006 (missing)
- [X] T105 Compare on the reader the page-turn time of a horizontal book (a novel of the Calibre library) with the build before this feature and with this one (`go run ./cmd/readerctl log`), and record both in research.md R6: no slower, per SC-005 (partial)
- [X] T106 Time on the reader a page of formulas and a full-page JPEG XL decode, record them in research.md and docs/reference/epub-conformance.md ("Still to measure there"), and check both against the 10-second limit per FR-014a and SC-005 (partial)
- [X] T107 Add `BenchmarkLabel` to `internal/typesetting/layout/bench_test.go` (an Arabic, a Japanese and a mixed title, `for b.Loop()`, `b.ReportAllocs()`), as `layout.Label` now runs on every interface draw of such a title, per Constitution IV (partial)
- [X] T108 Record in plan.md the decisions taken after the plan: the firmware's CJK collections read first with the compiled-in fonts as fallback, the firmware's Noto Sans Arabic UI as a fallback that shapes Arabic (copied into `emulator/fonts`), interface titles set by `layout.Label`, and `make reader-remote`; per plan: R6 decision and spec Assumptions (unrequested)

## Phase 11: Convergence

- [X] T109 Count a page's 10-second budget from the start of its preparation, not of its draw: start `layout.Elements` (`Started`) before the page's chapter is laid out in `internal/reading/renderer.go` (and the fixed-layout path), so that layout time comes out of the same budget; a test with a slow layout and a slow element shows the page within the budget; re-time the hostile SVG page on the reader (under 10 s, against 11.45 s) and update research.md R8 and the first "Remaining work" item of docs/reference/epub-conformance.md, per SC-006 and FR-014a (partial)
- [X] T110 Bound the time of the CSS cascade on hostile stylesheets in `internal/web/css` (a cap on the rules kept per book, or on rule-element match attempts per chapter, past which the first rules are kept and the rest ignored, logged once): a fuzz or unit test with a crafted stylesheet of many rules over a chapter of many elements stays under a set time; re-time the hostile book's 20,000-rule chapter on the reader (6.56 s today) and record it in research.md R8 and docs/reference/epub-conformance.md, per FR-014 (partial)
