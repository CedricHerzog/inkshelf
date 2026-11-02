# Pagination: from blocks to pages

[internal/typesetting/layout](../../internal/typesetting/layout) turns a
[document](document-model.md) into pages for the reader's screen. It does the job of a
browser's layout engine for the parts of CSS that books use. The entry point is

```go
pages, err := layout.Paginate(doc, settings, faces, imageSizer)
```

`Settings` give the page size, margins, font size, line spacing and whether to justify;
`Faces` open the fonts; the `ImageSizer` gives an image's size without decoding it. The result
is a list of `Page`s, each holding words, images, rules, strokes (underlines) and links at their
pixel positions, and the rune offsets it shows (`Start`, `End`). `Page.Draw` draws one.

```mermaid
flowchart TD
    block["a Block of Runs"] --> formats["formats: a font per run, at its size"]
    formats --> scripts{"scripts in the block?"}
    scripts -->|"CJK or break-all"| uax14["UAX 14 break opportunities"]
    scripts -->|"right to left"| bidi["bidi levels"]
    scripts -->|"Arabic, Indic..."| shaping["mark for shaping"]
    scripts -->|"Latin only"| fragments
    uax14 --> fragments["fragments: text between break points, measured"]
    bidi --> fragments
    shaping --> fragments
    fragments --> words["words: fragments that stay together"]
    words --> breaking["greedy line breaking, hyphenation"]
    breaking --> placing["placement: align, justify, baseline, visual order"]
    placing --> paging["on the page: margins, keep-with-next, new page"]
    paging --> page[("Page: words, images, rules, links at pixels")]
```

The code follows these steps in order: `text_preparation.go`, `fragments.go`,
`line_breaking.go`, `line_placement.go`, then `paginate.go`, with floats, tables and border
boxes in files of their own.

## 1. Preparing a block's text

Each run gets a *format*: its font opened at its size, its color, its link, its inline image
sized to fit (`makeFormats`). Then the scripts of the text decide what else is needed. Each of
these is costly, and Latin text needs none of them, so `Block.Scripts` (scanned once when the
document was built) lets most blocks skip them all:

- **Break opportunities.** Chinese and Japanese have no spaces: a line may break between most
  ideographs, but not before a closing bracket or a small kana. The Unicode line breaking
  algorithm (UAX #14, through `github.com/rivo/uniseg`) finds where (`break_opportunities.go`).
  Alphabetic text breaks only at spaces and dashes.
- **Bidirectional levels.** Hebrew and Arabic run right to left, and may hold numbers or Latin
  words that run left to right. The Unicode bidirectional algorithm (`bidi.go`) gives each
  character a *level*: even levels run left to right, odd ones right to left.
- **Shaping.** Scripts whose letters join or combine are measured and drawn as glyphs shaped by
  the font (see [epub-to-page.md](epub-to-page.md#fonts)).

## 2. Fragments and words

The text is cut at every point where a line may break into **fragments** (`fragments.go`):
stretches of text with no break inside, in one format, each measured once. What follows a
fragment is its *break kind*: a space (stretchable), a soft hyphen U+00AD (draws a hyphen if
the line breaks there), or a direct break after a dash, a zero-width space or an ideograph. A
no-break space never breaks.

Consecutive fragments with no break between them form a **word**, the smallest thing a line may
hold. A word is usually one fragment; "un*believ*able" is three, in two styles.

## 3. Line breaking

Lines are filled **greedily**: words go on the line until the next one does not fit, then a new
line starts (`breakLines` in `line_breaking.go`). Greedy breaking is what browsers do. It is
fast and its result never changes earlier lines, which matters on a slow processor.

```mermaid
flowchart TD
    next["next word"] --> fits{"fits on the line?"}
    fits -->|yes| add["add it to the line"]
    fits -->|no| hyph{"hyphenation on, and word not tried yet?"}
    hyph -->|yes| split["split at the last hyphen point that fits, put the parts back"]
    hyph -->|no| empty{"line already has words?"}
    empty -->|yes| finish["finish the line, retry the word on a new one"]
    empty -->|no| cut["word wider than a line: cut it between characters"]
    add --> next
    split --> next
    finish --> next
    cut --> next
```

**Hyphenation** uses [internal/typesetting/hyphen](../../internal/typesetting/hyphen): Liang's
algorithm, the one TeX uses, with patterns for a few languages. It runs only for a word that
does not fit, so most words are never hyphenated or even looked up. A line ending at a soft
hyphen draws a hyphen; if that hyphen does not fit, the last word moves to the next line.

## 4. Placing a line

`line_placement.go` places each line's words:

- **Alignment**: left, right, centred, or justified. *Justification* spreads the room left over
  the line's stretchable gaps (spaces, and the breaks next to ideographs). The last line of a
  justified paragraph aligns at the start, as in print, unless `AlignLast` says otherwise. In a
  right-to-left block, "start" is the right and the first-line indent is on the right.
- **Height and baseline**: all words of a line share one baseline. A line is as high as its
  largest font, inline image, emphasis marks or formula.
- **Visual order**: with bidi levels, the line's fragments are reordered for display (rule L2
  of the bidi algorithm). Offsets keep the logical order.
- Underlines and strikes become `Stroke`s, continuous across a run; links become tap
  rectangles, merged per line.
- **Emphasis marks** (`text-emphasis`) go above or under each character but spaces and
  punctuation, half its size. A keyword mark (dot, circle, double circle, triangle, sesame,
  filled or open) is a `PlacedWord` whose `Mark` is set, drawn as a shape in a square of that
  size (`emphasis_marks.go`): few fonts have the sesame, and a font's marks are not the size and
  place CSS gives them. A mark that is another character is drawn in the reader's font, or as a
  dot when no font has it.

## 5. Pages

`paginate.go` stacks blocks down the page, keeping a cursor `y`:

- **Margins collapse**, as in CSS: between two blocks the larger of the two margins is used,
  not their sum. A block at the top of a page has no top margin.
- **A new page starts** when the next line does not fit, or when a block has `PageBreak`.
- **Orphans and headings**: a paragraph keeps its first two lines together, so no page ends
  with a lone first line. A heading (`KeepWithNext`) moves to the next page unless two lines of
  the following block fit after it.
- **Images** are fitted in the page, enlarged at most three times.
- Anything that does not fit on an *empty* page goes there anyway: the next page would be no
  better.

## Floats

A float group (a picture with its caption, a sidebar) is laid out **apart**, in its own width,
then placed at its side where the next block starts (`floats.go`). The lines of the following
blocks are laid out again, shorter, while they are beside it. Several floats stack. A float
that does not fit on the rest of the page waits for the next page while the text goes on. One
wider than 60% of the text, or on a page too narrow for text beside it, stays in the text as
ordinary blocks. A floated letter much larger than its paragraph is a **drop cap**: it sits on
the baseline of one of the paragraph's first lines.

## Tables

A table is laid out as a grid (`tables.go`, `table_columns.go`, `table_rows.go`):

1. **Columns**: each cell's content is measured at its narrowest (its longest word) and widest
   (all on one line). Widths the book asks for are tried first. A table too wide shrinks its
   font, down to 60% of the text's, then squeezes its columns.
2. **Cells**: each cell is laid out unjustified in its column's width.
3. **Rows** are placed in *bands*: rows that no cell spans out of. A band that fits is kept whole
   on a page. A band too tall for a page is cut between the lines of its cells. On each page the
   table continues on, its header rows are repeated on top (if they take a quarter of the page
   at most).

## Border boxes across pages

A border box (a sidebar, a boxed note) is drawn under its blocks (`border_boxes.go`). When the
page ends inside a box, it is *broken*: drawn to the page's bottom without its bottom border,
then *restarted* at the next page's top without its top border. A box whose first part would
hold nothing moves to the next page whole, rather than leaving a lone top border at the
bottom of a page. Margins collapse through a box that has no border or padding on that side.
Each part of a broken box paints its own background layers, sized and placed in that part.

## Child paginators

Floats and table cells need a layout of their own, in another width. Rather than a separate
engine, the `paginator` creates a **child paginator** with adjusted settings and lets it place
the blocks with the same code:

- `layOutApart` lays a float group out on a page of the float's width;
- `cellPaginator` lays a cell out on one endlessly tall page, recording each line, image or
  rule it places as a `contentUnit`, so that a row can later be cut between units.

The parent then copies the child's result onto its own page, moved to where it goes. A table
in a cell is laid out by the cell's paginator, so nesting works the same way.

## Vertical writing

Chinese and Japanese books are often set in vertical lines: each line is a column that goes
down, and the next column is to its left (`vertical-rl`), or to its right in Mongolian
(`vertical-lr`). The document's `WritingMode` (its body's) decides; `vertical.go` does the rest
with the same line breaking, justification and pagination as horizontal text.

- **Laid out turned.** `paginateIn` lays a vertical document out in *logical* coordinates: the
  page's width and height swapped (`Settings.turned`), its top and bottom margins at the start
  and end of the lines, and the margin of the side columns stack from at the top. A line is
  then as long as the page is high, and a page ends when the columns fill its width. Images are
  sized with their width and height swapped too (`turnedSizer`), so that they stand upright.
- **Turned back.** When the pages are done, `pageTurner.turnPage` moves everything onto the
  page: a logical x becomes a y, and a logical y an x, from the right in `vertical-rl`. A word of
  vertical text ends with its column's centre line as its `X` and the top of its text as its
  `Top`. Offsets do not change, so a reading position means the same text in both directions.
- **Upright or sideways.** Each character is drawn one of two ways (`characterForm`):
  ideographs and kana upright, one under the other, shaped top to bottom (`Font.ShapeVertical`)
  so that brackets, the long vowel mark and small kana take their vertical forms; Latin words
  as a horizontal line turned a quarter turn clockwise (`Font.DrawTurned`). Which is Unicode's
  `Vertical_Orientation` (UAX #50), generated from the UCD into `vertical_orientation.go`, and
  `text-orientation` can make everything upright or everything sideways. A fragment never mixes
  the two: `splitFragments` ends one where the form changes, as it does at a change of bidi
  level.
- **Combined.** A run of `text-combine-upright` (a year, a number) is one fragment one em long,
  drawn across, squeezed into the cell if it is wider (`drawCombined`). Each element of it is a
  cell of its own.
- **Marks and lines to the side.** Emphasis marks and underlines go to the right of the column,
  or to its left when `text-emphasis-position` or `text-underline-position` say `left`: above
  the logical line in `vertical-rl`, under it in `vertical-lr`. A strike-through goes down the
  column's middle. An inline image is centred on the column, as the characters around it.
- **Other directions inside.** A block whose lines go the other way than the page's (a
  horizontal table in a vertical book, or vertical text in a horizontal one) is an *orthogonal
  box*: `placeOrthogonal` lays its blocks out apart, in their own mode, on a page of the
  screen's size (a child paginator of its own), and places each resulting page as one box where
  the next block goes, as long as its content along the line and as wide as it across. Images
  and rules have no direction and stay in the page's.
- Not followed in vertical text: drop caps (merged into their paragraph), and the physical
  margins of CSS, which keep the meaning they have across: `margin-top` is the space before a
  block, at the right of a `vertical-rl` page, not the space above its lines. A formula is laid
  out as across.

`Settings.Horizontal` lays every document out across whatever its mode: a note's popup is a
band across the page.

## Slow elements

`Page.Draw` draws a page's images and formulas when it draws the page, not when it lays it
out: layout only needs their sizes, read from the image's header. Decoding a photograph or
drawing an SVG can take seconds on the reader, and a crafted one longer. So a page waits for its
elements 10 seconds in all (`PageBudget`), not each element 10 seconds.

- `Settings.Elements` (an `Elements`) names the pool of workers that prepares them, the page's
  budget (`Budget`, `PageBudget` if 0) and the book, which names each element's key with its
  path and size. A nil pool prepares everything on the goroutine that draws, with no limit:
  tests and tools.
- `Draw` starts the budget (`Elements.Started`) and prepares each background image, image and
  formula through `PrepareElement`, which waits at most what is left of it. Once the budget is
  spent, every element left shows its fallback without a wait.
- An element not ready in time shows its text alternative (`PlacedImage.Alt`, from the
  document's `Alt`) in its box, in the text's font, broken into lines between words and cut at
  the box's edges (`drawAlt`); without one the box stays empty. An image that cannot be decoded
  at all is left out, as before.
- A formula is laid out and drawn on the worker, on a white canvas of its own, then darkens the
  page under it: the worker never touches the page, which may have moved on.
- A fixed-layout page drawn at download (`fixedlayout.Page.Render`) shares one budget across its
  regions, through `Elements.Started` as well.

The workers, their pools and their locks are in [app-threading.md](app-threading.md#the-element-workers).
