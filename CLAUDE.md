# CLAUDE.md

Guidance for Claude Code (claude.ai/code) and for people working in this repository.

## What this is

Inkshelf: an e-reader app for Komga (comics and manga), Mealie (recipes) and Calibre-Web (EPUB
and CBZ) servers, in pure Go: no cgo, no PocketBook SDK, no libinkview. It draws into
`/dev/fb0`, refreshes the e-ink panel with MXCFB ioctls and talks to the firmware's IPC itself.
Target: PocketBook InkPad 3 (PB740, 1404x1872, B288) on firmware 6.x. A Kobo or Kindle port may
come, and more kinds of servers: keep names free of the device and of the server outside the
packages that are about one.

[docs](docs) is the documentation site (`make docs`). [docs/reference](docs/reference) is the
precise reference, in Markdown: [usage.md](docs/reference/usage.md) is the user's view (screens,
gestures, sync, configuration); for the code, start with
[architecture.md](docs/reference/architecture.md). [docs/learn](docs/learn) teaches the same
domains from scratch, with interactive widgets, for newcomers. A change that makes a reference
page or a chapter wrong updates it too; how to write them is in [docs/WRITING.md](docs/WRITING.md).

## Commands

```sh
make build      # cross-compile -> build/inkshelf.app (CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7, static)
make test       # go test ./... on the host
make lint       # golangci-lint (.golangci.yml) on the host and for the reader; brew install golangci-lint
make verify     # build, make lint, go test, make shots-all, make shots-compare
make deploy     # deploy.sh: copy the app and icons to a reader mounted over USB, then eject
make icons      # icons/*.svg -> launcher BMPs and each server's logo.png (ImageMagick + rsvg-convert; committed, make -B icons)
make emu-fake   # the app on an emulated reader with fake servers, http://localhost:8740 (make emu: emulator/config.json)
make shots SCRIPT=emulator/scripts/mealie.txt EMUFLAGS="-fake-type mealie"   # one script, headless -> build/shots
make shots-all  # every script with its flags -> build/shots/<script>/
make baseline   # shots-all, saved as build/baseline/shots for shots-compare
make bench      # every benchmark x6 -> build/bench/new.txt (BENCH=Pattern PKG=./internal/...); bench-baseline, bench-compare (benchstat)
make profile SCRIPT=emulator/scripts/calibreweb-epub.txt   # a script with -profile -> build/profile/<script>/ (cpu.pprof, trace.out, ...)
make docs       # the documentation site (docs, Docusaurus, Node 20+), http://localhost:3000/inkshelf/
make docs-build # its typecheck and build, which fail on a broken link
```

Single test: `go test ./internal/graphics/pageimage -run TestPrepare`. The reader-only code of
`device/pocketbook` (files ending in `_linux_arm.go`) builds only for the reader: check it with
`GOOS=linux GOARCH=arm golangci-lint run ./...` (`make lint` does). Elsewhere `pocketbook.Run`
fails and the emulator stands in. Scripts, flags and screen coordinates:
[docs/reference/emulator.md](docs/reference/emulator.md).

## Definition of done

- `make verify` passes. Its `shots-compare` lists the shots that differ from `build/baseline`:
  none, or only changes you meant, each one opened and looked at (then `make baseline`). Run
  `make baseline` before you start if `build/baseline/shots` is missing.
- A UI change has a script step and a `shot` that shows it.
- A change to a hot path (below): its benchmark is no slower (`benchstat`, before and after), and
  `GOOS=linux GOARCH=arm GOARM=7 go build -gcflags=-m` shows the same inlining. See
  [docs/reference/image-pipeline.md](docs/reference/image-pipeline.md#hot-paths).
- Files on the reader stay readable: the golden tests pass unchanged. See
  [docs/reference/on-disk-formats.md](docs/reference/on-disk-formats.md).
- `app` and `shelf` import no specific server:
  `go list -deps ./internal/app/... ./internal/shelf | grep -E 'servers/(komga|mealie|calibreweb)'`
  prints nothing.

## The domain map

Domains go bottom to top: a package imports only from its own domain or lower ones
([docs/reference/architecture.md](docs/reference/architecture.md)).

```text
internal/
  device/pocketbook     the reader without its SDK: framebuffer, e-ink refresh, firmware IPC, event loop, Wi-Fi, emulator  -> docs/reference/eink-and-firmware.md
  graphics/fonts        TrueType fonts, fallbacks, HarfBuzz shaping, drawing text in gray
  graphics/pageimage    page images: decode, fit, tone, dither to 16 grays, .pg4  -> docs/reference/image-pipeline.md
  web/css               CSS parsing and cascade for EPUB stylesheets
  web/svgimage          SVG images, styled and rasterized
  web/mathml            MathML formulas laid out and drawn
  web/webfont           WOFF/WOFF2 to sfnt, font de-obfuscation
  web/webimage          a book's images, raster or SVG, sized and prepared
  web/uri               relative links and data: URIs
  typesetting/document  the document model: blocks of styled runs, tables, border boxes  -> docs/reference/document-model.md
  typesetting/layout    line breaking, bidi, justification, floats, tables, pagination, drawing  -> docs/reference/pagination.md
  typesetting/hyphen    Liang hyphenation with embedded patterns
  ebook/epub/container  an EPUB's zip, OPF, spine, navigation, resources, DRM
  ebook/epub            chapters to documents (XHTML+CSS), fixed-layout pages, the index  -> docs/reference/epub-to-page.md, docs/reference/epub-conformance.md
  ebook/fixedlayout     fixed-layout pages drawn whole
  ebook/comicarchive    the pages of a CBZ
  naturalorder          "tome 2" before "tome 10"
  timing                spans for the log and the execution trace, profile captures (any domain may use it)  -> docs/reference/profiling.md
  servers               the Server interface, its neutral model, config.json, title sorting  -> docs/reference/servers.md
  servers/{komga,mealie,calibreweb}   one package per kind of server, each with its fake*/ for the emulator
  servers/httpclient    the shared HTTP client and response handling
  servers/registry      Opener and KindOf, the switch on config.json's type (imported by cmd only)
  servers/servertest    a Server in memory for tests
  shelf                 downloaded books: Store, book.json, Download, covers  -> docs/reference/on-disk-formats.md
  progress              Merge (server vs reader) and the debounced Syncer  -> docs/reference/progress-sync.md
  reading               the engine for reading a reflowable EPUB: positions, links, notes, contents
  app                   the UI: dispatches input to screens  -> docs/reference/app-threading.md
  app/state             the State, its mutex, Update, Go, Preferences (filters.json)
  app/widgets           sizes, grays, fonts, buttons, panels, Targets and Commands
  app/{session,catalog,downloadqueue,covers,openbook}   services: functions over *state.State
  app/screens/*         one package per screen; screens never import each other
cmd/inkshelf            the program for the reader: logging, then pocketbook.Run
cmd/emulator            the app on the host, interactive or headless  -> docs/reference/emulator.md
emulator/               the emulator's fonts and scripts (committed); config.json, servers/, fake/ (gitignored)
```

## Conventions

- Descriptive names everywhere; no single-letter identifiers except `t *testing.T` and
  `b *testing.B`. Range loops (`for index := range count`, `for range n`) rather than 3-clause
  loops, which stay for hot paths, non-unit steps and loops from `Min.Y`.
- Threading ([docs/reference/app-threading.md](docs/reference/app-threading.md)): the event loop calls the app on one
  goroutine; other goroutines change the state only inside `state.Update` and never draw; only
  the main thread starts goroutines, with `state.Go`/`GoLoop`. A function whose name ends in
  `Locked` runs with the state's mutex held and never takes it: keep the suffix. Lock order:
  `progress.Syncer`'s lock, then the state's mutex, then a `reading.TextBook`'s renderer lock.
  Never call the Syncer with the state's mutex held.
- Comments are plain-English sentences that say why. A package doc (`doc.go`) teaches its
  domain's concepts to someone new to it. No file inventories, no comments that restate the
  code.
- Sizes in `app` are written in InkPad 3 pixels and scaled with `widgets.Px`. Text is placed by
  the top-left corner of its line, the baseline the font's ascent below it, as libinkview did.
- No copied third-party or standard-library code. Dependencies are pure Go.
- Lint findings are fixed, not silenced. A false positive gets `//nolint:<linter> // why`, or a
  rule in `.golangci.yml` with a comment when it repeats; a check that fights these conventions
  is turned off there, with the reason.
- Hot paths, tuned against the ARM assembly and marked `HOT PATH`: `copyRotated`
  (`device/pocketbook/framebuffer_copy.go`), `fit` (`pageimage/fit.go`, must match `fitRef`),
  `ditherAndPack` (`pageimage/dither.go`), `rgbaToGray` (`pageimage/decode.go`), `Unpack`
  (`pageimage/pg4.go`), the glyph drawing of `graphics/fonts/draw.go`. Change them only with
  benchmarks and the inlining list unchanged.
- Work that can take a while (I/O, decoding, layout, drawing, a refresh) runs in a
  `timing.Start` span named for the log and the trace, never inside a hot path. Costly code
  gets a benchmark (`for b.Loop()`, `b.ReportAllocs()`, inputs built in code). See
  [docs/reference/profiling.md](docs/reference/profiling.md).
- On-disk formats never change: JSON tags, the order of `servers.Format`, `.pg4`, file names.
  Bump `epub.IndexVersion` when a chapter's rune count changes.
- Commits: one change per commit, short imperative message; `make verify` before each.

## Things only the reader can check

The emulator runs the real event loop and screens, but not real ghosting, the reader's speed,
Wi-Fi, sleep, USB, or the firmware's behavior when the app goes to the background. For those:
`make deploy`, use the app, then read `applications/inkshelf.log` over USB (page turn times,
refresh waits, downloads, errors). Firmware details: [docs/reference/eink-and-firmware.md](docs/reference/eink-and-firmware.md).
To profile on the reader, create `applications/inkshelf/profile` (optionally holding a duration,
`90s`): the profiles land in `applications/inkshelf-profile/` ([docs/reference/profiling.md](docs/reference/profiling.md)).

## Paths on the reader

`/mnt/ext1/applications/inkshelf.app`; `/mnt/ext1/applications/inkshelf/config.json` (a list of
`{name, type, server, apiKey}`, or `username` and `password` for Calibre-Web);
`inkshelf/servers/<name>/filters.json`, `covers/`, `books/<book id>/` (pages, or `book.epub`,
`index.json`, `cover.pg4`; `book.json` last); `/mnt/ext1/applications/inkshelf.log`. Details in
[docs/reference/on-disk-formats.md](docs/reference/on-disk-formats.md). `deploy.sh` avoids macOS `._*` files
(`COPYFILE_DISABLE`, `cp -X`), which the launcher lists as broken apps.
