// The numbers and rules of e-ink refreshes that the "How e-ink works" widgets play with, and a
// toy model of a panel that ghosts. The timings, the flash rhythm and the overlap rule come
// from the Go code (named next to each); the ghosting model is an illustration, since real
// ghosting depends on the panel's chemistry and the waveform tables in /boot/default.wbf.

import {seededRandom} from './sampleImages';

/* ---------- From the code ---------- */

/** The firmware's waveform numbers (eink_refresh.go, Waveform). */
export const waveformNumbers = {DU: 1, GC16: 2, GC4: 3, A2: 4, GL16: 5, DU4: 8} as const;

/**
 * How long a refresh takes on an InkPad 3, as measured: emulated_firmware.go's
 * fullRefreshTime, partialRefreshTime and fastRefreshTime.
 */
export const refreshMs = {full: 600, partial: 490, fast: 265} as const;

/** Every fullEvery turns of a comic (openbook.go), textFullEvery of a text book (text.go). */
export const fullEvery = 5;
export const textFullEvery = 6;

/**
 * fullRefreshDue follows openbook.go's fullRefreshDue: the turn about to happen flashes when
 * turns (counted since the book was opened) % every == every − 1, so every every-th turn.
 */
export function fullRefreshDue(turns: number, every: number): boolean {
  return turns % every === every - 1;
}

/** A rectangle from (minX, minY) included to (maxX, maxY) excluded, like Go's image.Rectangle. */
export type Rect = {minX: number; minY: number; maxX: number; maxY: number};

/** overlaps follows image.Rectangle.Overlaps: both non-empty and sharing at least a pixel. */
export function overlaps(first: Rect, second: Rect): boolean {
  const empty = (rect: Rect) => rect.minX >= rect.maxX || rect.minY >= rect.maxY;
  return (
    !empty(first) &&
    !empty(second) &&
    first.minX < second.maxX &&
    second.minX < first.maxX &&
    first.minY < second.maxY &&
    second.minY < first.maxY
  );
}

/**
 * When two refreshes run, the second asked for `requestedAt` ms after the first, under the
 * rule of readerFirmware.waitForOverlappingRefresh (framebuffer_linux_arm.go): the second
 * waits for the first to end only if their rectangles overlap.
 */
export function scheduleTwo(
  first: Rect,
  second: Rect,
  requestedAt: number,
  firstMs: number,
  secondMs: number,
): {overlap: boolean; secondStart: number; secondEnd: number; waited: number; allDone: number} {
  const overlap = overlaps(first, second);
  const secondStart = overlap ? Math.max(requestedAt, firstMs) : requestedAt;
  const secondEnd = secondStart + secondMs;
  return {
    overlap,
    secondStart,
    secondEnd,
    waited: secondStart - requestedAt,
    allDone: Math.max(firstMs, secondEnd),
  };
}

/* ---------- A toy panel ---------- */

export type RefreshKind = 'partial' | 'full' | 'fast';

/**
 * A toy e-ink panel. `target` is the image last asked for, `ghost` what the particles still
 * remember of earlier images (added to target to get what the eye sees).
 */
export type ToyPanel = {
  width: number;
  height: number;
  target: Float32Array;
  ghost: Float32Array;
};

export function newToyPanel(width: number, height: number, image: Uint8ClampedArray): ToyPanel {
  return {width, height, target: Float32Array.from(image), ghost: new Float32Array(width * height)};
}

/** What the eye sees of a toy panel: its target plus its ghost. */
export function visible(panel: ToyPanel): Uint8ClampedArray {
  const out = new Uint8ClampedArray(panel.target.length);
  for (let index = 0; index < out.length; index++) out[index] = panel.target[index] + panel.ghost[index];
  return out;
}

/**
 * refreshToy applies a refresh to `next`. A partial refresh drives only the pixels whose
 * target changes, and each keeps a share of what it showed before, so the ghosts of several
 * pages pile up; a fast (DU) one keeps more, and knows only black and white; a full one wipes
 * every ghost. `leak` is how much a partial refresh keeps.
 */
export function refreshToy(
  panel: ToyPanel,
  next: Uint8ClampedArray,
  kind: RefreshKind,
  leak: number,
): ToyPanel {
  const target = new Float32Array(next.length);
  const ghost = new Float32Array(next.length);
  for (let index = 0; index < next.length; index++) {
    const wanted = kind === 'fast' ? (next[index] < 128 ? 0 : 255) : next[index];
    target[index] = wanted;
    if (kind === 'full') continue;
    const before = panel.target[index] + panel.ghost[index];
    if (wanted === panel.target[index]) {
      ghost[index] = panel.ghost[index]; // not driven: whatever ghost it had stays
    } else {
      const keep = kind === 'fast' ? leak * 1.8 : leak;
      ghost[index] = panel.ghost[index] * 0.82 + (before - wanted) * keep;
    }
  }
  return {width: panel.width, height: panel.height, target, ghost};
}

/**
 * frameDuring returns what the eye sees `fraction` (0 to 1) of the way through a refresh from
 * `from` (visible before) to `to` (visible after). A full refresh goes through black, then
 * white, then the image; the others move each pixel straight to its new gray.
 */
export function frameDuring(
  from: Uint8ClampedArray,
  to: Uint8ClampedArray,
  kind: RefreshKind,
  fraction: number,
): Uint8ClampedArray {
  const out = new Uint8ClampedArray(to.length);
  const smooth = (value: number) => value * value * (3 - 2 * value);
  if (kind === 'full') {
    for (let index = 0; index < out.length; index++) {
      if (fraction < 0.3) {
        out[index] = from[index] * (1 - smooth(fraction / 0.3)); // to black
      } else if (fraction < 0.55) {
        out[index] = 255 * smooth((fraction - 0.3) / 0.25); // to white
      } else {
        out[index] = 255 + (to[index] - 255) * smooth(Math.min(1, (fraction - 0.55) / 0.4));
      }
    }
    return out;
  }
  const amount = smooth(Math.min(1, fraction));
  for (let index = 0; index < out.length; index++) out[index] = from[index] + (to[index] - from[index]) * amount;
  return out;
}

/* ---------- Pages to turn ---------- */

/**
 * makeTextPage draws page `number` of a pretend novel: a heading on the first page, then lines
 * of dark "words" (blocks) on white, each page different.
 */
export function makeTextPage(number: number, width: number, height: number): Uint8ClampedArray {
  const random = seededRandom(1000 + number);
  const pixels = new Uint8ClampedArray(width * height).fill(255);
  const margin = Math.round(width * 0.1);
  const lineHeight = Math.max(6, Math.round(height / 22));
  const wordHeight = Math.max(3, Math.round(lineHeight * 0.45));
  let y = margin;
  if (number % 4 === 0) {
    fill(pixels, width, margin, y, Math.round(width * 0.5), Math.round(lineHeight * 0.8), 20);
    y += lineHeight * 2;
  }
  while (y + wordHeight < height - margin) {
    let x = margin + (random() < 0.15 ? Math.round(width * 0.06) : 0);
    const lineEnd = random() < 0.12 ? margin + Math.round((width - 2 * margin) * random()) : width - margin;
    while (x < lineEnd) {
      const wordWidth = Math.round(width * (0.03 + random() * 0.09));
      fill(pixels, width, x, y, Math.min(wordWidth, lineEnd - x), wordHeight, 30 + Math.round(random() * 30));
      x += wordWidth + Math.round(width * 0.025);
    }
    y += lineHeight;
  }
  // The page number.
  fill(pixels, width, Math.round(width / 2 - 4), height - Math.round(margin / 2), 8, wordHeight, 60);
  return pixels;
}

/**
 * makeComicPage draws page `number` of a pretend comic: a few panels with shaded shapes, so
 * its grays and dark areas leave stronger ghosts than text.
 */
export function makeComicPage(number: number, width: number, height: number): Uint8ClampedArray {
  const random = seededRandom(2000 + number);
  const pixels = new Uint8ClampedArray(width * height).fill(255);
  const gutter = Math.max(3, Math.round(width * 0.04));
  const split = 0.35 + random() * 0.3;
  const rows = [gutter, Math.round(height * split), height - gutter];
  for (let row = 0; row < 2; row++) {
    const top = rows[row] + (row === 0 ? 0 : gutter / 2);
    const bottom = rows[row + 1] - (row === 0 ? gutter / 2 : 0);
    const columns = random() < 0.5 ? 1 : 2;
    for (let column = 0; column < columns; column++) {
      const left = gutter + column * Math.round((width - gutter) / columns);
      const right = Math.round(((column + 1) * (width - gutter)) / columns);
      drawPanel(pixels, width, left, top, right, bottom, random);
    }
  }
  return pixels;
}

function drawPanel(
  pixels: Uint8ClampedArray,
  width: number,
  left: number,
  top: number,
  right: number,
  bottom: number,
  random: () => number,
) {
  const shade = 140 + Math.round(random() * 90);
  const vertical = random() < 0.5;
  const cx = left + (right - left) * (0.3 + random() * 0.4);
  const cy = top + (bottom - top) * (0.35 + random() * 0.3);
  const radius = Math.min(right - left, bottom - top) * (0.22 + random() * 0.15);
  const dark = Math.round(random() * 60);
  for (let y = top; y < bottom; y++) {
    for (let x = left; x < right; x++) {
      const border = x < left + 2 || x >= right - 2 || y < top + 2 || y >= bottom - 2;
      let level = vertical
        ? shade + ((y - top) / (bottom - top)) * 60 - 30
        : shade + ((x - left) / (right - left)) * 60 - 30;
      const distance = Math.hypot(x - cx, y - cy);
      if (distance < radius) level = dark + (distance / radius) * 90;
      if (border) level = 15;
      pixels[y * width + x] = level;
    }
  }
}

function fill(
  pixels: Uint8ClampedArray,
  width: number,
  left: number,
  top: number,
  rectWidth: number,
  rectHeight: number,
  level: number,
) {
  const height = pixels.length / width;
  for (let y = Math.max(0, top); y < Math.min(height, top + rectHeight); y++) {
    for (let x = Math.max(0, left); x < Math.min(width, left + rectWidth); x++) pixels[y * width + x] = level;
  }
}
