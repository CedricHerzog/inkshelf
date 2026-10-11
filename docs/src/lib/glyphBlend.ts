// A glyph rasterized into an alpha mask and blended into a gray page, as internal/graphics/fonts
// does: the mask says how much of each pixel the outline covers (here by sampling each pixel
// 8 × 8 times, as a rasterizer measures area), and blend (draw.go) moves each covered pixel from
// its gray toward the ink's in proportion: (page × (255 − alpha) + ink × alpha + 127) / 255.

export const maskColumns = 9;
export const maskRows = 11;

/**
 * insideO reports whether a point (in pixels) is inside a letter "o" drawn as an elliptical
 * ring, its left edge at offsetX: the outline, which knows no pixels.
 */
function insideO(x: number, y: number, offsetX: number): boolean {
  const centerX = 4 + offsetX;
  const centerY = 6.5;
  const outer = ((x - centerX) / 3.6) ** 2 + ((y - centerY) / 3.6) ** 2;
  const inner = ((x - centerX) / 2.0) ** 2 + ((y - centerY) / 2.2) ** 2;
  return outer <= 1 && inner > 1;
}

/** rasterize returns the alpha mask (0–255, row by row) of the "o" shifted by offsetX pixels. */
export function rasterize(offsetX: number): number[] {
  const samples = 8;
  const mask: number[] = [];
  for (let row = 0; row < maskRows; row++) {
    for (let column = 0; column < maskColumns; column++) {
      let covered = 0;
      for (let sampleY = 0; sampleY < samples; sampleY++) {
        for (let sampleX = 0; sampleX < samples; sampleX++) {
          const x = column + (sampleX + 0.5) / samples;
          const y = row + (sampleY + 0.5) / samples;
          if (insideO(x, y, offsetX)) covered++;
        }
      }
      mask.push(Math.round((covered * 255) / (samples * samples)));
    }
  }
  return mask;
}

/** blendPixel is one pixel of fonts.blend: page gray moved toward ink by alpha, rounded. */
export function blendPixel(page: number, ink: number, alpha: number): number {
  if (alpha === 0) return page;
  return Math.floor((page * (255 - alpha) + ink * alpha + 127) / 255);
}
