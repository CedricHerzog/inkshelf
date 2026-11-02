# Feature Specification: EPUB 3 books shown as their publisher made them

**Feature Branch**: `001-epub3-faithful-rendering`

**Created**: 2026-11-01

**Status**: Draft

**Input**: User description: "the ereader must display the ebook as the editor intended. All the ebook v3 features must be supported except the ones that execute code like javascript for instance (for security reasons)."

## Clarifications

### Session 2026-11-01

- Q: When you reach a two-page spread in a fixed-layout book, who turns the screen to landscape? → A: Automatic, per page: the app switches to landscape at each spread and back to portrait for single pages.
- Q: Should a reflowable book that asks for spreads also be shown as two pages side by side in landscape? → A: Only for images: a full-page image spread in a reflowable book is shown in landscape; text stays portrait.
- Q: Should fixed-layout books already on the reader be redrawn to get the improvements, and when? → A: Never automatically: only new downloads get them; delete and download a book again to update it.
- Q: May the app ship its own Chinese/Japanese/Korean font so those books never show missing-character boxes? → A: Yes, two fonts, a serif (Mincho/Song style) and a sans, stored compressed in the app to save space.
- Q: If one element on a page takes too long to prepare on the reader, should the app wait or give up after a time limit? → A: Give up on the element after 10 seconds and show its fallback or alternative text; the rest of the page appears.
- Q: Must AVIF be decoded even if no pure-Go decoder runs on the reader? → A: No: AVIF falls back to the manifest fallback when no pure-Go decoder runs on the reader within the limit, recorded as a gap.
- Q: Is the 10-second limit per element or per page? → A: Per page: the page waits at most 10 seconds in all; every element still not ready then shows its fallback.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A styled novel looks as it was designed (Priority: P1)

A reader opens a reflowable EPUB 3 whose publisher designed its look: chapter openings with
drop caps and ornaments, embedded fonts, emphasis marks, decorated backgrounds, illustrations in
SVG. Every page shows that design: the same fonts, spacing, alignment, decorations and images
the publisher chose, in grays instead of colors, with no part of the design dropped or
replaced by a plain fallback when the format allows it to be drawn.

**Why this priority**: most books read on the reader are reflowable; their look is what the
reader sees on every page.

**Independent Test**: open each of the W3C reading-system tests for CSS, fonts, SVG and
reflowable layout, and a set of real styled books; compare every page with what the test says
it must show, or with the book as published.

**Acceptance Scenarios**:

1. **Given** a book whose CSS uses emphasis marks of any shape (dots, circles, sesame,
   triangles), **When** its page is shown, **Then** each mark appears in its shape above or
   beside the emphasized characters.
2. **Given** an SVG illustration with a large or small coordinate system, **When** it is shown,
   **Then** its lines are as thick, relative to the drawing, as the SVG specifies.
3. **Given** a background made of repeated or tiled gradients, **When** the page is drawn,
   **Then** the pattern appears, not one flat color.
4. **Given** a book that sets `rendition:flow` or `rendition:layout` to a value a paged e-ink
   screen can honor, **When** it is opened, **Then** its pages follow that setting.

---

### User Story 2 - A book in any writing system reads correctly (Priority: P1)

A reader opens a Japanese, Chinese or Korean book set in vertical columns, a Mongolian text, an
Arabic or Hebrew novel. The text runs in the direction the publisher set, characters turn the
way the book says, short numbers sit upright in a column, pages turn in the book's direction,
and every character has a glyph.

**Why this priority**: a book in a vertical or right-to-left script is unreadable, not just
less pretty, when its direction is wrong or its glyphs missing.

**Independent Test**: open the W3C tests for vertical writing and internationalization and a
real vertical Japanese novel; check the column order, glyph orientation, combined digits, page
order and the absence of missing-glyph boxes.

**Acceptance Scenarios**:

1. **Given** a book whose writing mode is vertical right-to-left, **When** a page is shown,
   **Then** lines are columns read from the right, and the next page is reached with the
   gesture that turns a right-to-left book forward.
2. **Given** `text-orientation` and `text-combine-upright` in the CSS, **When** the page is
   drawn, **Then** Latin letters turn or stand as specified, and combined digits share one
   character cell.
3. **Given** a book in Chinese, Japanese, Korean, Arabic or Hebrew, **When** any page is shown,
   **Then** no character is drawn as a missing-glyph box.
4. **Given** a book whose spine direction is right-to-left, **When** the reader moves forward,
   **Then** pages come in the book's order.

---

### User Story 3 - Fixed-layout books and spreads look like the print edition (Priority: P2)

A reader opens a fixed-layout EPUB 3: a picture book, a cookbook, a comic made in EPUB. Each
page is drawn whole, its boxes, backgrounds and images where the publisher put them, and
two-page spreads are shown as the publisher paired them.

**Why this priority**: fixed-layout books already pass most tests; what is left is spreads and
backgrounds.

**Independent Test**: open the W3C fixed-layout tests and a real picture book with spreads;
check page geometry, clipping, backgrounds and spread pairing.

**Acceptance Scenarios**:

1. **Given** a fixed-layout book that asks for synthetic spreads, **When** it is read, **Then**
   the screen turns to landscape and shows the left and right pages of each spread side by
   side, paired as the book's `page-spread-*` properties say.
2. **Given** a spread whose book sets `rendition:orientation` to portrait, or a page marked
   `page-spread-center`, **When** it is reached, **Then** it is shown alone, on the portrait
   screen.
3. **Given** the reader leaves a spread for a single page, or the reverse, **When** the page
   turns, **Then** the app changes the screen's orientation by itself, with no action from the
   reader, and the reading position stays the same.
4. **Given** a page whose background is a tiled grid of gradients, **When** it is drawn,
   **Then** the grid appears.
5. **Given** a reflowable book with two consecutive spine items that each hold only one image
   and are marked `page-spread-left` and `page-spread-right`, **When** the reader reaches them,
   **Then** the two images are shown side by side in landscape, and the text before and after
   stays in portrait.
6. **Given** a reflowable book that asks for spreads, **When** a page of text is shown,
   **Then** it is one portrait page, as in a book without spreads.

---

### User Story 4 - Every core image format and formula shows (Priority: P2)

A reader opens a book that uses any of EPUB 3's core media types for images (including AVIF,
WebP, JPEG XL), and formulas in MathML. Each image appears; each formula is laid out.

**Why this priority**: a missing image loses content, but well-made books give a fallback,
which the reader already shows.

**Independent Test**: open the W3C core-media-type and MathML tests; every image and formula is
shown from its own data, not its fallback.

**Acceptance Scenarios**:

1. **Given** an AVIF image, **When** its page is shown, **Then** the image appears; when no
   pure-Go decoder runs on the reader within the limit, its manifest fallback appears instead
   (FR-008).
2. **Given** a formula in presentation MathML, **When** its page is shown, **Then** it is laid
   out in place, at the text's size.

---

### User Story 5 - Nothing in a book runs or calls out (Priority: P1)

A reader opens a book that contains scripts, forms, WebAssembly or links to remote resources.
The book is shown from its own static content and fallbacks; nothing in it runs, and nothing is
fetched from the network on its behalf.

**Why this priority**: a book is a file from anywhere; running its code on the reader is a
security risk the user excluded explicitly.

**Independent Test**: open the W3C scripting tests and books with remote resources while
watching the network and the process; no code runs, no request leaves the reader, and each
page shows its fallback or its static content.

**Acceptance Scenarios**:

1. **Given** a spine item marked `scripted` with a manifest fallback, **When** it is reached,
   **Then** the fallback is shown.
2. **Given** a page with a `<script>` and static text around it, **When** the page is shown,
   **Then** the static text is shown and the script is not run.
3. **Given** an image or font referenced by an `http:` or `https:` URL, **When** its page is
   shown, **Then** no request is made and the image's fallback, alternative text or the
   book's next font is used.

---

### User Story 6 - Audio, video and read-aloud leave the text readable (Priority: P3)

A reader opens a book with narration (media overlays), audio clips or videos. The InkPad 3 has
no speaker and its panel cannot play video, so these are out of scope: the book reads as text
and images, a video shows its poster image or fallback, and nothing is lost from the text.

**Why this priority**: such books are rare in this library, and the reader cannot play them.

**Independent Test**: open the W3C media-overlay tests and a book with audio and video; each
reads as text, with posters or fallbacks where the media were.

**Acceptance Scenarios**:

1. **Given** a book with media overlays, **When** it is read, **Then** its text and pages are
   shown as if it had none.
2. **Given** a `<video>` with a poster image, **When** its page is shown, **Then** the poster
   appears in its place; without a poster, its fallback content or nothing.
3. **Given** an `<audio>` element with fallback content, **When** its page is shown, **Then**
   the fallback content appears.

### Edge Cases

- A book uses a feature the reader can only approximate on a 16-gray screen (colors, color
  fonts, transparency): it is drawn in grays that keep the contrast between the colors.
- A book's CSS or SVG is malformed: the reader shows what it can parse, as browsers do, and
  never refuses the whole chapter.
- A book uses vertical writing and also embeds horizontal blocks (a table, a heading set
  sideways): each block keeps its own direction.
- A font the book needs is missing from the book and the reader: a reader font with the glyphs
  is used, and the text is never left out.
- A feature costs much more time on the reader (shaping a page of vertical text, decoding a
  large AVIF): the page still appears, with a progress indication if it takes longer than a
  usual page turn; the page waits at most 10 seconds in all, and every element still not ready
  then is replaced by its fallback (FR-014a).
- The reader's own preferences (text size, margins, font) meet the publisher's styles:
  publisher styles apply; the reader's text size scales them, as today.
- A reflowable book marks only one image as a spread side, or a spread item also holds text:
  it is not an image spread, and it is shown in portrait.
- A fixed-layout book was downloaded before this feature: it opens with its old pages, in
  portrait, without spreads; deleting and downloading it again gives the new rendering.
- A book is DRM-protected: it is refused with a message, as today.
- Positions saved on the reader before this feature (reading progress, bookmarks) still lead to
  the same text after it.

## Requirements *(mandatory)*

### Functional Requirements

**Fidelity**

- **FR-001**: The reader MUST lay out and draw every reflowable EPUB 3 chapter with the
  publisher's CSS as the EPUB 3.3 specification and its referenced CSS modules define it,
  within the limits of a paged 16-gray screen.
- **FR-002**: The reader MUST support vertical writing: `writing-mode` (vertical-rl,
  vertical-lr), `text-orientation`, `text-combine-upright`, and the vertical forms of
  underlines and emphasis marks, including the `-epub-` prefixed forms.
- **FR-003**: The reader MUST draw every emphasis-mark shape CSS defines, including sesame,
  whether or not the book's font has the glyph.
- **FR-004**: The reader MUST honor the page progression direction of the spine, and the
  direction of the page-turn gesture MUST follow it.
- **FR-005**: The reader MUST have a glyph for every character of the Chinese, Japanese, Korean,
  Arabic, Hebrew and Latin/Cyrillic/Greek scripts that a book uses, using reader-provided fonts
  when the book's fonts lack it.
- **FR-005a**: The app MUST ship two CJK fonts of its own, a serif (Mincho/Song style) and a
  sans, covering Chinese, Japanese and Korean. A book's serif text falls back to the serif, its
  other text to the sans. The fonts MUST be stored compressed in the app, so that they add as
  little as possible to its size on the reader and to each deploy.
- **FR-006**: The reader MUST draw SVG stroke widths, dashes and markers in the SVG's own
  coordinates, scaled with the image.
- **FR-007**: The reader MUST draw repeated gradients and backgrounds tiled by
  `background-size` and `background-repeat`, in reflowable and fixed-layout pages.
- **FR-008**: The reader MUST decode every image core media type of EPUB 3.3. AVIF is decoded
  when a pure-Go decoder runs on the reader within FR-014a's limit; otherwise its manifest
  fallback is shown, and SC-001 counts `pub-cmt-avif` as a recorded gap.
- **FR-009**: The reader MUST honor `rendition:layout`, `rendition:flow`, `rendition:spread`,
  `rendition:orientation` and the `page-spread-*` properties, each in the way that fits a
  paged screen; values that only make sense for scrolling (scrolled flows) MUST fall back to
  paged reading with the same content.
- **FR-010**: The reader MUST keep everything that passes today (navigation, footnotes,
  fallbacks, fonts and obfuscation, MathML, bidirectional text, fixed layout).

**Security**

- **FR-011**: The reader MUST NOT execute any code a book carries: JavaScript, event handlers,
  WebAssembly, forms that submit, or any other active content.
- **FR-012**: When a spine item or resource needs scripts, the reader MUST show its manifest
  fallback or its static content.
- **FR-013**: The reader MUST NOT fetch any resource on a book's behalf (remote images, fonts,
  styles, audio); a remote resource is treated as missing and its fallback is used.
- **FR-014**: Every parser that reads book content (CSS, SVG, MathML, fonts, images) MUST bound
  its time and memory on hostile input, so that a crafted book can neither crash nor hang the
  reader.
- **FR-014a**: When one element of a page (an image, an SVG, a formula) is not ready after 10
  seconds on the reader, the app MUST give up on it and show its fallback, or its alternative
  text, or nothing when it has neither; the rest of the page MUST appear. The page as a whole
  waits at most 10 seconds: past that, every element still not ready shows its fallback at
  once.

**Compatibility and feedback**

- **FR-015**: Books saved on the reader before this feature MUST still open, and their saved
  reading positions MUST lead to the same text. Pages drawn at download time are never redrawn
  automatically: a fixed-layout book downloaded before this feature keeps its old pages and is
  read one portrait page at a time, as before, until the reader deletes it and downloads it
  again.
- **FR-016**: When a book uses a feature the reader cannot honor (out of scope above), the book
  MUST still open and show the rest of its content.
- **FR-017**: Media overlays, audio and video are out of scope: the reader MUST ignore media
  overlays and show a video's poster or an audio or video element's fallback content.
- **FR-018**: A synthetic spread MUST be shown side by side with the screen turned to landscape;
  pages that are not paired (a portrait `rendition:orientation`, `page-spread-center`, a book
  without spreads) MUST be shown alone in portrait. The app MUST switch orientation by itself at
  each change between a spread and a single page, with no setting or action from the reader.
  Changing orientation MUST keep the reading position, and gestures and menus MUST work in both
  orientations.
- **FR-019**: In a reflowable book, only image spreads MUST be shown in landscape: two
  consecutive spine items, each holding a single image and nothing else that is visible,
  marked `page-spread-left` and `page-spread-right`. Text in a reflowable book MUST always be
  laid out as one portrait page, whatever `rendition:spread` says.

### Key Entities

- **Book**: an EPUB 3 publication, with its package (metadata, manifest, spine, rendition
  properties), its content documents, stylesheets, fonts and media.
- **Rendition properties**: the publisher's choices of layout, flow, spread, orientation and
  page progression, for the whole book or per spine item.
- **Fallback**: the alternative a book provides for a resource or document the reader cannot
  use (a manifest fallback, an image's alternative, `altimg` for MathML).
- **Conformance result**: for each W3C reading-system test, pass, fail or out of scope, with the
  reason.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every W3C EPUB 3 reading-system test (w3c/epub-tests) that is in scope passes:
  from 109 of 121 today to all of them but the two deliberate departures listed under
  Assumptions (lenient XML, an image missing from the manifest), and `pub-cmt-avif` if AVIF
  falls back under FR-008. Out of scope are only tests
  that need code execution, a network, sound or video playback.
- **SC-002**: Each test counted out of scope has a written reason that falls under one of the
  exclusions above.
- **SC-003**: A real vertical Japanese novel and a real Arabic novel read from start to end with
  no missing glyph, no text left out and pages in the book's order.
- **SC-004**: The 60-book library of the previous conformance run reads as before: the same
  pages, characters, notes and links, and its sample pages identical to the pixel unless a
  difference is a fixed defect that was looked at.
- **SC-005**: On the reader, a page turn in a horizontal book takes no longer than before, and a
  page of vertical or shaped text appears within 2 seconds.
- **SC-006**: No book in the W3C suite or a hostile-input test set makes the reader run code,
  open a network connection, crash or stop responding; on the reader, no page of those books
  takes more than 10 seconds before it appears.

## Assumptions

- "The editor" in the request means the publisher: the book shows the publisher's design, not a
  design the reader imposes.
- "EPUB v3" means EPUB 3.3, the current W3C Recommendation, with its referenced CSS, SVG and
  MathML specifications, measured by the W3C's reading-system tests.
- The reader shows grays: colors are mapped to grays, and a test passes when what it asks to see
  is visible in gray.
- The reader's text-size preference keeps scaling the publisher's styles, as today; it is the
  only reader preference that overrides them.
- Malformed XML stays read leniently, and an image missing from the manifest is still shown:
  real books depend on both, and they are deliberate departures already documented.
- Scrolled flows and `rendition:layout` roll are shown as pages, since the screen is paged
  e-ink; this counts as honoring them on this device.
- Metadata shown in the app's lists stays the server's (Calibre-Web reads the same package
  document); internationalization tests about titles are judged on the book's parsed metadata.
- DRM-protected books stay refused.
- The CJK fonts are under a license that allows shipping them with the app (for example the
  SIL Open Font License of the Noto CJK fonts).
- Only the InkPad 3 is targeted; another device would be checked again.
