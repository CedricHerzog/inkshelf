# Files on the reader

Inkshelf keeps everything it needs on the reader's internal storage, under
`/mnt/ext1/applications/` (shown as `applications/` when the reader is mounted over USB). These
files outlive any version of the app: an update replaces `inkshelf.app` and nothing else. So
**the format of every file below must stay readable by every later version**. Golden tests
guard it (see the end of this page).

## The tree

```text
/mnt/ext1/applications/
├── inkshelf.app                         the program (make deploy, or make reader-deploy)
├── inkshelf.app.new                     only while remote control installs a new program
├── icons/inkshelf.bmp, inkshelf_f.bmp   launcher icons
├── inkshelf.log                         the log, truncated at start-up above 1 MB
└── inkshelf/
    ├── config.json                      the servers, written by the user
    ├── profile, remote                  markers written by the user: profiling, remote control
    ├── fonts/                           the CJK fonts, decompressed from the program (a cache)
    └── servers/<name>/                  one folder per server, named after its "name"
        ├── filters.json                 the lists' filters and the reader's settings
        ├── covers/
        │   ├── series/<series id>.pg4
        │   └── books/<book id>.pg4
        └── books/<book id>/
            ├── book.json                the book and its progress, written last
            ├── 0001.pg4, 0002.pg4 …     a book of pages
            ├── spread-0002.pg4 …        the spreads of a fixed-layout EPUB, in landscape
            ├── book.epub, index.json, cover.pg4    a Reflowable EPUB
            └── book.cbz, book.epub.part …          only while downloading
```

The paths are set in [cmd/inkshelf](../../cmd/inkshelf) (`appDir`, `logPath`),
[app/session](../../internal/app/session) (`openServerDir`: `servers/<name>/`, `books/`, `covers/`)
and [shelf](../../internal/shelf) (the files of a book). Books live under `applications/` so that the
reader's own library does not index thousands of page images as pictures.

## config.json

Written by the user and copied over USB: a list of `servers.Config`
([servers/config.go](../../internal/servers/config.go)), or a single object.

```json
[{"name": "Komga", "type": "komga", "server": "http://nas:25600", "apiKey": "…"},
 {"name": "Livres", "type": "calibreweb", "server": "http://calibre.local:8083", "username": "…", "password": "…"}]
```

`type` is `komga` (or empty), `mealie` or `calibreweb`. `name` names the server's folder, so it
must be a valid FAT folder name and unique, ignoring case. The app never writes this file.

## filters.json

`state.Preferences` ([app/state/preferences.go](../../internal/app/state/preferences.go)), per
server: the series list's library (`library`, an id) and read status (`status`), the book list's
filter (`books`), the server's `libraries` (so the filter works offline), the reading
`directions` chosen per series id (`"ltr"`/`"rtl"`), `textSize`, the EPUB font size, and
`darkPages`, whether a series' pages show white on black in dark mode, per series id (`true` or
`false`, absent until chosen). The name is historical: keep it.

## Covers

The small covers of the lists, prepared like pages ([shelf](../../internal/shelf) `CoverFiles`).
A cover file's modification time is set to the series' or book's last change on the server, so a
cover older than its series is fetched again.

## A downloaded book

`book.json` is `shelf.Book` ([shelf/book.go](../../internal/shelf/book.go)): ids, titles, `pages`,
`format`, `number`, `direction`, and the progress: `page`, `completed`, `dirty` (the Go field is
`Unsynced`), `remove` (`PendingRemoval`), `readAt`, and for an EPUB `location`
(`{"chapter", "offset"}`, see [progress-sync.md](progress-sync.md)). A fixed-layout EPUB with
two-page spreads also has `spreads`: the pages, counted from 1, that start one (`[2, 4]`). Each
number n has its `spread-%04d.pg4` on disk, written before `book.json`; n + 1 is a page of the
book, and spreads do not overlap. The field is left out when empty: a book downloaded before
spreads reads in portrait, one page at a time, and an older version ignores the field.

**book.json is written last.** A folder without it is an interrupted download, and the next
`Download` resumes it, skipping pages already on disk. Every file is written to a temporary file
and renamed (`writeFileAtomic`), so a crash or a full disk never leaves half a file.

What else the folder holds depends on `format` (the integer values of `servers.Format`):

| `format` | What | Files |
|---|---|---|
| 0, `PageImages` | Komga pages, Mealie recipes | `0001.pg4` …; books from old versions have `0001.png` …, still read |
| 1, `ComicArchive` | a CBZ, or a fixed-layout EPUB drawn at download | `0001.pg4` …; for a fixed-layout EPUB, `spread-0002.pg4` … (1872×1404, the two pages side by side); `book.cbz` or `book.epub` only until the pages are made, then deleted |
| 2, `Reflowable` | an EPUB laid out while reading | `book.epub`; `index.json`; `cover.pg4` |

A CBZ or an EPUB is fetched to `book.cbz.part` or `book.epub.part`, renamed once complete.

### .pg4

A page prepared for the panel: `"PG4\x01"`, width and height as little-endian `uint16`, then
the rows, two pixels a byte (the left one in the high 4 bits), 0 black to 15 white. See
[graphics/pageimage/pg4.go](../../internal/graphics/pageimage/pg4.go) and
[image-pipeline.md](image-pipeline.md).

### index.json

`epub.Index` ([ebook/epub/index.go](../../internal/ebook/epub/index.go)): the length of each chapter
in runes, the table of contents, page list and landmarks as (chapter, rune offset), the language
and the page progression. It carries `version`, `epub.IndexVersion`: an index of another version
is built again when the book opens (`Store.LoadIndex`).

Bump `IndexVersion` when a change to [ebook/epub](../../internal/ebook/epub) changes how many runes a
chapter has, or what else the index says. Note that the reading positions saved in `book.json` are
rune offsets too: a change that moves offsets moves the readers' places in their books a little.
Version 8 (from 7) gives a vertical-rl book without a `page-progression-direction` the right to left
page progression; its rune counts are those of version 7, so saved positions keep their place.
Version 9 adds `spreads`, the image spreads of a reflowable book (`{"first", "left", "right"}`,
chapter indexes), left out when empty; rune counts do not change. The golden
`testdata/golden/index.json` is version 9, and `index-7.json` and `index-8.json` check that an older
index is built again.

## fonts

`fonts/NotoSansCJKjp-Regular.otf` and `fonts/NotoSerifCJKjp-Regular.otf` are the CJK fonts of
[graphics/fonts](../../internal/graphics/fonts) (`cjk.go`), decompressed from the program the first
time a page needs a Chinese, Japanese or Korean character, so that later starts skip the
decompression. Only on a firmware without Noto CJK of its own: the InkPad 3's 6.x has them, and
leaves the folder unwritten (a folder written by an earlier version stays, unused). A cache: deleting the folder is safe, and a file whose size is not the font's (written
in part when the reader went off, or by another version) is written again. Each is written to a
temporary file renamed over it. The emulator keeps its copy in `build/emulator-cache/fonts/`.

## inkshelf.log

Everything the app logs, plus stdout and stderr (so panics land there), from
[cmd/inkshelf/logging.go](../../cmd/inkshelf/logging.go). Read it over USB after trying something
on the reader.

## Files of older versions

- Single-server versions kept `books/` and `filters.json` directly in `inkshelf/`.
  `moveLegacyFiles` ([app/session](../../internal/app/session)) moves them into the first server's
  folder at start-up.
- The app used to be called komga. [deploy.sh](../../deploy.sh) moves `applications/komga/` to
  `applications/inkshelf/` and deletes the old program, icons and log.

## What must never change

- The JSON field names and value types of `config.json`, `filters.json`, `book.json` and
  `index.json`. Renaming a Go field is fine; its `json:"…"` tag stays (`Unsynced` is still
  `"dirty"`).
- The order of the `servers.Format` constants, saved as integers.
- The `.pg4` layout and the page file names (`%04d.pg4`).
- The folder layout above.

Adding an optional field is fine. Removing or renaming one is not: older files would lose it.

These golden tests read a file as a released version saved it (or, for `config.json`, as a
user writes it) and check what comes out; the files the app writes must be written back byte for
byte:

| Test | Guards |
|---|---|
| `TestConfigGolden` in [servers](../../internal/servers) | `config.json` |
| `TestFiltersGolden` in [app/state](../../internal/app/state) | `filters.json` |
| `TestBookGolden` and `TestIndexGolden` in [shelf](../../internal/shelf) | `book.json`, `index.json` |
| `TestPG4Golden` in [graphics/pageimage](../../internal/graphics/pageimage) | `.pg4` |

If one fails, the change broke the readers' files: change the code, not the golden file.
