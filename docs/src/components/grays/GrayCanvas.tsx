// A canvas that shows a gray (or color) image, each image pixel as a crisp square: the widgets
// of this chapter are about pixels, so the browser must not smooth them.

import React, {useEffect, useRef} from 'react';
import type {GrayImage} from '@site/src/lib/pageimage';
import {drawColor, drawGray, type ColorImage} from '@site/src/lib/sampleImages';
import styles from './grays.module.css';

export default function GrayCanvas({
  image,
  color,
  displayWidth,
  label,
  caption,
}: {
  image?: GrayImage;
  color?: ColorImage;
  /** The width on the page, in CSS pixels; the height follows the image's shape. */
  displayWidth: number;
  label: string;
  caption?: React.ReactNode;
}) {
  const canvas = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    if (!canvas.current) return;
    if (image) drawGray(canvas.current, image);
    else if (color) drawColor(canvas.current, color);
  }, [image, color]);
  const source = image ?? color;
  const ratio = source ? source.height / source.width : 1;
  return (
    <figure className={styles.canvasFigure}>
      <canvas
        ref={canvas}
        className={styles.pixelated}
        style={{width: displayWidth, maxWidth: '100%', aspectRatio: `${1} / ${ratio}`}}
        aria-label={label}
      />
      {caption && <figcaption className={styles.canvasCaption}>{caption}</figcaption>}
    </figure>
  );
}
