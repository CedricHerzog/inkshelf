# Contract: files on the reader

What this feature adds to the files of [on-disk-formats.md](../../../docs/reference/on-disk-formats.md).
Nothing existing changes: no JSON tag, no file name, no `.pg4` layout, no order of
`servers.Format`. Constitution principle III.

## book.json

One new optional field, written only when not empty:

```json
{ "id": "7", "pages": 24, "format": 1, "page": 5, "spreads": [2, 4, 6, 8] }
```

- Absent: the book reads in portrait, one page at a time (every book downloaded before).
- Present: `books/<id>/spread-0002.pg4` etc. exist; written before `book.json`, which stays the
  last file written.
- An older version reading a newer `book.json` ignores the field and shows single pages.

Golden test: `internal/shelf/testdata/golden/books/*/book.json` unchanged and still
byte-identical on save; a new golden book with `spreads`.

## spread-NNNN.pg4

Same `.pg4` format as pages, 1872x1404 (landscape), 16 grays. `NNNN` is the 1-based number of
the spread's first page in reading order, four digits as page files.

## index.json

`version` one higher than before (8, or 9 after US2's bump for vertical books); an index of another version is rebuilt on open, as today. New optional
field:

```json
{ "version": 8, "spreads": [ { "first": 12, "left": 12, "right": 13 } ] }
```

Golden test: `testdata/golden/index.json` moves up one version with `spreads`; the previous version's file
is kept as a test that it is rebuilt.

## fonts/

`/mnt/ext1/applications/inkshelf/fonts/` holds the decompressed CJK fonts. A cache: deleting it
is safe, it is written again on need.
