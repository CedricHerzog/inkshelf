# Data model: EPUB 3 books shown as their publisher made them

Phase 1 of [plan.md](plan.md). What each new or changed type holds, the rules on its values, and
where it is saved. Names are indicative; the code's names follow CLAUDE.md. Saved formats are in
[contracts/on-disk.md](contracts/on-disk.md).

## Rendition (ebook/epub/container)

The publisher's choices, for the whole package (`<meta property="rendition:*">`) and per
itemref (`properties`), the itemref overriding the package.

| Field | Values | Default |
|---|---|---|
| Layout | reflowable, pre-paginated, roll | reflowable |
| Flow | auto, paginated, scrolled-continuous, scrolled-doc | auto |
| Spread | none, landscape, both, auto (`portrait` read as `both`, deprecated) | auto |
| Orientation | auto, landscape, portrait | auto |
| PageSpread (itemref only) | none, left, right, center (`rendition:page-spread-center` too) | none |

Rules: an unknown value is ignored (the default stays), as EPUB 3.3 asks. Today's
`SpineItem.Layout` and `Book.Layout` become `Rendition.Layout`; `PageProgression` stays where it
is.

## Spread (ebook/epub)

A pair of consecutive spine items shown side by side in landscape.

| Field | Meaning |
|---|---|
| First | spine index of the item read first |
| Left, Right | spine indexes of the left and right pages (`Left` is `First` when pages turn left to right) |

Rules for a fixed-layout book (R2): pair only where the effective `Spread` is not `none`; a
pinned side is honored; unmarked items alternate, the first one alone; `center`, portrait
`Orientation`, or a reflowable item stands alone; two items pinned to the same side are not
paired. Rules for a reflowable book (R3): both items are image-only and pinned to opposite
sides, the first on the side reading starts from; anything else is not a spread.

## Book (shelf, book.json)

New field:

| Field | JSON | Meaning |
|---|---|---|
| Spreads | `spreads,omitempty` | 1-based page numbers that start a spread, ascending |

Rules: every number n has `spread-%04d.pg4` for n on disk and n+1 ≤ Pages; spreads do not
overlap. Empty for books downloaded before this feature, for comics and for books without
spreads. The reading position (`Page`) on a spread is its first page; a position on the second
page (from the server, or from an older version) is shown as that spread.

## Index (ebook/epub, index.json)

`Version` goes up one: 8 if spreads ship first, 9 after US2's bump for vertical books (T045). New field:

| Field | JSON | Meaning |
|---|---|---|
| Spreads | `spreads,omitempty` | reflowable image spreads, each `{"first": n, "left": n, "right": n}` with spine indexes |

Rune counts and `Total` do not change, so `Location` values stay valid.

## Style (typesetting/document)

New computed properties, inherited as CSS says:

| Field | Values | Default |
|---|---|---|
| WritingMode | horizontal-tb, vertical-rl, vertical-lr (`sideways-*` read as their vertical forms) | horizontal-tb |
| TextOrientation | mixed, upright, sideways | mixed |
| TextCombineUpright | none, all, digits(2..4) | none |
| EmphasisPosition | over/under, right/left | over right |
| Background | list of BackgroundLayer, at most 8 | none |

`Emphasis.Mark` stays a rune; drawing maps the keyword runes (•, ◦, ◉, ○, ▲, △, ﹅, ﹆) to
shapes.

## BackgroundLayer (typesetting/document)

| Field | Values |
|---|---|
| Image | none, an image resource, or a gradient |
| Gradient | kind (linear, radial), repeating, stops (≤ 32), angle or shape and position |
| Size | auto, cover, contain, or two lengths/percentages |
| Position | two lengths/percentages |
| Repeat | repeat, repeat-x, repeat-y, no-repeat, space, round |

Rules: tiles drawn per layer are capped (4096); past the cap the layer keeps its color only.

## Image (typesetting/document)

New field `Alt`: the `alt` of `<img>`, the `<title>` of an inline SVG, or a formula's `alttext`.
Not counted in runes.

## Orientation (device/pocketbook)

`Portrait` (default) or `Landscape`. Changing it swaps the screen size (1404x1872 ↔ 1872x1404),
the touch mapping and the canvas, and forces a flash refresh. Only the main thread changes it.
State transitions: portrait → landscape when a spread is drawn; landscape → portrait when a
single page, a non-reader screen, or the library is drawn, and when the app goes to the
background.

## Element deadline (deadline)

| Field | Meaning |
|---|---|
| Workers | a reading pool of 2 and a download pool of 1, long-lived goroutines started by the main thread |
| Limit | 10 s per page: each element waits min(10 s, what is left of the page's 10 s) |
| GivenUp | set of element keys (book, resource, size) given up this session |

States of a job: queued → running → done (result used) or given up (result dropped when it
comes). A key in GivenUp is not prepared again until the app restarts.

## CJK font cache (graphics/fonts)

`/mnt/ext1/applications/inkshelf/fonts/NotoSerifCJKjp-Regular.otf` and
`NotoSansCJKjp-Regular.otf`, decompressed from the binary on first need. A file whose size
differs from the embedded font's recorded size is written again.
