// Positions in a text book: layout.PageAt (internal/typesetting/layout/page.go), and the progress
// of internal/reading/textbook.go (global, Percent, VirtualPage) with shelf.CharsPerPage. The page
// breaks of a chapter come from a rough model of the app's page (internal/app/openbook/text.go),
// not from real fonts.

/** shelf.CharsPerPage: the length of a virtual page, in runes. */
export const charsPerPage = 1500;

/** openbook's textSizes, in InkPad 3 pixels, and its default. */
export const textSizes = [32, 36, 40, 44, 50, 56, 64] as const;
export const defaultTextSize = 44;

/** The text area of a page: the InkPad 3's 1404 x 1872 minus the margins of TextSettings. */
const textWidth = 1404 - 2 * 90;
const textHeight = 1872 - 80 - 100;
const lineSpacing = 135; // TextLineSpacing, in percent of the font size

/** An assumption of the model: an average character is half an em wide. */
const averageCharWidth = 0.5;

export function linesPerPage(size: number): number {
  return Math.floor(textHeight / Math.floor((size * lineSpacing) / 100));
}

export function charsPerLine(size: number): number {
  return Math.floor(textWidth / (size * averageCharWidth));
}

/**
 * modelPageStarts returns the offset of the first rune of each page of a chapter whose
 * paragraphs are `paragraphs` runes long: each paragraph fills whole lines, a page holds a fixed
 * number of lines.
 */
export function modelPageStarts(paragraphs: readonly number[], size: number): number[] {
  const perLine = charsPerLine(size);
  const perPage = linesPerPage(size);
  const starts = [0];
  let linesOnPage = 0;
  let offset = 0;
  for (const length of paragraphs) {
    for (let line = 0; line * perLine < length; line++) {
      if (linesOnPage === perPage) {
        starts.push(offset + line * perLine);
        linesOnPage = 0;
      }
      linesOnPage++;
    }
    offset += length;
  }
  return starts;
}

/** pageAt is layout.PageAt: the last page starting at or before offset. */
export function pageAt(starts: readonly number[], offset: number): number {
  let index = 0;
  for (const [candidate, start] of starts.entries()) {
    if (start > offset) break;
    index = candidate;
  }
  return index;
}

/** A location, as shelf.Location: a chapter (from 0) and a rune offset in it. */
export type Location = {chapter: number; offset: number};

/** globalOffset is TextBook.global: the offset from the start of the book, in runes. */
export function globalOffset(chapters: readonly number[], location: Location): number {
  return chapters.slice(0, location.chapter).reduce((sum, chars) => sum + chars, location.offset);
}

/** percent is TextBook.Percent: integer division, as in Go. */
export function percent(chapters: readonly number[], location: Location): number {
  const total = chapters.reduce((sum, chars) => sum + chars, 0);
  return total === 0 ? 0 : Math.floor((globalOffset(chapters, location) * 100) / total);
}

/** virtualPages is Book.Pages, as downloadReflowable computes it. */
export function virtualPages(total: number): number {
  return Math.max(Math.ceil(total / charsPerPage), 1);
}

/** virtualPage is TextBook.VirtualPage: from 1 to pages. */
export function virtualPage(chapters: readonly number[], location: Location): number {
  const total = chapters.reduce((sum, chars) => sum + chars, 0);
  const page = Math.floor(globalOffset(chapters, location) / charsPerPage) + 1;
  return Math.min(Math.max(page, 1), virtualPages(total));
}

/** locationOf turns an offset from the start of the book into a chapter and an offset in it. */
export function locationOf(chapters: readonly number[], global: number): Location {
  let rest = global;
  for (let chapter = 0; chapter < chapters.length; chapter++) {
    if (rest < chapters[chapter] || chapter === chapters.length - 1) {
      return {chapter, offset: Math.min(rest, Math.max(chapters[chapter] - 1, 0))};
    }
    rest -= chapters[chapter];
  }
  return {chapter: 0, offset: 0};
}
