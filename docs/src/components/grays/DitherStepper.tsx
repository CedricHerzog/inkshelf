// Floyd–Steinberg, one pixel at a time, as ditherAndPack does it: on a gentle 8×6 gradient,
// each pixel is rounded to the nearest of the 16 panel grays, and its error goes 7/16 to the
// right, 3/16 below left, 5/16 below and 1/16 below right. Rows always run left to right.

import React, {useMemo} from 'react';
import {Narration, StepperBar, useStepper} from '@site/src/components/kit';
import {
  ditherTrace,
  errorDivisor,
  errorToBelow,
  errorToBelowLeft,
  errorToBelowRight,
  errorToRight,
  grayStep,
  newGray,
  quantize,
} from '@site/src/lib/pageimage';
import styles from './grays.module.css';

const columns = 8;
const rows = 6;
const cell = 40;

/** A gentle gradient, 100 to 136: it spans only two steps of 17, so rounding bands it. */
const gradient = (() => {
  const image = newGray(columns, rows);
  for (let y = 0; y < rows; y++) {
    for (let x = 0; x < columns; x++) image.pixels[y * columns + x] = 100 + Math.round((x * 36) / (columns - 1));
  }
  return image;
})();

const neighbours = [
  {dx: 1, dy: 0, weight: errorToRight},
  {dx: -1, dy: 1, weight: errorToBelowLeft},
  {dx: 0, dy: 1, weight: errorToBelow},
  {dx: 1, dy: 1, weight: errorToBelowRight},
];

function gray(level: number) {
  const value = Math.max(0, Math.min(255, Math.round(level)));
  return `rgb(${value},${value},${value})`;
}

function signed(value: number) {
  return value > 0 ? `+${value}` : value < 0 ? `−${-value}` : '0';
}

function SmallGrid({levels, title}: {levels: ArrayLike<number>; title: string}) {
  const size = 18;
  return (
    <figure className={styles.canvasFigure}>
      <svg className={styles.svg} width={columns * size} viewBox={`0 0 ${columns * size} ${rows * size}`} role="img" aria-label={title}>
        {Array.from(levels).map((level, index) => (
          <rect key={index} x={(index % columns) * size} y={Math.floor(index / columns) * size} width={size} height={size} fill={gray(level)} />
        ))}
      </svg>
      <figcaption className={styles.canvasCaption}>{title}</figcaption>
    </figure>
  );
}

export default function DitherStepper() {
  const trace = useMemo(() => ditherTrace(gradient), []);
  const rounded = useMemo(() => quantize(gradient, 16, false), []);
  const stepper = useStepper(trace.length + 1, 650);
  const done = stepper.index; // pixels already drawn
  const current = done > 0 ? trace[done - 1] : null;
  const pending = current ? current.pendingAfter : new Int32Array(columns * rows);
  const center = (x: number, y: number) => ({cx: x * cell + cell / 2 + 10, cy: y * cell + cell / 2 + 10});
  const width = columns * cell + 20;
  const height = rows * cell + 20;

  return (
    <div>
      <svg className={styles.svg} width={width} viewBox={`0 0 ${width} ${height}`} style={{margin: '0 auto'}} role="img" aria-label="The gradient being dithered">
        <defs>
          <marker id="dither-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
            <path d="M0,0 L10,5 L0,10 z" fill="var(--ink-accent)" />
          </marker>
        </defs>
        {Array.from(gradient.pixels).map((level, index) => {
          const x = index % columns;
          const y = Math.floor(index / columns);
          const visited = index < done;
          const step = visited ? trace[index] : null;
          const carried = Math.trunc(pending[index] / errorDivisor);
          const fill = step ? step.index * grayStep : level;
          const isCurrent = current !== null && index === done - 1;
          return (
            <g key={index}>
              <rect x={x * cell + 10} y={y * cell + 10} width={cell - 1} height={cell - 1} fill={gray(fill)} />
              <text x={x * cell + 10 + cell / 2} y={y * cell + 10 + (visited ? cell / 2 : cell * 0.38)} textAnchor="middle" dominantBaseline="central" fontSize={10.5} fontFamily="var(--ink-mono)" fill={fill < 128 ? '#fff' : '#222'} fontWeight={visited ? 700 : 400}>
                {step ? step.index * grayStep : level}
              </text>
              {!visited && carried !== 0 && (
                <text x={x * cell + 10 + cell / 2} y={y * cell + 10 + cell * 0.72} textAnchor="middle" dominantBaseline="central" fontSize={9} fontFamily="var(--ink-mono)" fill="var(--ink-accent)" fontWeight={700}>
                  {signed(carried)}
                </text>
              )}
              {isCurrent && (
                <rect x={x * cell + 11.5} y={y * cell + 11.5} width={cell - 4} height={cell - 4} fill="none" stroke="var(--ink-accent)" strokeWidth={3} />
              )}
            </g>
          );
        })}
        {current &&
          current.pixelError !== 0 &&
          neighbours.map(({dx, dy, weight}) => {
            const x = current.column + dx;
            const y = current.row + dy;
            if (x < 0 || x >= columns || y >= rows) return null;
            const from = center(current.column, current.row);
            const to = center(x, y);
            const shorten = 0.42;
            const labelAt = {x: from.cx + (to.cx - from.cx) * 0.55, y: from.cy + (to.cy - from.cy) * 0.55};
            return (
              <g key={`${dx},${dy}`} pointerEvents="none">
                <line
                  x1={from.cx + (to.cx - from.cx) * 0.2}
                  y1={from.cy + (to.cy - from.cy) * 0.2}
                  x2={to.cx - (to.cx - from.cx) * shorten}
                  y2={to.cy - (to.cy - from.cy) * shorten}
                  stroke="var(--ink-accent)"
                  strokeWidth={2}
                  markerEnd="url(#dither-arrow)"
                />
                <rect x={labelAt.x - 13} y={labelAt.y - 7} width={26} height={13} rx={3} fill="var(--ink-paper)" stroke="var(--ink-accent)" strokeWidth={0.8} />
                <text x={labelAt.x} y={labelAt.y} textAnchor="middle" dominantBaseline="central" fontSize={8.5} fontFamily="var(--ink-mono)" fill="var(--ink-accent)">
                  {weight}/16
                </text>
              </g>
            );
          })}
      </svg>
      <StepperBar stepper={stepper} label="pixel" />
      <Narration>
        {current ? (
          <>
            Pixel ({current.column}, {current.row}): its gray <strong>{current.level}</strong>
            {current.carried !== 0 && (
              <>
                {' '}
                {current.carried > 0 ? '+' : '−'} {Math.abs(current.carried)} carried from pixels already drawn
              </>
            )}{' '}
            = {current.value}. Nearest panel gray: {current.index} × 17 = <strong>{current.index * grayStep}</strong>. Error{' '}
            {current.value} − {current.index * grayStep} = <strong>{signed(current.pixelError)}</strong>
            {current.pixelError === 0
              ? ': nothing to pass on.'
              : `: ${signed(current.pixelError)} × 7/16 goes right, × 3/16 below left, × 5/16 below, × 1/16 below right.`}
          </>
        ) : (
          <>
            A gradient from 100 to 136. Each cell shows its gray; red numbers are errors carried to
            it from pixels already drawn. Press play, or step with ›.
          </>
        )}
      </Narration>
      <div className={styles.row} style={{marginTop: '0.8rem'}}>
        <SmallGrid levels={gradient.pixels} title="the gradient" />
        <SmallGrid levels={rounded.pixels} title="plain rounding: bands" />
        <SmallGrid levels={trace.map((step) => step.index * grayStep)} title="Floyd–Steinberg: mixed" />
      </div>
    </div>
  );
}
