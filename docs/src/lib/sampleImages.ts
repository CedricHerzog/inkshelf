// Procedural sample images for the image-pipeline widgets: a faded comic "scan", a smooth
// shaded picture and a colored page. Drawn pixel by pixel from a seeded random generator, so
// the site needs no image files and every visitor sees the same pictures.

import {newGray, type GrayImage} from './pageimage';

/** mulberry32 is a tiny seeded random generator: the same seed gives the same sequence. */
export function seededRandom(seed: number): () => number {
  let state = seed >>> 0;
  return () => {
    state = (state + 0x6d2b79f5) >>> 0;
    let mixed = state;
    mixed = Math.imul(mixed ^ (mixed >>> 15), mixed | 1);
    mixed ^= mixed + Math.imul(mixed ^ (mixed >>> 7), mixed | 61);
    return ((mixed ^ (mixed >>> 14)) >>> 0) / 4294967296;
  };
}

/** A shape the drawings are made of: how much ink covers point (x, y), from 0 to 1. */
type Coverage = (x: number, y: number) => number;

function clamp01(value: number): number {
  return Math.min(1, Math.max(0, value));
}

/** A stroke `thickness` wide along the segment from (x1, y1) to (x2, y2). */
function segment(x1: number, y1: number, x2: number, y2: number, thickness: number): Coverage {
  const dx = x2 - x1;
  const dy = y2 - y1;
  const lengthSquared = dx * dx + dy * dy || 1;
  return (x, y) => {
    const along = clamp01(((x - x1) * dx + (y - y1) * dy) / lengthSquared);
    const distance = Math.hypot(x - (x1 + along * dx), y - (y1 + along * dy));
    return clamp01(thickness / 2 + 0.5 - distance);
  };
}

/** The outline of an ellipse, `thickness` wide. */
function ellipseOutline(cx: number, cy: number, rx: number, ry: number, thickness: number): Coverage {
  return (x, y) => {
    const normalized = Math.hypot((x - cx) / rx, (y - cy) / ry);
    const distance = Math.abs(normalized - 1) * Math.min(rx, ry);
    return clamp01(thickness / 2 + 0.5 - distance);
  };
}

/** The outline of a rectangle, `thickness` wide. */
function rectangleOutline(left: number, top: number, right: number, bottom: number, thickness: number): Coverage {
  const sides = [
    segment(left, top, right, top, thickness),
    segment(right, top, right, bottom, thickness),
    segment(right, bottom, left, bottom, thickness),
    segment(left, bottom, left, top, thickness),
  ];
  return (x, y) => Math.max(...sides.map((side) => side(x, y)));
}

/**
 * makeScan draws a small comic panel as an old scan would show it: yellowed, uneven paper
 * (light gray, never white), faded ink (dark gray, never black), screentone dots and a few
 * specks of dust, both dark and bright. Its grays sit in a narrow band in the middle.
 */
export function makeScan(width: number, height: number, seed = 7): GrayImage {
  const random = seededRandom(seed);
  const image = newGray(width, height);
  const scale = width / 240;
  const shapes: Coverage[] = [
    rectangleOutline(8 * scale, 8 * scale, width - 8 * scale, height - 8 * scale, 2.5 * scale),
    ellipseOutline(70 * scale, 100 * scale, 34 * scale, 40 * scale, 2.2 * scale), // a head
    segment(56 * scale, 92 * scale, 62 * scale, 92 * scale, 2.4 * scale), // eyes
    segment(78 * scale, 92 * scale, 84 * scale, 92 * scale, 2.4 * scale),
    segment(58 * scale, 118 * scale, 82 * scale, 114 * scale, 1.8 * scale), // mouth
    ellipseOutline(168 * scale, 52 * scale, 56 * scale, 30 * scale, 2 * scale), // a speech bubble
    segment(130 * scale, 72 * scale, 106 * scale, 92 * scale, 2 * scale),
    segment(136 * scale, 44 * scale, 196 * scale, 44 * scale, 3 * scale), // its words
    segment(132 * scale, 54 * scale, 204 * scale, 54 * scale, 3 * scale),
    segment(140 * scale, 64 * scale, 186 * scale, 64 * scale, 3 * scale),
  ];
  // Hatching: diagonal strokes in the lower right corner.
  for (let index = 0; index < 9; index++) {
    const offset = (150 + index * 9) * scale;
    shapes.push(segment(offset, height - 14 * scale, offset + 26 * scale, height - 52 * scale, 1.4 * scale));
  }
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      // Paper: light gray, a little darker towards the bottom, with grain.
      const paper = 200 - (y / height) * 14 + (random() - 0.5) * 10;
      const ink = 82 + (random() - 0.5) * 16;
      let coverage = 0;
      for (const shape of shapes) coverage = Math.max(coverage, shape(x, y));
      // Screentone: a grid of dots on the left of the floor, a mid gray when seen from afar.
      if (y > height * 0.72 && x < width * 0.5 && x > 10 * scale && y < height - 10 * scale) {
        const cell = 4 * scale;
        const dx = (x % cell) - cell / 2;
        const dy = (y % cell) - cell / 2;
        coverage = Math.max(coverage, clamp01(cell * 0.32 + 0.5 - Math.hypot(dx, dy)));
      }
      image.pixels[y * width + x] = Math.round(paper + (ink - paper) * coverage);
    }
  }
  // Dust: a few dark specks and bright scratches.
  for (let speck = 0; speck < 6; speck++) {
    const x = Math.floor(random() * width);
    const y = Math.floor(random() * height);
    const bright = speck % 2 === 1;
    for (let dy = 0; dy < 2; dy++) {
      for (let dx = 0; dx < 2; dx++) {
        if (x + dx < width && y + dy < height) image.pixels[(y + dy) * width + x + dx] = bright ? 255 : 0;
      }
    }
  }
  return image;
}

/**
 * makeShaded draws smooth tones, where rounding to few grays shows bands: a gradient from
 * black to white along the top, and a shaded ball on a soft background.
 */
export function makeShaded(width: number, height: number): GrayImage {
  const image = newGray(width, height);
  const band = Math.round(height * 0.28);
  const radius = height * 0.3;
  const cx = width * 0.5;
  const cy = band + (height - band) / 2;
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      let level: number;
      if (y < band) {
        level = (x / (width - 1)) * 255;
      } else {
        level = 150 + 70 * (y / height) - 40 * (x / width); // the background
        const dx = (x - cx) / radius;
        const dy = (y - cy) / radius;
        const inside = 1 - dx * dx - dy * dy;
        if (inside >= 0) {
          // A ball lit from the upper left: Lambert shading of its surface normal.
          const dz = Math.sqrt(inside);
          const light = Math.max(0, -0.5 * dx - 0.55 * dy + 0.67 * dz);
          level = 25 + 225 * light;
        }
      }
      image.pixels[y * width + x] = Math.round(level);
    }
  }
  return image;
}

/** An RGB image, three bytes a pixel. */
export type ColorImage = {width: number; height: number; rgb: Uint8ClampedArray};

/**
 * makeColorPage draws a colored comic panel as a server sends it: a sky, a sun, grass and a
 * red roof, at a size larger than the screen it will be fitted to.
 */
export function makeColorPage(width: number, height: number): ColorImage {
  const rgb = new Uint8ClampedArray(width * height * 3);
  const random = seededRandom(3);
  const roof = (x: number, y: number) => {
    const peakX = width * 0.36;
    const peakY = height * 0.42;
    const baseY = height * 0.58;
    if (y < peakY || y > baseY) return false;
    const halfWidth = ((y - peakY) / (baseY - peakY)) * width * 0.2;
    return Math.abs(x - peakX) <= halfWidth;
  };
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      let color: [number, number, number];
      const sun = Math.hypot(x - width * 0.78, y - height * 0.2) < width * 0.1;
      if (sun) color = [250, 205, 60];
      else if (roof(x, y)) color = [190, 50, 40];
      else if (y > height * 0.58 && y < height * 0.8 && Math.abs(x - width * 0.36) < width * 0.15)
        color = [235, 225, 200]; // the wall
      else if (y > height * 0.68) color = [70, 150, 60];
      else {
        const sky = y / (height * 0.68);
        color = [110 + 100 * sky, 160 + 70 * sky, 235];
      }
      const grain = (random() - 0.5) * 12;
      const at = (y * width + x) * 3;
      rgb[at] = color[0] + grain;
      rgb[at + 1] = color[1] + grain;
      rgb[at + 2] = color[2] + grain;
    }
  }
  return {width, height, rgb};
}

/** drawGray paints a gray image on a canvas, one canvas pixel per image pixel. */
export function drawGray(canvas: HTMLCanvasElement, image: GrayImage) {
  canvas.width = image.width;
  canvas.height = image.height;
  const context = canvas.getContext('2d');
  if (!context) return;
  const data = context.createImageData(image.width, image.height);
  for (let index = 0; index < image.pixels.length; index++) {
    const level = image.pixels[index];
    data.data[index * 4] = level;
    data.data[index * 4 + 1] = level;
    data.data[index * 4 + 2] = level;
    data.data[index * 4 + 3] = 255;
  }
  context.putImageData(data, 0, 0);
}

/** drawColor paints an RGB image on a canvas. */
export function drawColor(canvas: HTMLCanvasElement, image: ColorImage) {
  canvas.width = image.width;
  canvas.height = image.height;
  const context = canvas.getContext('2d');
  if (!context) return;
  const data = context.createImageData(image.width, image.height);
  for (let index = 0; index < image.width * image.height; index++) {
    data.data[index * 4] = image.rgb[index * 3];
    data.data[index * 4 + 1] = image.rgb[index * 3 + 1];
    data.data[index * 4 + 2] = image.rgb[index * 3 + 2];
    data.data[index * 4 + 3] = 255;
  }
  context.putImageData(data, 0, 0);
}
