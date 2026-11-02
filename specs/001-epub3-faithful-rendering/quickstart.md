# Quickstart: checking the feature end to end

How to see that each user story works and that nothing else moved. Commands are from
[CLAUDE.md](../../CLAUDE.md); the formats are in [contracts/](contracts/) and the types in
[data-model.md](data-model.md).

## Before starting

```sh
make baseline                      # if build/baseline/shots is missing
make bench-baseline                # before touching fonts/draw.go or the framebuffer copy
git clone https://github.com/w3c/epub-tests ~/src/epub-tests   # the conformance suite
```

## The W3C suite (SC-001, SC-002, SC-006)

```sh
make epub-tests EPUB_TESTS=~/src/epub-tests   # new: cmd/epubtests -> build/epub-tests/
```

For each test it writes the laid-out words and the first pages as PNGs, as the previous run's
throwaway program did (see [epub-conformance.md](../../docs/reference/epub-conformance.md)), and
a `results.tsv` of pass, fail or out of scope with the reason; pass and fail are judged by
looking, against what the test says it shows. Expected: every test in scope passes but the two
deliberate departures; every out-of-scope line names scripts, network, audio or video. The run
must open no network connection and finish without a crash.

## User story 1: a styled book

```sh
make shots SCRIPT=emulator/scripts/calibreweb-showcase.txt
```

The showcase book gains pages with sesame and triangle marks, a tiled gradient background, and
an SVG drawn in a 10x10 and a 1000x1000 viewBox with the same stroke. Look at them in
`build/shots/`: the marks in their shapes, the grid visible, both strokes as thick relative to
their drawing.

## User story 2: vertical and right-to-left books

```sh
make shots SCRIPT=emulator/scripts/calibreweb-vertical.txt   # new script and fake book
```

A vertical Japanese chapter: columns from the right, Latin words turned, "2026" as one upright
cell, emphasis marks to the right of the characters, no missing-glyph box; a tap on the left
moves forward. An Arabic chapter keeps its current shots.

## User story 3: spreads

```sh
make shots SCRIPT=emulator/scripts/calibreweb-fxl.txt
```

The fake fixed-layout book gains `rendition:spread` and `page-spread-*`. After download: the
cover alone in portrait, then landscape shots (1872x1404) of each pair, a centred page alone in
portrait, the menu opened over a spread. In `emulator/fake/.../books/<id>/`: `spread-*.pg4`
files and `spreads` in `book.json`. A book downloaded with the baseline build keeps portrait
pages (copy its folder before rebuilding, then open it).

A reflowable book with two image-only items marked left and right shows them in landscape, and
the text around them in portrait.

## User story 4: AVIF and formulas

```sh
make shots SCRIPT=emulator/scripts/calibreweb-mathml.txt
```

The fake book gains an AVIF image without fallback; it appears. The formulas keep their shots.

## User story 5: nothing runs, nothing is fetched

The scripting tests of the suite show their fallbacks or their static text. A fake chapter with
an `https:` image and a `<script>` shows the image's alternative text and the text around the
script. Watch the emulator's network: no request but to the fake server.

## The 10-second limit

`go test ./internal/deadline && go test ./internal/typesetting/layout -run 'TestDraw(ShowsAlt|Budget|WithoutPool)'` runs a job that sleeps
past the limit (a short limit in tests): the page is drawn, the element's alt text is in its
box, and the next draw does not wait. On the reader, a test book with a 30-megapixel AVIF shows
its page within the limit.

## Unchanged things

```sh
make verify                         # build, lint, tests, shots-all, shots-compare
make bench-compare                  # fonts/draw.go and the framebuffer copy: no slower
GOOS=linux GOARCH=arm GOARM=7 go build -gcflags=-m ./internal/graphics/fonts ./internal/device/pocketbook 2>&1 | grep inlin
go test ./internal/shelf ./internal/graphics/pageimage -run Golden
```

`shots-compare` lists only the shots changed on purpose. The 60-book library of the previous
conformance run reads with the same pages, characters, notes and links (SC-004).

## On the reader

```sh
make reader-deploy
make reader-shots READER_SCRIPT=emulator/scripts/reader/spreads.txt   # new, reads only
go run ./cmd/readerctl log
```

Check what only the reader shows: the landscape direction and its ghosting, the time of the
first CJK page (font decompression, about a second once), a vertical page turn (under 2 s), a
horizontal page turn (as before), an AVIF decode, and the thumbnail after going to the
background from a spread.
