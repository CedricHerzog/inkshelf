// TypeScript ports of the stages of internal/graphics/pageimage, for the widgets of the
// "From a scan to 16 grays" chapter. Each function names the Go function it follows; they use
// the same constants, the same integer divisions and the same order of operations, so a widget
// shows exactly what the reader would store. Integer division in Go truncates toward zero: here
// that is Math.trunc, or Math.floor where the operands are never negative.

/** A gray image: width×height levels from 0 (black) to 255 (white), row by row. */
export type GrayImage = {width: number; height: number; pixels: Uint8ClampedArray};

export function newGray(width: number, height: number): GrayImage {
  return {width, height, pixels: new Uint8ClampedArray(width * height)};
}

/* ---------- Decode: color to gray (decode.go, luma) ---------- */

/**
 * luma follows pageimage's luma: the ITU-R 601 weights 0.299, 0.587 and 0.114 in 16.16 fixed
 * point, on 16-bit channels (to16Bits multiplies by 0x101), keeping the high 8 bits.
 */
export function luma(red: number, green: number, blue: number): number {
  const red16 = red * 0x101;
  const green16 = green * 0x101;
  const blue16 = blue * 0x101;
  // The sum exceeds 2^32 for white, so it is computed in floating point and shifted by
  // division: exact, since every term is an integer below 2^53.
  return Math.floor((19595 * red16 + 38470 * green16 + 7471 * blue16 + 2 ** 15) / 2 ** 24);
}

/* ---------- Fit: the box filter (fit.go) ---------- */

/** A range of source rows or columns, from start included to end excluded. */
export type Span = {start: number; end: number};

/** fitSize follows pageimage's fitSize: the size of the image scaled to fit, at least 1×1. */
export function fitSize(
  srcWidth: number,
  srcHeight: number,
  width: number,
  height: number,
): [number, number] {
  let dstWidth = width;
  let dstHeight = Math.floor((srcHeight * width) / srcWidth);
  if (dstHeight > height) {
    dstWidth = Math.floor((srcWidth * height) / srcHeight);
    dstHeight = height;
  }
  return [Math.max(dstWidth, 1), Math.max(dstHeight, 1)];
}

/** boxSpan follows pageimage's boxSpan: the source rows (or columns) output index covers. */
export function boxSpan(index: number, srcLength: number, dstLength: number): Span {
  const start = Math.floor((index * srcLength) / dstLength);
  return {start, end: Math.max(Math.floor(((index + 1) * srcLength) / dstLength), start + 1)};
}

/**
 * fit follows pageimage's fit (and fitRef, the plain version its tests compare it with): each
 * output pixel is the average, rounded down, of the box of source pixels it covers. It also
 * returns the histogram of the result.
 */
export function fit(
  src: GrayImage,
  width: number,
  height: number,
): {image: GrayImage; histogram: number[]} {
  const histogram = new Array<number>(256).fill(0);
  const [dstWidth, dstHeight] = fitSize(src.width, src.height, width, height);
  const dst = newGray(dstWidth, dstHeight);
  const columnBoxes = Array.from({length: dstWidth}, (_, column) =>
    boxSpan(column, src.width, dstWidth),
  );
  for (let row = 0; row < dstHeight; row++) {
    const rowBox = boxSpan(row, src.height, dstHeight);
    for (let column = 0; column < dstWidth; column++) {
      const box = columnBoxes[column];
      let sum = 0;
      for (let y = rowBox.start; y < rowBox.end; y++) {
        for (let x = box.start; x < box.end; x++) sum += src.pixels[y * src.width + x];
      }
      const average = Math.floor(sum / ((rowBox.end - rowBox.start) * (box.end - box.start)));
      dst.pixels[row * dstWidth + column] = average;
      histogram[average]++;
    }
  }
  return {image: dst, histogram};
}

/* ---------- Tone (tone.go) ---------- */

/** clippedShare and gamma are tone.go's constants. */
export const clippedShare = 0.005;
export const gamma = 1.8;

export function histogramOf(image: GrayImage): number[] {
  const histogram = new Array<number>(256).fill(0);
  for (const level of image.pixels) histogram[level]++;
  return histogram;
}

/**
 * clipLevel follows pageimage's clipLevel: walking the grays in order, the first one at which
 * more than clip pixels have been counted.
 */
export function clipLevel(histogram: readonly number[], clip: number, darkToLight: boolean): number {
  let counted = 0;
  let last = 0;
  for (let index = 0; index < 256; index++) {
    const level = darkToLight ? index : 255 - index;
    counted += histogram[level];
    if (counted > clip) return level;
    last = level;
  }
  return last;
}

/**
 * toneTable follows pageimage's toneTable: stretch so that the darkest and lightest `share`
 * of the pixels become black and white, then apply `curveGamma`. The widgets let both vary;
 * the reader always uses clippedShare and gamma.
 */
export function toneTable(
  histogram: readonly number[],
  share = clippedShare,
  curveGamma = gamma,
): {tones: Uint8Array; darkest: number; lightest: number} {
  const pixelCount = histogram.reduce((total, count) => total + count, 0);
  const clip = Math.trunc(pixelCount * share);
  let darkest = clipLevel(histogram, clip, true);
  let lightest = clipLevel(histogram, clip, false);
  if (lightest <= darkest) {
    darkest = 0;
    lightest = 255;
  }
  const tones = new Uint8Array(256);
  for (let level = 0; level < 256; level++) {
    const normalized = Math.min(Math.max((level - darkest) / (lightest - darkest), 0), 1);
    tones[level] = goRound(255 * normalized ** curveGamma);
  }
  return {tones, darkest, lightest};
}

/** goRound is Go's math.Round: halves away from zero (Math.round rounds them up). */
function goRound(value: number): number {
  return Math.sign(value) * Math.round(Math.abs(value));
}

export function applyTones(image: GrayImage, tones: Uint8Array): GrayImage {
  const out = newGray(image.width, image.height);
  for (let index = 0; index < image.pixels.length; index++) {
    out.pixels[index] = tones[image.pixels[index]];
  }
  return out;
}

/* ---------- Dither (dither.go) and .pg4 (pg4.go) ---------- */

/** dither.go's Floyd–Steinberg weights, in sixteenths. */
export const errorToRight = 7;
export const errorToBelowLeft = 3;
export const errorToBelow = 5;
export const errorToBelowRight = 1;
export const errorDivisor = 16;

/** pg4.go's panelGrays and grayStep: 16 grays, 17 apart (255 / 15). */
export const panelGrays = 16;
export const grayStep = Math.floor(255 / (panelGrays - 1));

/** pg4.go's header: the magic, then width and height as little-endian uint16. */
export const pg4Magic = [0x50, 0x47, 0x34, 0x01]; // "PG4\x01"
export const pg4Header = 8;

/** nearestPanelGray follows dither.go's: the index of the panel gray nearest to value. */
export function nearestPanelGray(value: number): number {
  return Math.min(Math.max(Math.trunc((value + Math.floor(grayStep / 2)) / grayStep), 0), panelGrays - 1);
}

/** newPG4 follows pg4.go's: a .pg4 page of width×height, all black, and its row size. */
export function newPG4(width: number, height: number): {bytes: Uint8Array; rowBytes: number} {
  const rowBytes = Math.floor((width + 1) / 2);
  const bytes = new Uint8Array(pg4Header + rowBytes * height);
  bytes.set(pg4Magic, 0);
  bytes[4] = width & 0xff;
  bytes[5] = width >> 8;
  bytes[6] = height & 0xff;
  bytes[7] = height >> 8;
  return {bytes, rowBytes};
}

/**
 * ditherAndPack follows dither.go's: each pixel through tones, plus the error carried to it,
 * to the nearest panel gray; its error spread 7/16 right, 3/16 below left, 5/16 below and
 * 1/16 below right; rows always left to right (not serpentine). Errors are kept times 16 and
 * divided, truncating, when a pixel reads them.
 */
export function ditherAndPack(gray: GrayImage, tones?: Uint8Array): Uint8Array {
  const {width, height} = gray;
  const {bytes, rowBytes} = newPG4(width, height);
  let thisRow = new Int32Array(width + 2);
  let nextRow = new Int32Array(width + 2);
  for (let row = 0; row < height; row++) {
    const dst = pg4Header + row * rowBytes;
    for (let column = 0; column < width; column++) {
      const level = gray.pixels[row * width + column];
      const value = (tones ? tones[level] : level) + Math.trunc(thisRow[column + 1] / errorDivisor);
      const index = nearestPanelGray(value);
      if (column % 2 === 0) bytes[dst + column / 2] = index << 4;
      else bytes[dst + (column >> 1)] |= index;
      const pixelError = value - index * grayStep;
      thisRow[column + 2] += pixelError * errorToRight;
      nextRow[column] += pixelError * errorToBelowLeft;
      nextRow[column + 1] += pixelError * errorToBelow;
      nextRow[column + 2] += pixelError * errorToBelowRight;
    }
    [thisRow, nextRow] = [nextRow, thisRow];
    nextRow.fill(0);
  }
  return bytes;
}

/** unpack follows pg4.go's Unpack: a .pg4 page back to grays (index × grayStep). */
export function unpack(bytes: Uint8Array): GrayImage {
  const width = bytes[4] | (bytes[5] << 8);
  const height = bytes[6] | (bytes[7] << 8);
  const rowBytes = Math.floor((width + 1) / 2);
  const out = newGray(width, height);
  for (let row = 0; row < height; row++) {
    for (let column = 0; column < width; column++) {
      const packed = bytes[pg4Header + row * rowBytes + (column >> 1)];
      const index = column % 2 === 0 ? packed >> 4 : packed & 15;
      out.pixels[row * width + column] = index * grayStep;
    }
  }
  return out;
}

/* ---------- For the widgets: any number of levels, and a trace ---------- */

/**
 * quantize maps an image to `levels` evenly spaced grays, by plain rounding or by the same
 * Floyd–Steinberg as ditherAndPack. With 16 levels it gives exactly what ditherAndPack stores;
 * other counts only exist to show banding.
 */
export function quantize(image: GrayImage, levels: number, dither: boolean): GrayImage {
  const {width, height} = image;
  const step = 255 / (levels - 1);
  const half = Math.floor(step / 2);
  const out = newGray(width, height);
  let thisRow = new Float64Array(width + 2);
  let nextRow = new Float64Array(width + 2);
  for (let row = 0; row < height; row++) {
    for (let column = 0; column < width; column++) {
      const carried = dither ? Math.trunc(thisRow[column + 1] / errorDivisor) : 0;
      const value = image.pixels[row * width + column] + carried;
      const index = Math.min(Math.max(Math.trunc((value + half) / step), 0), levels - 1);
      out.pixels[row * width + column] = Math.round(index * step);
      if (!dither) continue;
      const pixelError = value - index * step;
      thisRow[column + 2] += pixelError * errorToRight;
      nextRow[column] += pixelError * errorToBelowLeft;
      nextRow[column + 1] += pixelError * errorToBelow;
      nextRow[column + 2] += pixelError * errorToBelowRight;
    }
    [thisRow, nextRow] = [nextRow, thisRow];
    nextRow.fill(0);
  }
  return out;
}

/** One pixel's visit in ditherTrace. */
export type DitherStep = {
  column: number;
  row: number;
  level: number; // the pixel's own gray
  carried: number; // the error carried to it, already divided by 16
  value: number; // level + carried
  index: number; // the panel gray chosen, 0..15
  pixelError: number; // value − index × grayStep
  /** The errors, times 16, waiting at every pixel of the image once this pixel is done. */
  pendingAfter: Int32Array;
};

/** ditherTrace runs ditherAndPack's loop on a small image and records every pixel's visit. */
export function ditherTrace(image: GrayImage): DitherStep[] {
  const {width, height} = image;
  const pending = new Int32Array(width * height);
  const steps: DitherStep[] = [];
  for (let row = 0; row < height; row++) {
    for (let column = 0; column < width; column++) {
      const at = row * width + column;
      const level = image.pixels[at];
      const carried = Math.trunc(pending[at] / errorDivisor);
      const value = level + carried;
      const index = nearestPanelGray(value);
      const pixelError = value - index * grayStep;
      const give = (x: number, y: number, weight: number) => {
        if (x >= 0 && x < width && y < height) pending[y * width + x] += pixelError * weight;
      };
      give(column + 1, row, errorToRight);
      give(column - 1, row + 1, errorToBelowLeft);
      give(column, row + 1, errorToBelow);
      give(column + 1, row + 1, errorToBelowRight);
      steps.push({column, row, level, carried, value, index, pixelError, pendingAfter: pending.slice()});
    }
  }
  return steps;
}
