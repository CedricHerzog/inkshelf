// Two refreshes of the panel, as rectangles you can drag, and a timeline of what the reader
// does with them. readerFirmware.flush waits for the previous refresh only when its rectangle
// overlaps the new one (waitForOverlappingRefresh); otherwise both refreshes run at once.

import React, {useRef, useState} from 'react';
import {
  Button,
  Controls,
  Segmented,
  Slider,
  useAnimationFrame,
} from '@site/src/components/kit';
import {refreshMs, scheduleTwo, type Rect} from '@site/src/lib/einkPanel';
import styles from './eink.module.css';

const screenWidth = 1404;
const screenHeight = 1872;
const toolbarHeight = 168; // widgets.BarHeight, in InkPad 3 pixels

type Which = 'first' | 'second';
type Preset = 'toolbar' | 'corners' | 'custom';

const presets: Record<Exclude<Preset, 'custom'>, {first: Rect; second: Rect}> = {
  // A page turn refreshes the page; then the toolbar is opened over it.
  toolbar: {
    first: {minX: 0, minY: 0, maxX: screenWidth, maxY: screenHeight},
    second: {minX: 0, minY: 0, maxX: screenWidth, maxY: toolbarHeight},
  },
  corners: {
    first: {minX: 80, minY: 120, maxX: 640, maxY: 760},
    second: {minX: 760, minY: 1100, maxX: 1320, maxY: 1760},
  },
};

const colors: Record<Which, {stroke: string; fill: string}> = {
  first: {stroke: 'var(--ink-blue)', fill: 'var(--ink-blue-soft)'},
  second: {stroke: 'var(--ink-accent)', fill: 'var(--ink-accent-soft)'},
};

/* The timeline's scale. */
const timelineMs = 1300;
const axisLeft = 70;
const axisRight = 340;
const toX = (ms: number) => axisLeft + (Math.min(ms, timelineMs) / timelineMs) * (axisRight - axisLeft);

export default function RefreshOverlap() {
  const [preset, setPreset] = useState<Preset>('toolbar');
  const [rects, setRects] = useState(presets.toolbar);
  const [requestedAt, setRequestedAt] = useState(60);
  const [secondKind, setSecondKind] = useState<'partial' | 'fast'>('partial');
  const [playhead, setPlayhead] = useState<number | null>(null);
  const svg = useRef<SVGSVGElement>(null);
  const drag = useRef<{which: Which; startX: number; startY: number; rect: Rect} | null>(null);

  const firstMs = refreshMs.partial;
  const secondMs = secondKind === 'partial' ? refreshMs.partial : refreshMs.fast;
  const plan = scheduleTwo(rects.first, rects.second, requestedAt, firstMs, secondMs);

  useAnimationFrame((elapsedMs) => {
    setPlayhead((current) => {
      if (current === null) return null;
      const next = current + elapsedMs * 0.5; // half speed, to follow
      return next > plan.allDone + 150 ? null : next;
    });
  }, playhead !== null);

  const toSvg = (clientX: number, clientY: number) => {
    const matrix = svg.current?.getScreenCTM();
    if (!matrix) return {x: 0, y: 0};
    const point = new DOMPoint(clientX, clientY).matrixTransform(matrix.inverse());
    return {x: point.x, y: point.y};
  };

  const onPointerDown = (which: Which) => (event: React.PointerEvent) => {
    const {x, y} = toSvg(event.clientX, event.clientY);
    drag.current = {which, startX: x, startY: y, rect: rects[which]};
    svg.current?.setPointerCapture(event.pointerId);
  };

  const onPointerMove = (event: React.PointerEvent) => {
    const current = drag.current;
    if (!current) return;
    const {x, y} = toSvg(event.clientX, event.clientY);
    const rectWidth = current.rect.maxX - current.rect.minX;
    const rectHeight = current.rect.maxY - current.rect.minY;
    const minX = Math.round(Math.min(Math.max(current.rect.minX + x - current.startX, 0), screenWidth - rectWidth));
    const minY = Math.round(Math.min(Math.max(current.rect.minY + y - current.startY, 0), screenHeight - rectHeight));
    setPreset('custom');
    setRects((all) => ({...all, [current.which]: {minX, minY, maxX: minX + rectWidth, maxY: minY + rectHeight}}));
  };

  const endDrag = () => {
    drag.current = null;
  };

  const choosePreset = (value: Preset) => {
    setPreset(value);
    if (value !== 'custom') setRects(presets[value]);
  };

  /** How far (0 to 1) a refresh has gone at the playhead, or null when it is not running. */
  const progress = (startMs: number, durationMs: number) => {
    if (playhead === null || playhead < startMs) return null;
    return Math.min(1, (playhead - startMs) / durationMs);
  };

  const drawRect = (which: Which, rect: Rect, fraction: number | null, waiting: boolean) => (
    <g key={which} onPointerDown={onPointerDown(which)} style={{cursor: 'grab'}}>
      <rect
        x={rect.minX}
        y={rect.minY}
        width={rect.maxX - rect.minX}
        height={rect.maxY - rect.minY}
        fill={colors[which].fill}
        stroke={colors[which].stroke}
        strokeWidth={14}
        strokeDasharray={waiting ? '40 30' : undefined}
      />
      {fraction !== null && fraction < 1 && (
        // The refresh sweeping over its rectangle.
        <rect
          x={rect.minX}
          y={rect.minY}
          width={(rect.maxX - rect.minX) * fraction}
          height={rect.maxY - rect.minY}
          fill={colors[which].stroke}
          opacity={0.35}
          pointerEvents="none"
        />
      )}
      <text
        x={(rect.minX + rect.maxX) / 2}
        y={(rect.minY + rect.maxY) / 2}
        textAnchor="middle"
        dominantBaseline="central"
        fontSize={which === 'second' && rect.maxY - rect.minY < 200 ? 110 : 150}
        fontWeight={700}
        fill={colors[which].stroke}
        pointerEvents="none">
        {which === 'first' ? '1' : '2'}
        {waiting ? ' waits' : ''}
      </text>
    </g>
  );

  const secondWaiting =
    playhead !== null && plan.overlap && playhead >= requestedAt && playhead < plan.secondStart;

  return (
    <div>
      <Controls>
        <Segmented
          options={[
            {value: 'toolbar', label: 'Page, then toolbar'},
            {value: 'corners', label: 'Two corners'},
            {value: 'custom', label: 'Dragged'},
          ]}
          value={preset}
          onChange={choosePreset}
        />
      </Controls>
      <div className={styles.stage}>
        <svg
          ref={svg}
          viewBox={`-20 -20 ${screenWidth + 40} ${screenHeight + 40}`}
          style={{width: 170, height: 'auto', display: 'block', touchAction: 'none', userSelect: 'none'}}
          onPointerMove={onPointerMove}
          onPointerUp={endDrag}
          onPointerCancel={endDrag}
          role="img"
          aria-label="The panel with two refresh rectangles you can drag">
          <rect x={-20} y={-20} width={screenWidth + 40} height={screenHeight + 40} rx={60} fill="#3a3835" />
          <rect x={0} y={0} width={screenWidth} height={screenHeight} fill="var(--ink-paper)" />
          {drawRect('first', rects.first, progress(0, firstMs), false)}
          {drawRect('second', rects.second, progress(plan.secondStart, secondMs), secondWaiting)}
        </svg>
        <svg viewBox="0 0 350 150" style={{flex: '1 1 260px', maxWidth: 420, height: 'auto', display: 'block'}} role="img" aria-label="Timeline of the two refreshes">
          {[0, 250, 500, 750, 1000, 1250].map((ms) => (
            <g key={ms}>
              <line x1={toX(ms)} x2={toX(ms)} y1={18} y2={118} stroke="var(--ink-line)" strokeWidth={0.6} />
              <text x={toX(ms)} y={130} textAnchor="middle" fontSize={8.5} fill="var(--ink-muted)" fontFamily="var(--ink-mono)">
                {ms}
              </text>
            </g>
          ))}
          <text x={axisRight} y={144} textAnchor="end" fontSize={8.5} fill="var(--ink-muted)">
            milliseconds
          </text>
          <text x={4} y={44} fontSize={10} fill={colors.first.stroke} fontWeight={700}>1 · GL16</text>
          <rect x={toX(0)} y={32} width={toX(firstMs) - toX(0)} height={18} rx={3} fill={colors.first.fill} stroke={colors.first.stroke} />
          <text x={4} y={86} fontSize={10} fill={colors.second.stroke} fontWeight={700}>
            2 · {secondKind === 'partial' ? 'GL16' : 'DU'}
          </text>
          {plan.waited > 0 && (
            <>
              <rect x={toX(requestedAt)} y={74} width={toX(plan.secondStart) - toX(requestedAt)} height={18} fill="var(--ink-warn-soft)" stroke="var(--ink-warn)" strokeDasharray="3 2" />
              <text x={(toX(requestedAt) + toX(plan.secondStart)) / 2} y={86} textAnchor="middle" fontSize={8.5} fill="var(--ink-warn)">
                waits {plan.waited} ms
              </text>
            </>
          )}
          <rect x={toX(plan.secondStart)} y={74} width={toX(plan.secondEnd) - toX(plan.secondStart)} height={18} rx={3} fill={colors.second.fill} stroke={colors.second.stroke} />
          <path d={`M${toX(requestedAt)},66 l-4,-7 h8 z`} fill={colors.second.stroke} />
          <text x={toX(requestedAt) + 6} y={64} fontSize={8} fill="var(--ink-muted)">asked</text>
          <line x1={toX(plan.allDone)} x2={toX(plan.allDone)} y1={22} y2={112} stroke="var(--ink-ok)" strokeWidth={1.4} strokeDasharray="4 2" />
          <text x={toX(plan.allDone) + 3} y={110} fontSize={8.5} fill="var(--ink-ok)">done</text>
          {playhead !== null && (
            <line x1={toX(playhead)} x2={toX(playhead)} y1={14} y2={118} stroke="var(--ink-text)" strokeWidth={1.5} />
          )}
        </svg>
      </div>
      <p className={`${styles.verdict} ${plan.overlap ? styles.verdictWait : styles.verdictParallel}`}>
        {plan.overlap
          ? `They overlap: refresh 2 waits ${plan.waited} ms for refresh 1 to end. All done at ${plan.allDone} ms.`
          : `No overlap: both run at once. All done at ${plan.allDone} ms.`}
      </p>
      <Controls>
        <Button primary onClick={() => setPlayhead(0)}>
          ▶ Play
        </Button>
        <Slider label="2 is asked for" value={requestedAt} min={0} max={600} step={10} onChange={setRequestedAt} format={(value) => `${value} ms after 1`} />
        <Segmented
          options={[
            {value: 'partial', label: '2 is GL16'},
            {value: 'fast', label: '2 is DU'},
          ]}
          value={secondKind}
          onChange={setSecondKind}
        />
      </Controls>
    </div>
  );
}
