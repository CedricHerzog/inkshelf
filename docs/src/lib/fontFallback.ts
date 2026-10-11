// Which font draws each character of a text, as forEachCoveredPart
// (internal/typesetting/layout/faces.go) decides: text the font covers is one part, in one
// check; otherwise each character the font lacks goes to the first fallback that has it
// (fallbacksOf: the reader's typeface, then fonts.OpenFallbacks's DejaVu Sans and Go), or stays
// in the font, to be drawn as its .notdef box. Neighbouring characters with the same font make
// one part. The fonts' coverage here is a simplified picture, by Unicode blocks, not their
// real tables.

export type ToyFont = {
  name: string;
  /** Inclusive code point ranges the font has a glyph for. */
  ranges: readonly [number, number][];
};

const latin: [number, number][] = [
  [0x20, 0x7e],
  [0xa0, 0x24f],
];
const punctuation: [number, number][] = [[0x2010, 0x2027]];
const greekCyrillic: [number, number][] = [
  [0x370, 0x3ff],
  [0x400, 0x4ff],
];

/** A font embedded in a book that only has Latin letters, as many do. */
export const bookFont: ToyFont = {name: 'Book font', ranges: [...latin]};

/** The reader's serif typeface, Liberation Serif. */
export const readerSerif: ToyFont = {name: 'Liberation Serif', ranges: [...latin, ...punctuation, ...greekCyrillic, [0x20ac, 0x20ac]]};

/** DejaVu Sans, the firmware font with the most characters. */
export const dejaVuSans: ToyFont = {
  name: 'DejaVu Sans',
  ranges: [...latin, ...punctuation, ...greekCyrillic, [0x20ac, 0x20ac], [0x590, 0x5ff], [0x600, 0x6ff], [0x2190, 0x21ff], [0x2200, 0x22ff], [0x2600, 0x26ff]],
};

/** Go's font, compiled into the program: the last resort, which always opens. */
export const goFont: ToyFont = {name: 'Go', ranges: [...latin, ...punctuation, ...greekCyrillic, [0x2190, 0x2199]]};

export function has(font: ToyFont, character: string): boolean {
  const code = character.codePointAt(0)!;
  return font.ranges.some(([first, last]) => code >= first && code <= last);
}

export type Part = {
  font: ToyFont;
  text: string;
  /** Set when the font lacks the characters: they are drawn as .notdef boxes. */
  missing: boolean;
};

/**
 * coveredParts cuts text into parts, one per run of characters drawn by the same font, as
 * forEachCoveredPart does. The fallbacks skip a font already tried, as fallbacksOf does.
 */
export function coveredParts(font: ToyFont, fallbacks: readonly ToyFont[], text: string): {parts: Part[]; oneCheck: boolean} {
  const characters = Array.from(text);
  if (characters.every((character) => has(font, character))) {
    return {parts: text === '' ? [] : [{font, text, missing: false}], oneCheck: true};
  }
  const tried = fallbacks.filter((fallback) => fallback !== font);
  const parts: Part[] = [];
  for (const character of characters) {
    let chosen = font;
    let missing = false;
    if (!has(font, character)) {
      const found = tried.find((fallback) => has(fallback, character));
      if (found) chosen = found;
      else missing = true;
    }
    const last = parts[parts.length - 1];
    if (last && last.font === chosen && last.missing === missing) last.text += character;
    else parts.push({font: chosen, text: character, missing});
  }
  return {parts, oneCheck: false};
}
