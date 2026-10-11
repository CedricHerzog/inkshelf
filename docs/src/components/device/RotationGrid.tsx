// Where a canvas pixel lands in the framebuffer: screen (x, y) is framebuffer column y, row
// framebufferHeight-1-x (framebuffer_copy.go). Shown on a 4x6 canvas, then with the InkPad 3's
// real sizes.

import React, {useState} from 'react';
import {PixelGrid, Slider} from '@site/src/components/kit';
import {FRAMEBUFFER_HEIGHT, FRAMEBUFFER_WIDTH, framebufferPosition} from '@site/src/lib/framebufferRotation';
import styles from './device.module.css';

const WIDTH = 4;
const HEIGHT = 6;
// The framebuffer of this small canvas: HEIGHT columns, WIDTH rows.
const FB_COLUMNS = HEIGHT;
const FB_ROWS = WIDTH;

const letters = 'abcdefghijklmnopqrstuvwx'.split('');

// A soft gray per canvas row, so the rows stay recognisable once turned.
const rowGray = (y: number) => 255 - y * 18;

export default function RotationGrid() {
  const [picked, setPicked] = useState<number>(1); // b
  const [hovered, setHovered] = useState<number | null>(null);
  const [realX, setRealX] = useState(100);
  const [realY, setRealY] = useState(300);

  const shown = hovered ?? picked;
  const x = shown % WIDTH;
  const y = Math.floor(shown / WIDTH);
  const target = framebufferPosition(x, y, FB_ROWS);

  const canvasCells = letters.map((_, index) => rowGray(Math.floor(index / WIDTH)));
  const fbCells = new Array<number>(FB_COLUMNS * FB_ROWS).fill(255);
  const fbLabels = new Array<string | undefined>(FB_COLUMNS * FB_ROWS);
  letters.forEach((letter, index) => {
    const position = framebufferPosition(index % WIDTH, Math.floor(index / WIDTH), FB_ROWS);
    const fbIndex = position.row * FB_COLUMNS + position.column;
    fbCells[fbIndex] = rowGray(Math.floor(index / WIDTH));
    fbLabels[fbIndex] = letter;
  });
  const fbIndex = target.row * FB_COLUMNS + target.column;

  // The canvas index of a framebuffer cell, for hovering the framebuffer too.
  const canvasIndexOf = (index: number) => {
    const column = index % FB_COLUMNS;
    const row = Math.floor(index / FB_COLUMNS);
    return column * WIDTH + (FB_ROWS - 1 - row);
  };

  const real = framebufferPosition(realX, realY);

  return (
    <div className={styles.stack}>
      <div className={styles.row} style={{alignItems: 'center'}}>
        <div>
          <div className={styles.label}>Canvas, as the app draws (4 × 6)</div>
          <PixelGrid
            columns={WIDTH}
            cells={canvasCells}
            labels={letters}
            cellSize={32}
            highlight={new Set([shown])}
            onCellClick={setPicked}
            onCellHover={setHovered}
            ariaLabel="A 4 by 6 canvas, lettered a to x"
          />
        </div>
        <div className={styles.arrow}>⟳ a quarter turn →</div>
        <div>
          <div className={styles.label}>Framebuffer, as the panel reads (6 × 4)</div>
          <PixelGrid
            columns={FB_COLUMNS}
            cells={fbCells}
            labels={fbLabels}
            cellSize={32}
            highlight={new Set([fbIndex])}
            onCellClick={(index) => setPicked(canvasIndexOf(index))}
            onCellHover={(index) => setHovered(index === null ? null : canvasIndexOf(index))}
            ariaLabel="The framebuffer of that canvas"
          />
        </div>
      </div>
      <div className={styles.code}>
        {`canvas (x ${x}, y ${y})  →  framebuffer column y = ${target.column}, row (${FB_ROWS} − 1) − x = ${target.row}`}
      </div>
      <p className={styles.small} style={{margin: 0}}>
        The canvas’s top row <strong>a b c d</strong> becomes the framebuffer’s left column, read from the bottom up.
        Each canvas <em>row</em> becomes a framebuffer <em>column</em>.
      </p>
      <div className={styles.label}>The same rule on the InkPad 3 (canvas 1404 × 1872, framebuffer {FRAMEBUFFER_WIDTH} × {FRAMEBUFFER_HEIGHT})</div>
      <Slider label="x" value={realX} min={0} max={1403} onChange={setRealX} />
      <Slider label="y" value={realY} min={0} max={1871} onChange={setRealY} />
      <div className={styles.code}>
        {`screen (${realX}, ${realY})  →  framebuffer column ${real.column}, row 1403 − ${realX} = ${real.row}`}
      </div>
    </div>
  );
}
