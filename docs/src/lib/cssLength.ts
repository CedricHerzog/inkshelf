// CSS lengths as Inkshelf computes them, in hundredths of an em of the reader's font size.
// A TypeScript copy of lengthOf, fontSizeOf and the font-size clamp of computeStyle
// (internal/ebook/epub/style.go), and of css.ParseLength (internal/web/css/values.go).

/** The page that stylesheets are laid out for, in CSS pixels (pageWidth, pageHeight). */
export const pageWidth = 600;
export const pageHeight = 800;
/** pixelsPerEm: the browsers' default font size. */
export const pixelsPerEm = 16;
/** textWidth is the page's width in hundredths of an em: what 100% of a margin is. */
export const textWidth = (pageWidth * 100) / pixelsPerEm;
/** maxMargin is the largest margin or indent an element adds, in hundredths of an em. */
export const maxMargin = 600;
/** The font size clamp of computeStyle, in percent of the reader's size. */
export const minFontSize = 50;
export const maxFontSize = 300;

/** textSizes and defaultTextSize of internal/app/openbook/text.go, in InkPad 3 pixels. */
export const textSizes = [32, 36, 40, 44, 50, 56, 64] as const;
export const defaultTextSize = 44;

// lengthUnits are the units ParseLength accepts, with their size in CSS pixels when fixed.
const lengthUnits: Record<string, number> = {
  px: 1, pt: 96 / 72, pc: 16, in: 96, cm: 96 / 2.54, mm: 96 / 25.4, q: 96 / 101.6,
  em: 0, rem: 0, ex: 0, ch: 0, '%': 0, vw: 0, vh: 0, vmin: 0, vmax: 0, '': 0,
};

export type Length = {value: number; unit: string};

/** parseLength parses "1.5em", "-10%" or "0"; null if it is no length. */
export function parseLength(text: string): Length | null {
  const match = /^\s*([+-]?(?:\d+\.?\d*|\.\d+)(?:e[+-]?\d+)?)\s*(%|[a-z]*)\s*$/i.exec(text);
  if (!match) return null;
  const unit = match[2].toLowerCase();
  if (!(unit in lengthUnits)) return null;
  return {value: Number(match[1]), unit};
}

// Go's math.Round rounds halves away from zero; JavaScript's Math.round does not.
function roundHalfAway(value: number): number {
  return Math.sign(value) * Math.round(Math.abs(value));
}

/**
 * lengthOf returns a CSS length in hundredths of an em of the reader's font size. An em is
 * fontSize (in percent of the reader's), a percentage is of `hundred`, and a pixel 1/16 em.
 * It returns null if the value is no length (auto, normal…).
 */
export function lengthOf(value: string, fontSize: number, hundred: number): number | null {
  const length = parseLength(value);
  if (!length) return null;
  let hundredths: number;
  switch (length.unit) {
    case 'em':
      hundredths = length.value * fontSize;
      break;
    case 'rem':
      hundredths = length.value * 100;
      break;
    case 'ex':
    case 'ch':
      hundredths = (length.value * fontSize) / 2;
      break;
    case '%':
      hundredths = (length.value * hundred) / 100;
      break;
    case 'vw':
    case 'vmin':
      hundredths = (length.value * textWidth) / 100;
      break;
    case 'vh':
    case 'vmax':
      hundredths = (length.value * ((pageHeight * 100) / pixelsPerEm)) / 100;
      break;
    default: {
      const pixels = length.unit === '' ? (length.value === 0 ? 0 : null) : length.value * lengthUnits[length.unit];
      if (pixels === null) return null; // a number other than 0
      hundredths = (pixels * 100) / pixelsPerEm;
    }
  }
  return roundHalfAway(hundredths);
}

/** The absolute sizes of font-size, in percent of the reader's font size. */
export const fontSizeKeywords: Record<string, number> = {
  'xx-small': 60, 'x-small': 75, small: 89, medium: 100, large: 120, 'x-large': 150,
  'xx-large': 200, 'xxx-large': 300,
};

/** fontSizeOf returns the size a font-size value gives, from the parent's, in percent. */
export function fontSizeOf(value: string, parentSize: number): number {
  const keyword = value.trim().toLowerCase();
  if (keyword in fontSizeKeywords) return fontSizeKeywords[keyword];
  if (keyword === 'smaller') return Math.trunc((parentSize * 5) / 6);
  if (keyword === 'larger') return Math.trunc((parentSize * 6) / 5);
  const size = lengthOf(keyword, parentSize, parentSize);
  return size === null || size <= 0 ? parentSize : size;
}

/**
 * computedFontSize is the font size computeStyle keeps: fontSizeOf, clamped to 50–300% of
 * the reader's size unless the page is pre-paginated.
 */
export function computedFontSize(value: string, parentSize: number, fixedPage: boolean): number {
  const size = fontSizeOf(value, parentSize);
  return fixedPage ? size : Math.min(Math.max(size, minFontSize), maxFontSize);
}

/** emPixels is layout's Settings.em: hundredths of an em to pixels, at fontSize pixels. */
export function emPixels(hundredths: number, fontSize: number): number {
  return Math.trunc((hundredths * fontSize) / 100);
}
