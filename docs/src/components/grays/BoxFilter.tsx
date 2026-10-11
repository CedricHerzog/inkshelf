// The box filter of pageimage's fit, on a 12×9 picture: each output pixel is the average of
// the box of source pixels it covers. Boxes come from boxSpan's integer division, so they are
// not all the same size. Picking one source pixel per box instead (nearest neighbour) shows
// why averaging matters: thin lines vanish.

import React, {useMemo, useState} from 'react';
import {Controls, Slider, Toggle} from '@site/src/components/kit';
import {boxSpan, fitSize, newGray} from '@site/src/lib/pageimage';
import styles from './grays.module.css';

const srcWidth = 12;
const srcHeight = 9;
const cell = 24;
const gap = 1;

/** A light page with a thin diagonal stroke, a thin vertical one and a small dark spot. */
const source = (() => {
  const image = newGray(srcWidth, srcHeight);
  for (let y = 0; y < srcHeight; y++) {
    for (let x = 0; x < srcWidth; x++) {
      let level = 224;
      if (x === y + 1) level = 20; // the diagonal
      if (x === 9 && y >= 1 && y <= 7) level = 40; // the vertical stroke
      if ((x === 4 || x === 5) && y === 7) level = 90; // a gray spot
      image.pixels[y * srcWidth + x] = level;
    }
  }
  return image;
})();

function grayFill(level: number) {
  return `rgb(${level},${level},${level})`;
}

export default function BoxFilter() {
  const [outWidth, setOutWidth] = useState(5);
  const [nearest, setNearest] = useState(false);
  const [selected, setSelected] = useState<{column: number; row: number}>({column: 1, row: 0});

  const [dstWidth, dstHeight] = fitSize(srcWidth, srcHeight, outWidth, 100);
  const result = useMemo(() => {
    const columns = Array.from({length: dstWidth}, (_, column) => boxSpan(column, srcWidth, dstWidth));
    const rows = Array.from({length: dstHeight}, (_, row) => boxSpan(row, srcHeight, dstHeight));
    const out: {level: number; sum: number; area: number}[] = [];
    for (const rowBox of rows) {
      for (const columnBox of columns) {
        let sum = 0;
        for (let y = rowBox.start; y < rowBox.end; y++) {
          for (let x = columnBox.start; x < columnBox.end; x++) sum += source.pixels[y * srcWidth + x];
        }
        const area = (rowBox.end - rowBox.start) * (columnBox.end - columnBox.start);
        const level = nearest ? source.pixels[rowBox.start * srcWidth + columnBox.start] : Math.floor(sum / area);
        out.push({level, sum, area});
      }
    }
    return {columns, rows, out};
  }, [dstWidth, dstHeight, nearest]);

  const column = Math.min(selected.column, dstWidth - 1);
  const row = Math.min(selected.row, dstHeight - 1);
  const columnBox = result.columns[column];
  const rowBox = result.rows[row];
  const chosen = result.out[row * dstWidth + column];

  const srcPixelWidth = srcWidth * (cell + gap) + gap;
  const srcPixelHeight = srcHeight * (cell + gap) + gap;
  const outCell = (srcPixelWidth - gap) / dstWidth;
  const outHeight = outCell * dstHeight;
  const at = (index: number) => gap + index * (cell + gap);

  return (
    <div>
      <div className={styles.row}>
        <figure className={styles.canvasFigure}>
          <svg
            className={styles.svg}
            width={srcPixelWidth}
            viewBox={`0 0 ${srcPixelWidth} ${srcPixelHeight}`}
            role="img"
            aria-label="The 12 by 9 source pixels, with the boxes of the output pixels">
            <rect width={srcPixelWidth} height={srcPixelHeight} fill="var(--ink-line)" />
            {Array.from(source.pixels).map((level, index) => {
              const x = index % srcWidth;
              const y = Math.floor(index / srcWidth);
              const inBox = x >= columnBox.start && x < columnBox.end && y >= rowBox.start && y < rowBox.end;
              const picked = nearest && x === columnBox.start && y === rowBox.start;
              return (
                <g key={index}>
                  <rect x={at(x)} y={at(y)} width={cell} height={cell} fill={grayFill(level)} opacity={inBox ? 1 : 0.55} />
                  <text x={at(x) + cell / 2} y={at(y) + cell / 2} textAnchor="middle" dominantBaseline="central" fontSize={8} fontFamily="var(--ink-mono)" fill={level < 128 ? '#fff' : '#555'}>
                    {inBox ? level : ''}
                  </text>
                  {picked && <circle cx={at(x) + cell / 2} cy={at(y) + cell / 2} r={cell / 2 - 1} fill="none" stroke="var(--ink-accent)" strokeWidth={2} />}
                </g>
              );
            })}
            {/* The box boundaries: where boxSpan cuts the source. */}
            {result.columns.slice(1).map((box) => (
              <line key={`c${box.start}`} x1={at(box.start) - gap / 2} x2={at(box.start) - gap / 2} y1={0} y2={srcPixelHeight} stroke="var(--ink-blue)" strokeWidth={1.5} />
            ))}
            {result.rows.slice(1).map((box) => (
              <line key={`r${box.start}`} y1={at(box.start) - gap / 2} y2={at(box.start) - gap / 2} x1={0} x2={srcPixelWidth} stroke="var(--ink-blue)" strokeWidth={1.5} />
            ))}
            <rect
              x={at(columnBox.start) - 1}
              y={at(rowBox.start) - 1}
              width={(columnBox.end - columnBox.start) * (cell + gap) + 1}
              height={(rowBox.end - rowBox.start) * (cell + gap) + 1}
              fill="none"
              stroke="var(--ink-accent)"
              strokeWidth={3}
            />
          </svg>
          <figcaption className={styles.canvasCaption}>source: 12×9</figcaption>
        </figure>
        <figure className={styles.canvasFigure}>
          <svg
            className={styles.svg}
            width={srcPixelWidth}
            viewBox={`0 0 ${srcPixelWidth} ${outHeight + 2}`}
            role="img"
            aria-label={`The ${dstWidth} by ${dstHeight} output pixels`}>
            {result.out.map((pixel, index) => {
              const x = index % dstWidth;
              const y = Math.floor(index / dstWidth);
              const isSelected = x === column && y === row;
              return (
                <g
                  key={index}
                  onMouseEnter={() => setSelected({column: x, row: y})}
                  onClick={() => setSelected({column: x, row: y})}
                  style={{cursor: 'pointer'}}>
                  <rect x={x * outCell + 1} y={y * outCell + 1} width={outCell - 1} height={outCell - 1} fill={grayFill(pixel.level)} stroke="var(--ink-line)" strokeWidth={1} />
                  <text x={x * outCell + outCell / 2} y={y * outCell + outCell / 2} textAnchor="middle" dominantBaseline="central" fontSize={Math.min(14, outCell * 0.3)} fontFamily="var(--ink-mono)" fill={pixel.level < 128 ? '#fff' : '#333'}>
                    {pixel.level}
                  </text>
                  {isSelected && (
                    <rect x={x * outCell + 2.5} y={y * outCell + 2.5} width={outCell - 4} height={outCell - 4} fill="none" stroke="var(--ink-accent)" strokeWidth={3} />
                  )}
                </g>
              );
            })}
          </svg>
          <figcaption className={styles.canvasCaption}>
            output: {dstWidth}×{dstHeight}
          </figcaption>
        </figure>
      </div>
      <p className={styles.formula}>
        {nearest ? (
          <>
            output ({column}, {row}) = the box's top-left pixel = <strong>{chosen.level}</strong>
          </>
        ) : (
          <>
            output ({column}, {row}) = {chosen.sum} ÷ {chosen.area} pixels = <strong>{chosen.level}</strong>
            <br />
            columns {columnBox.start}–{columnBox.end - 1}, rows {rowBox.start}–{rowBox.end - 1}
          </>
        )}
      </p>
      <Controls>
        <Slider label="Output width" value={outWidth} min={2} max={12} onChange={setOutWidth} format={(value) => `${value} px`} />
        <Toggle label="Pick one pixel instead of averaging" checked={nearest} onChange={setNearest} />
      </Controls>
    </div>
  );
}
