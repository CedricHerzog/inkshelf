# From an EPUB to pixels

An EPUB is a zip of small web pages. A browser would show it with an HTML parser, a CSS
engine, a layout engine and a font stack. Inkshelf has none of those available on the reader,
so it carries a small version of each, limited to what books use. This page follows one
chapter from the zip file to the pixels on the panel, and says which package does each step.

```mermaid
flowchart TD
    zip[("book.epub: a zip")] --> container["container: container.xml, OPF, manifest, spine, nav"]
    container --> xhtml["chapter XHTML"]
    container --> sheets["CSS stylesheets"]
    container --> fontfiles["embedded fonts"]
    sheets --> cascade["css: parse, cascade"]
    xhtml --> builder["epub builder: token by token, stack of open elements"]
    cascade --> builder
    builder --> computed["computeStyle: inheritance, ems"]
    computed --> doc["document.Document: blocks of runs"]
    fontfiles --> webfont["webfont: de-obfuscate, WOFF to sfnt"]
    webfont --> fonts["fonts.File"]
    fonts --> doc
    doc --> paginate["layout.Paginate"]
    paginate --> pages["layout.Page list"]
    pages --> draw["Page.Draw"]
    draw --> faces["Faces: fonts, fallbacks, shaping"]
    draw --> images["webimage: raster images and SVG"]
    draw --> math["mathml: formulas"]
    draw --> gray[("gray image, then the panel")]
```

The packages, from the bottom up:

| Step | Package |
|---|---|
| Open the zip, read the package document and navigation | [internal/ebook/epub/container](../../internal/ebook/epub/container) |
| Parse CSS, match selectors, run the cascade | [internal/web/css](../../internal/web/css) |
| XHTML + CSS → document | [internal/ebook/epub](../../internal/ebook/epub) |
| The document model | [internal/typesetting/document](../../internal/typesetting/document) |
| Lines, pages, drawing | [internal/typesetting/layout](../../internal/typesetting/layout) |
| Fonts | [internal/graphics/fonts](../../internal/graphics/fonts), [internal/web/webfont](../../internal/web/webfont) |
| Images | [internal/web/webimage](../../internal/web/webimage), [internal/web/svgimage](../../internal/web/svgimage), [internal/graphics/pageimage](../../internal/graphics/pageimage) |
| Formulas | [internal/web/mathml](../../internal/web/mathml) |
| Fixed-layout pages | [internal/ebook/fixedlayout](../../internal/ebook/fixedlayout) |
| Turning pages, links, table of contents | [internal/reading](../../internal/reading) |

## 1. The container

The zip follows the Open Container Format. `META-INF/container.xml` names the *package
document* (the `.opf` file), which has three parts that matter:

- the **metadata**: title, language, identifiers, and the *rendition* properties: whether the
  book is reflowable or fixed-layout (`rendition:layout`), how it flows (`rendition:flow`),
  when two pages may show side by side (`rendition:spread`) and the orientation its pages are
  meant for (`rendition:orientation`). An `itemref` of the spine may override them for its own
  item, and place its page on one side of a spread (`page-spread-left`, `-right`, `-center`).
  `container.Rendition` holds them; unknown values are ignored, as EPUB 3.3 asks;
- the **manifest**: every file of the book, by id, with its media type. An item the app cannot
  show (a scripted page, an image in a format it does not decode) may name a *fallback* item to
  show instead;
- the **spine**: the reading order. Its *linear* items are the chapters; the others (notes,
  answers) are shown only when a link leads to them.

The **navigation document** (EPUB 3) or the **NCX** (EPUB 2) holds the table of contents, the
printed page list and landmarks. `container.Book` reads all of this once.

`META-INF/encryption.xml` lists encrypted files. A book with DRM is refused with `ErrDRM`.
Books that only *obfuscate* their fonts are fine: see Fonts below.

All paths are paths inside the zip. A link in a file is relative to that file, and
[internal/web/uri](../../internal/web/uri) resolves it.

## 2. XHTML and CSS into a document

`epub.Book.Chapter(index)` builds one chapter (`chapter.go`, `build*.go`):

- A **builder** reads the XHTML as a stream of tokens, keeping the open elements on a stack.
- Each element's **declared values** come from a `css.Cascade`: the user agent stylesheet
  (`userAgentCSS` in `style.go`, the defaults such as "h1 is bold and large") first, then the
  book's `<link>`ed sheets, `<style>` blocks and `style` attributes. The cascade decides which
  declaration wins: `!important`, then the more specific selector, then the later rule.
- CSS selectors only look at an element, its ancestors and its *previous* siblings. That is
  what lets the builder style an element the moment it opens, without the whole tree. Selectors
  that need later siblings (`:last-child`, `:empty`) are dropped.
- `computeStyle` turns declared values into **computed values**: it applies inheritance (a
  `<span>` inside an italic `<p>` is italic) and converts lengths to hundredths of an em.
- Text is gathered into **runs** of one style, whitespace collapsed as HTML does. A block
  element (`<p>`, `<h1>`, `<li>`) ends the current **block** and starts another.
- `html` opens before the stylesheets of its head are read, so its style is computed again
  when its `<body>` opens (`restyleRoot`): a writing mode or a font size set on `html` reaches
  the body and the rest, as in a browser.

Some elements produce more than plain blocks: a border or background becomes a border box, a
CSS float a float group, a `<table>` a grid of cells, `epub:type="footnote"` (or a link back to
its call) marks a footnote. [document-model.md](document-model.md) describes the result.

A box's background is a color, in gray, and up to 8 *layers* over it (`css.Backgrounds`): the
`background` shorthand is expanded into `background-color`, `-image`, `-repeat`, `-position` and
`-size` as it is parsed, and each image of `background-image` is a layer, with the others'
lists repeated to give it a size, a place and a repeat. An image is a `url()` the book has (a
remote one is left out) or a gradient: linear or radial, repeating or not, of up to 32 stops,
in pixels or percentages. The builder converts every length of a layer into the box's units
(`build_backgrounds.go`), and one routine, `layout.PaintBackground`, paints them for both kinds
of page: it sizes a tile (`cover`, `contain`, lengths, `auto` from the image's own size),
places it, and tiles it (`repeat`, `space`, `round`), up to 4096 tiles a layer.

**Vertical text.** `writing-mode` gives the direction of the lines: `horizontal-tb`,
`vertical-rl` or `vertical-lr`, with the SVG 1.1 values EPUB 3.0 books give `-epub-writing-mode`
(`tb-rl` and the like), and `sideways-rl`/`-lr` read as their vertical forms. The body's is the
chapter's (`Document.WritingMode`), as CSS takes it for HTML; each block keeps its own. With it
come `text-orientation` (`mixed`, `upright`, `sideways`, and EPUB 3.0's `vertical-right` and
`rotate-right`), `text-combine-upright` (`all`, `digits` 2 to 4; once named
`text-combine-horizontal`, and `-epub-text-combine: horizontal` in EPUB 3.0) and the `left` and
`right` of `text-emphasis-position` and `text-underline-position`. All are read with their
`-epub-` and `-webkit-` forms. In vertical text, a mark without a shape (`text-emphasis: filled`)
is a sesame, and each element of combined text stays a run of its own, a cell each. Margins and
paddings keep their horizontal meaning (see [pagination.md](pagination.md#vertical-writing)).

## 3. Layout and drawing

`layout.Paginate(doc, settings, faces, sizer)` breaks the document into `Page`s for a screen
size, margins and font size. Each `Page` holds words, images, rules and links at their pixel
positions. `Page.Draw` then draws a page into a gray image. [pagination.md](pagination.md)
explains the layout.

[internal/reading](../../internal/reading) is the caller while reading: it lays out the current
chapter and its neighbours, keeps their pages, and draws the page shown.

## Fonts

A book may embed fonts with `@font-face`. They arrive in three formats: plain TrueType or
OpenType (*sfnt*), WOFF (each table zlib-compressed) and WOFF2 (Brotli-compressed, some tables
transformed). Publishers often *obfuscate* them: the first bytes are XORed with a key derived
from the book's identifier. `container.Book.Font` loads each font once: `webfont.Deobfuscate`
(trying each identifier, then `webfont.RecoverAdobe`), `webfont.ToSFNT`, then `fonts.Parse`.

`layout.Faces` opens fonts when layout first needs them: the book's embedded families, the
generic `serif`/`sans-serif`/`monospace`, and the reader's typeface (the firmware's Liberation
and DejaVu fonts, else Go's fonts compiled in).

No font has every character. When a font lacks one, `Faces` draws that character with a
**fallback** (`fonts.OpenFallbacks`): the reader's typeface, DejaVu Sans, then a CJK font, then
Go's. The CJK fonts are the app's own, as the firmware has none: Noto Serif CJK JP for serif
text, Noto Sans CJK JP otherwise, which have Chinese and Korean too. They are compiled into the
program compressed with Brotli (27.5 MB for both), decompressed the first time a character needs
one into `inkshelf/fonts/` (0.2 to 0.3 s each on the Mac), and read from there at later starts
([on-disk-formats.md](on-disk-formats.md#fonts)). They have one weight: their bold is the
regular emboldened. A word its own font covers costs nothing extra, and every font file is parsed
once, whatever the sizes it is opened at.

Latin text is drawn one glyph per character. Arabic letters change shape with their neighbours,
Hebrew stacks points, and so on. For those scripts `fonts.Font.Shape` runs **shaping** with
go-text's pure-Go port of HarfBuzz, and only for text that needs it.

## Images

The document stores an image by its path in the zip, or as a `data:` URI. At draw time,
`webimage.NewLoader` reads the file and prepares it at the size the layout gave it:

- a raster image (JPEG, PNG, GIF, BMP, WebP, JPEG XL, AVIF) through `pageimage.Prepare`, the same
  stages as comic pages ([image-pipeline.md](image-pipeline.md));
- an SVG through `svgimage.Rasterize`: it runs a small CSS cascade on the SVG, hands shapes to
  `github.com/srwiley/oksvg`, and draws the SVG's `<text>` with the book's `Faces` (through
  `Faces.SVGText`).

An inline `<svg>` in the XHTML becomes an image whose path is a `data:image/svg+xml` URI of its
markup, styled by the chapter's CSS first. Layout only needs an image's size, which
`epub.Book.ImageSize` reads from the file's header without decoding it.

Each image keeps its text alternative (`Alt`): `<img alt>`, an inline SVG's own `<title>`, a
formula's `alttext`. A page waits 10 seconds in all for its images and formulas; one not ready
by then shows its text alternative in its box ([pagination.md](pagination.md#slow-elements)).

Nothing is fetched. A `src` with a scheme (`https:`) is resolved as a path inside the book,
which holds no such file: the image is missing and left out. An `@font-face` source that the
book does not hold is skipped for the next one, then for the next family of `font-family`.
Scripts (`<script>`, `on*` attributes, `javascript:` links) never run: their text is not shown,
and a spine item marked `scripted` shows its manifest fallback when it has one.

## Formulas

A `<math>` element is read by `mathml.Read` into a tree of nodes. It cannot be laid out yet:
its size depends on the text's font size, which is only known at layout time. So the builder
stores it as an *image* whose path is `mathml:` followed by its markup (`mathml.Path`).
When layout meets such a path (`mathml.Is`), it lays the formula out with `mathml.Layout` at
the size of the surrounding text, using the serif fonts of `Faces`, and places it on the line's
baseline. Formulas past mathml's size limits fall back to the book's `altimg`, then its
`alttext`. A formula laid out also keeps its `alttext` as its `Alt`, drawn in its place when it
is not ready in time.

## Fixed-layout pages

Some books (comics, children's books) are *pre-paginated*: each spine item is one page designed
at a fixed size, its elements placed with `position: absolute`. Text flowing around would make
no sense, so these chapters take another route:

- The builder, given a `fixedlayout.Page`, makes a `Region` of each positioned element and of
  each background; the page's size comes from its viewport `<meta>`. The `html` element's
  background is placed when `body` opens, under body's: the builder meets `<html>` before the
  stylesheets of its `<head>`.
- `Page.Render` draws the regions, scaled to the screen: backgrounds (a color, then its layers
  through `layout.PaintBackground`, an image's transparency kept), then each region's content
  (a lone image fitted, or text laid out by `layout` inside the region).
- A book whose chapters are all fixed-layout (`Book.FixedLayout`) is drawn page by page at
  download time (`Book.RenderPage`), prepared as `.pg4` pages, and then read like a comic.
- In a *mixed* book, a fixed-layout chapter's document is one image block whose path is
  `rendered-page:` followed by the item's path. `Book.Resource` recognizes the prefix and draws
  the page as a PNG at 1404×1872 when the image loader asks for it.

## Rolls and scrolled flows

A *roll* (`rendition:layout` roll, from the EPUB 3.4 draft) is a strip of pictures meant to be
scrolled down, as webtoons are: no gap between them, each as wide as the screen. A paged screen
cannot scroll, so the strip is cut into pages a screen high (`roll.go`): `Book.Roll` lays every
chapter's images out one under the other, fitted to the screen's width, and `Book.RenderPage`
draws the slice a page covers, a picture going on over as many pages as it needs. Like a
fixed-layout book, a roll is drawn at download time (`Book.DrawnAsPages`, `Book.PageCount`) and
read as a comic. A roll whose chapters hold text is read as a reflowable book.

`rendition:flow` asks for pages (`paginated`) or for scrolling (`scrolled-continuous`,
`scrolled-doc`). The panel only shows pages, so every flow is paginated, with the same content.

## The index and rune offsets

Laying out a whole book at the reader's font size takes seconds. But the app needs the book's
length for progress, and where each table-of-contents entry leads, as soon as the book opens.

`epub.BuildIndex` reads every chapter once, at download time, without fonts, and saves an
`Index` as `index.json` next to the book: each chapter's length, the table of contents, page
list and landmarks (each as a chapter and an offset), the language and page progression.
`IndexVersion` is bumped when the index changes; `shelf.Store.LoadIndex` rebuilds a stale one.
A book whose spine gives no `page-progression-direction` turns its pages from right to left when
its first chapter with text is `vertical-rl`, as its columns go (version 8).

Every position is a **rune offset** in its chapter's document: a count of characters, an image
or a rule counting as one. Offsets come from the document, not the layout, so they never
depend on the font size or the screen. A reading position is saved as a chapter and the offset
of the page's first rune (`shelf.Location`). After a font size change, the reader lays the
chapter out again and opens the page that holds that offset (`layout.PageAt`). For the lists
and progress sync, a book also has virtual pages of `shelf.CharsPerPage` (1500) runes.
