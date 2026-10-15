# Using Inkshelf

An e-reader app for the books of [Komga](https://komga.org) and
[Calibre-Web](https://github.com/janeczku/calibre-web) servers and the recipes of
[Mealie](https://mealie.io) servers, written in pure Go. It runs on PocketBook e-readers: no
cgo, no PocketBook SDK, no libinkview. It draws to the e-ink panel and talks to the firmware
directly. It targets the InkPad 3 (PB740) on firmware 6.x.

- List your series and their books, with their covers. Filter the series by Komga library (e.g. Manga, Comics, BD)
  and by read status: reading, unread, read.
- Download a book to the reader. At download time, pages are resized to the screen, their
  contrast stretched and darkened for e-ink, and dithered to the panel's 16 grays, so reading
  works offline and page turns are fast. The next and previous pages are decoded ahead.
- Read right to left (manga) or left to right (comics, BD), as Komga's reading direction says;
  the reader's toolbar switches it for a series.
- Sync the reading progress with Komga, both ways.
- Also reads the recipes of a [Mealie](https://mealie.io) server: one series per category, each
  recipe laid out into pages (photo, times, ingredients, steps, notes) when it is downloaded.

## Install a release

Each [release](https://github.com/CedricHerzog/inkshelf/releases) has a zip laid out like the
reader's storage. Connect the reader over USB, choose **PC link**, and unzip it at the root of
the reader: it adds `applications/inkshelf.app` and the launcher icons
(`applications/icons/inkshelf.bmp`, `inkshelf_f.bmp`). Then write
`applications/inkshelf/config.json` ([Configure](#configure)) and eject the reader.

## Build and install

You need Go and `make`, and golangci-lint for `make verify` (`brew install golangci-lint`). The app
cross-compiles from any host:

```sh
make build    # -> build/inkshelf.app (linux/arm, static)
make test     # all tests, on the host
make lint     # golangci-lint, on the host and for the reader
make verify   # what every change must pass: build, lint, tests, and the emulator's screenshots
```

Connect the reader over USB, choose **PC link**, then run `make deploy`. This copies
`applications/inkshelf.app` and the launcher icons (`applications/icons/inkshelf.bmp`,
`inkshelf_f.bmp`), creates `applications/inkshelf/` and ejects the reader.

To get the icon in the launcher, add an entry to `system/config/desktop/view.json` as described
in pocketbook-passbolt's README, with `U_inkshelf`, `/mnt/ext1/applications/inkshelf.app`, the
title `Inkshelf` and the icons above.

The app used to be called komga. The first `make deploy` of Inkshelf moves `applications/komga/`
(servers, downloads, filters) to `applications/inkshelf/` and deletes `komga.app`, its icons and
its log. Its launcher entry has to be changed by hand: in `view.json`, rename `U_komga` to
`U_inkshelf` (in `applications` and in its group's `apps`) and replace `komga` by `inkshelf` in
its path and icons. Deploy reminds you while the entry still starts `komga.app`.

## Run it on the computer

The emulator (`cmd/emulator`) runs the app on an emulated reader, so UI changes can be tried
without the reader:

```sh
make emu-fake  # against a built-in fake Komga; open http://localhost:8740
make emu       # against the servers in emulator/config.json (same format as on the reader)
make shots     # headless: run emulator/scripts/smoke.txt, screenshots in build/shots
make shots SCRIPT=emulator/scripts/mealie.txt EMUFLAGS="-fake-type mealie"  # against a fake Mealie
make shots-all # every script of emulator/scripts, screenshots in build/shots/<script>
make emu-fake EMUFLAGS="-servers 2 -fake-type komga,mealie"           # one fake of each
```

On the page, a click is a tap, holding is a long press and dragging is a swipe. `[` and `]`
are the page buttons, `m` is Menu, and the side panel has Home/Return, Exit, Wi-Fi events and
the battery. "Outline refreshes" shows each panel refresh and its waveform.
`EMUFLAGS=-eink` makes refreshes as slow as on an InkPad 3, and `EMUFLAGS="-size 1072x1448"`
tries another screen, `EMUFLAGS="-servers 2"` (with `emu-fake` or `shots`) gives a choice of
fake servers. Use `make shots SCRIPT=…` to run another script; the commands are listed
at the top of `cmd/emulator/script.go`, and [emulator.md](emulator.md) explains them.

Downloaded books go to `emulator/servers/<name>/books/` (under `emulator/fake/` with the fake
servers). The emulator draws with the reader's fonts, kept in `emulator/fonts/`.

The emulator runs the real event loop and UI, but not the e-ink panel's ghosting, the reader's
speed, its Wi-Fi or its firmware: check those on the reader.

## Configure

In Komga, create an API key under **Account settings > API keys**. Then save this as
`applications/inkshelf/config.json` on the reader, with one entry per server:

```json
[
    {
        "name": "Manga",
        "server": "http://nas:25600",
        "apiKey": "<your API key>"
    },
    {
        "name": "Comics",
        "server": "http://nas:25601",
        "apiKey": "<the other server's API key>"
    }
]
```

With several servers, the app starts on a screen to choose one. The name is required: it is
shown on that screen and in the series list's header, and it names the folder of the server's
downloaded books and filters, so renaming a server leaves its downloads behind (rename its
folder under `applications/inkshelf/servers/` too). It cannot contain `/ \ : * ? " < > |`.
A single `{"name": …, "server": …, "apiKey": …}` object is accepted as well. An optional
`"type"` names the kind of server: `"komga"`, the default, `"mealie"` or `"calibreweb"`.

For Mealie, create an API token under **Profile > API Tokens** and add an entry such as:

```json
{
    "name": "Recettes",
    "type": "mealie",
    "server": "http://nas:9925",
    "apiKey": "<your Mealie API token>"
}
```

Each Mealie category is a series (recipes in no category are under *Uncategorized*, and a
recipe in two categories is in both), each recipe a book read left to right. Mealie has no
libraries and no read progress: the library filter is not shown, and the progress stays on the
reader. The app shows each server with its logo (Komga's or Mealie's), and Mealie's categories
and recipes by those names.

For Calibre-Web, give the username and password of a Calibre-Web user instead of an API key
(the app reads its OPDS catalog, at `/opds`):

```json
{
    "name": "Livres",
    "type": "calibreweb",
    "server": "http://nas:8083",
    "username": "<your username>",
    "password": "<your password>"
}
```

Each Calibre series is a series, its books numbered by their series index (2.5, or 5 when 4 is
missing), and the books in none are under *No series*. Series sort without their leading
article (*Le Trône de Fer* under T). Books are downloaded as CBZ (comics: the archive is
unpacked into pages) or EPUB; books with neither, and CBRs, are not listed. Calibre-Web keeps
no reading position over OPDS: the progress stays on the reader.

EPUBs are laid out on the reader as they are read: justified text in the reader's serif font
(Go's fonts if the firmware has none; a character a font lacks comes from another one), italic
and bold, headings, lists, quotes, illustrations and tables (as grids, their header repeated
on each page they continue on, their font smaller if too wide), each chapter on a new page,
following a small part of the book's CSS. The toolbar's A− and A+ change the size of the text, kept for the
server; the place in the book stays. Its label shows the page in the chapter and how far into
the book you are, and the lists show "~N pages" (of 1500 characters) and "At n%". The toolbar's
Contents lists the book's table of contents (its chapters, by their headings, when it lists
only one), the part being read marked, and, if the book has them, its printed pages and
landmarks as tabs; a tap goes there, and the toolbar then shows ‹ Return, back to the page
left. A book written right to left (its `page-progression-direction`, else its language) turns
its pages as manga do. Arabic and Hebrew text runs right to left, joined and mixed with left-to-right
text as in a browser; Chinese and Japanese lines break between ideographs, never before a closing
mark; formulas (MathML) are laid out with the text, at its size; SVG illustrations get their
styles and their text. Borders and background colors of paragraphs and boxes are drawn in
grays, and colored text as its gray, kept legible: light text is darkened, but on a dark box.
Books protected by DRM are refused at download. How much of EPUB 3.3
this covers, measured with the W3C's tests, is in [epub-conformance.md](epub-conformance.md).

A tap on a link follows it (the margins still turn pages). A link to another part of the book
goes there, and the toolbar shows ‹ Return. A footnote (an `aside` or `epub:type="footnote"`
the chapter links to, or a note that starts with a link back to its call) is not shown in the
text: its call opens it in a panel over the lower part of the page, and so do notes in a part
of the book outside the reading order. A tap in the panel or a page button shows its next page
("More ›"), a tap outside it or Back closes it. A link to the web or an email address shows its
address: the reader has no browser.

Fixed-layout EPUBs (`rendition:layout` pre-paginated: picture books, comics) are drawn at
download time, one page per spine item, and read as comics: the lists show their page count,
and the pages turn as the EPUB's `page-progression-direction` (else its language) says unless
the server says otherwise. Each page is drawn at the size its viewport gives (else its first image's), fitted
in the screen and clipped to it: a page that is one image shows it whole; otherwise the
background colors, simple gradients and images, the images and the text that `position: absolute`
places (in px, %, em), and the rest of the text laid out from the top. Floats, flexbox, grids,
transforms and vertical writing are left out, and two-page spreads show one page at a time. In an EPUB that is
reflowable but for some pages (often its cover), each such page is drawn the same way, as an
image filling a page of the text.

The books and filters of versions with a single server move to the first server's folder at the
next start.

## Use

| Screen | Tap | Keys |
| --- | --- | --- |
| Servers | a server to use it (only with several servers in config.json) | Back exits |
| Series | **Continue** (the first row) to go back to the book read last; the header to reload the list from the server; a series to open it, its cover to see it on the whole screen (tap or Back to close); a library or a read status above the list to filter; ‹ Prev / Next › or swipe to page; the page number to jump to a letter | Next/Prev page; Back or Menu exits |
| Books | a book to download it, or to read it once downloaded; its cover to see it on the whole screen; **Delete** removes the download; All, Downloaded or Unread above the list to filter. Downloads queue up: tap a queued book to take it out of the queue, the one downloading to cancel it. Hold a book for its menu: download it and the next unread ones (3 in all), mark it read or unread, delete its download | Next/Prev page; Back or Menu goes back |
| Reader | right to left: left third for the next page, right third for the previous one, swipe right for the next page; left to right: the opposite. Middle: toolbar with ‹ Back, the reading direction (tap to switch it for the series) and the page number | Next/Prev page; Menu or OK toggles the toolbar; Back closes the book |

Online, the next book of the series starts downloading once you are 80% into a book. Turning
past a book's last page shows that next book: **Read next** once it is
downloaded, **Download next** otherwise (it can be read when its download ends); the previous
page goes back to the book.

Rows show a bar of how far a book (or a series being read) is read, and a READ badge on the
cover of those read to the end.

A series' books list the downloaded ones first, then the others, each in Komga's order, and
open on the page of the book to continue (marked *Continue*): the one in progress, else the
first unread one after the last read one.

The filters are kept for the next start. A series is *Reading* once one of its books is started
and until all are read, as in Komga. Offline, the library filter knows only books downloaded with
this version (or opened online since), and the status counts only the downloaded books.

The panel does a full (flashing) refresh every 5 pages to clear ghosting. The lists show the
free space, the time and the battery level in their header. Closing a book read to the end
(or opening the next one from the end panel) asks whether to delete it from the reader; a book
whose progress isn't synced yet goes once it is. The reader doesn't go to sleep while a book
downloads.

Without the server (no Wi-Fi, server down), the app lists only the chosen server's downloaded
books and you can still read them. When the reader is not connected at start, those books show
at once, with "Connecting to Wi-Fi…" in the header, while the Wi-Fi turns on; once it connects,
the server's list replaces them, if you are still on the series list.

## Progress sync

- Each page turn saves the progress in the book's `book.json` and marks it unsynced. It is sent
  to Komga 3 seconds after the last turn, when the book is closed, when the app exits, and at the
  next start for anything read offline.
- When you open a book online, the app starts at Komga's progress, so you continue where
  another device stopped. Progress that hasn't been synced yet takes priority.
- Mealie keeps no progress: the reader's is the only one.

## Files on the reader

| Path | |
| --- | --- |
| `applications/inkshelf.app` | the app |
| `applications/inkshelf/config.json` | servers: name, address and API key (or username and password) |
| `applications/inkshelf/servers/<name>/filters.json` | the series and book lists' filters, the server's libraries and the reading directions chosen on the reader |
| `applications/inkshelf/servers/<name>/covers/` | the covers of the series and books listed, ready for the panel (`series/<id>.pg4`, `books/<id>.pg4`) |
| `applications/inkshelf/servers/<name>/books/<book id>/` | downloaded pages (`0001.pg4`, ...: 4-bit grayscale) and `book.json` (progress); while a CBZ or a fixed-layout EPUB downloads, `book.cbz` or `book.epub`; for an EPUB, `book.epub`, `index.json` (its chapters' lengths and table of contents) and `cover.pg4` |
| `applications/inkshelf.log` | log, truncated at startup above 1 MB |
| `applications/inkshelf/profile` | if it exists, the app profiles itself for the duration it holds (`90s`; 5 minutes if empty) into `applications/inkshelf-profile/`: see [profiling.md](profiling.md) |

The books are kept under `applications/` so that the reader's library doesn't index the page
images. A folder without `book.json` is an interrupted download; tapping the book resumes it.

## Layout

The folders are named after what you need to know to work in them, from the reader's hardware
at the bottom to the app's screens at the top. [architecture.md](architecture.md) is
the map; each package's `doc.go` explains its part.

```text
cmd/inkshelf/       the program for the reader: logging, then runs the app
cmd/emulator/       the app on an emulated reader: web page or headless script
emulator/           the emulator's fonts and scripts (scripts/), and, gitignored, its files
                    (config.json, servers/, fake/)
deploy.sh           make deploy: copies the app and icons to the reader over USB
icons/              the launcher icon (inkshelf.svg, and the BMPs made from it by make icons) and the
                    servers' logos (komga.svg, mealie.svg, calibreweb.svg: make icons makes each server's logo.png)
docs/               how it works: architecture, e-ink and firmware, images, EPUB, layout, sync…
internal/device/    the reader: framebuffer and panel refreshes, input, firmware IPC, Wi-Fi, emulator
internal/graphics/  fonts and text drawing; page images (decode, fit, tone, dither, .pg4 files)
internal/web/       the web formats of books: CSS, SVG, MathML, web fonts, images, URIs
internal/typesetting/ the document model, line breaking and pagination, hyphenation
internal/ebook/     EPUB (container, chapters as documents, fixed-layout pages), CBZ
internal/servers/   what the app needs from a server, and Komga, Mealie and Calibre-Web, each
                    with a fake for the emulator
internal/shelf/     books downloaded on the reader, their download, and the covers
internal/progress/  the reading progress, synced with the server
internal/reading/   reading an EPUB: positions, page turns, links and notes, contents
internal/app/       the app: state, services, and one package per screen
```
