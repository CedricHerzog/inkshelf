// The rotation between the app's portrait canvas and the reader's landscape framebuffer, and
// the 4x4 block transpose that copies it a word at a time. Follows framebufferRect,
// copyTileWords, interleaveBytes and interleaveHalves of
// internal/device/pocketbook/framebuffer_copy.go.

/** The InkPad 3's framebuffer: 1872 columns by 1404 rows (the screen, turned a quarter). */
export const FRAMEBUFFER_WIDTH = 1872;
export const FRAMEBUFFER_HEIGHT = 1404;

/**
 * framebufferPosition returns where screen pixel (x, y) lands in a framebuffer of
 * `framebufferHeight` rows: column y, row framebufferHeight - 1 - x.
 */
export function framebufferPosition(
  x: number,
  y: number,
  framebufferHeight: number = FRAMEBUFFER_HEIGHT,
): {column: number; row: number} {
  return {column: y, row: framebufferHeight - 1 - x};
}

// The masks of framebuffer_copy.go, bytes numbered from the low one, 0, to the high one, 3.
export const BYTES_0_AND_2 = 0x00ff00ff;
export const BYTES_1_AND_3 = 0xff00ff00;
export const BYTES_0_AND_1 = 0x0000ffff;
export const BYTES_2_AND_3 = 0xffff0000;

/** u32 keeps a JavaScript number as an unsigned 32-bit word. */
const u32 = (value: number) => value >>> 0;

/** interleaveBytes: from a b c d and e f g h, returns a e c g (even) and b f d h (odd). */
export function interleaveBytes(first: number, second: number): [number, number] {
  const even = u32((first & BYTES_0_AND_2) | ((second << 8) & BYTES_1_AND_3));
  const odd = u32(((first >>> 8) & BYTES_0_AND_2) | (second & BYTES_1_AND_3));
  return [even, odd];
}

/** interleaveHalves: from a e c g and i m k o, returns a e i m (low) and c g k o (high). */
export function interleaveHalves(first: number, second: number): [number, number] {
  const low = u32((first & BYTES_0_AND_1) | (second << 16));
  const high = u32((first >>> 16) | (second & BYTES_2_AND_3));
  return [low, high];
}

/** wordFromBytes packs four bytes into a little-endian word: bytes[0] is the low byte. */
export function wordFromBytes(bytes: readonly number[]): number {
  return u32(bytes[0] | (bytes[1] << 8) | (bytes[2] << 16) | (bytes[3] << 24));
}

/** bytesOfWord unpacks a little-endian word: the first byte returned is the low one. */
export function bytesOfWord(word: number): number[] {
  return [word & 0xff, (word >>> 8) & 0xff, (word >>> 16) & 0xff, (word >>> 24) & 0xff];
}

/** hexWord writes a word as Go would print it with %#08x. */
export function hexWord(word: number): string {
  return `0x${u32(word).toString(16).padStart(8, '0')}`;
}

/** The words of one block's transpose, as copyTileWords computes them. */
export type BlockTranspose = {
  rows: [number, number, number, number];
  even01: number;
  odd01: number;
  even23: number;
  odd23: number;
  column0: number;
  column1: number;
  column2: number;
  column3: number;
};

/** transposeBlock runs copyTileWords' transpose on the four row words of a 4x4 block. */
export function transposeBlock(rows: [number, number, number, number]): BlockTranspose {
  const [even01, odd01] = interleaveBytes(rows[0], rows[1]);
  const [even23, odd23] = interleaveBytes(rows[2], rows[3]);
  const [column0, column2] = interleaveHalves(even01, even23);
  const [column1, column3] = interleaveHalves(odd01, odd23);
  return {rows, even01, odd01, even23, odd23, column0, column1, column2, column3};
}
