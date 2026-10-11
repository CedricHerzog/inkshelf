# Files on the reader

Inkshelf keeps everything it needs on the reader's internal storage, under
`/mnt/ext1/applications/` (shown as `applications/` when the reader is mounted over USB). These
files outlive any version of the app: an update replaces `inkshelf.app` and nothing else. So
**the format of every file below must stay readable by every later version**. Golden tests
guard it (see the end of this page).

## The tree

```text
/mnt/ext1/applications/
├── inkshelf.app                         the program (make deploy)
├── icons/inkshelf.bmp, inkshelf_f.bmp   launcher icons
├── inkshelf.log                         the log, truncated at start-up above 1 MB
└── inkshelf/
    ├── config.json                      the servers, written by the user
    └── servers/<name>/                  one folder per server, named after its "name"
        ├── filters.json                 the lists' filters and the reader's settings
        ├── covers/
        │   ├── series/<series id>.pg4
        │   └── books/<book id>.pg4
        └── books/<book id>/
            ├── book.json                the book and its progress, written last
            ├── 0001.pg4, 0002.pg4 …     a book of pages
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
`directions` chosen per series id (`"ltr"`/`"rtl"`), and `textSize`, the EPUB font size. The
name is historical: keep it.

## Covers

The small covers of the lists, prepared like pages ([shelf](../../internal/shelf) `CoverFiles`).
A cover file's modification time is set to the series' or book's last change on the server, so a
cover older than its series is fetched again.

## A downloaded book

`book.json` is `shelf.Book` ([shelf/book.go](../../internal/shelf/book.go)): ids, titles, `pages`,
`format`, `number`, `direction`, and the progress: `page`, `completed`, `dirty` (the Go field is
`Unsynced`), `remove` (`PendingRemoval`), `readAt`, and for an EPUB `location`
(`{"chapter", "offset"}`, see [progress-sync.md](progress-sync.md)).

**book.json is written last.** A folder without it is an interrupted download, and the next
`Download` resumes it, skipping pages already on disk. Every file is written to a temporary file
and renamed (`writeFileAtomic`), so a crash or a full disk never leaves half a file.

What else the folder holds depends on `format` (the integer values of `servers.Format`):

| `format` | What | Files |
|---|---|---|
| 0, `PageImages` | Komga pages, Mealie recipes | `0001.pg4` …; books from old versions have `0001.png` …, still read |
| 1, `ComicArchive` | a CBZ, or a fixed-layout EPUB drawn at download | `0001.pg4` …; `book.cbz` or `book.epub` only until the pages are made, then deleted |
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
chapter has. Note that the reading positions saved in `book.json` are rune offsets too: a change
that moves offsets moves the readers' places in their books a little.

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
