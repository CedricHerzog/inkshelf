// A touch pad that runs the event loop's touch logic (TouchTracker, after handleTouch and
// sendLongPress of touch.go) at its real pace: input is read at 20 ms ticks, a still touch
// becomes Long after 500 ms, then Hold every 500 ms, and a move past the dead zone (R² = 200)
// makes it a drag.

import React, {useEffect, useRef, useState} from 'react';
import {Button, Controls} from '@site/src/components/kit';
import {
  DRAG_DEAD_ZONE,
  LONG_PRESS_DELAY_MS,
  POLL_INTERVAL_MS,
  TouchTracker,
  type Point,
  type PointerEvent as AppPointerEvent,
} from '@site/src/lib/touchGestures';
import {rawTouch} from '@site/src/lib/firmwareMessages';
import styles from './device.module.css';

// The pad shows a 170x110 corner of the screen, two CSS pixels per screen pixel, from (600, 900).
const SCALE = 2;
const PAD_WIDTH = 340;
const PAD_HEIGHT = 220;
const ORIGIN: Point = {x: 600, y: 900};

const RADIUS = Math.sqrt(DRAG_DEAD_ZONE);

type Raw = {kind: 'down' | 'move' | 'up'; point: Point};

type Recording = {events: AppPointerEvent[]; start: Point; trail: Point[]; active: boolean; now: number};

// The opening example: a finger held still for 1.2 s, as the loop would see it.
function example(): Recording {
  const tracker = new TouchTracker();
  const start = {x: 680, y: 955};
  const events: AppPointerEvent[] = [...tracker.handle('down', start, 0)];
  for (let tick = POLL_INTERVAL_MS; tick < 1200; tick += POLL_INTERVAL_MS) events.push(...tracker.poll(tick));
  events.push(...tracker.handle('up', start, 1200));
  return {events, start, trail: [start], active: false, now: 1200};
}

const stateColor: Record<AppPointerEvent['state'], string> = {
  Down: 'var(--ink-text)',
  Move: 'var(--ink-muted)',
  Drag: 'var(--ink-blue)',
  Long: 'var(--ink-accent)',
  Hold: 'var(--ink-warn)',
  Up: 'var(--ink-text)',
};

export default function TouchPad() {
  const [recording, setRecording] = useState<Recording>(example);
  const tracker = useRef(new TouchTracker());
  const pending = useRef<Raw[]>([]);
  const timer = useRef<number | null>(null);
  const clockStart = useRef(0);

  const stopTicking = () => {
    if (timer.current !== null) window.clearInterval(timer.current);
    timer.current = null;
  };
  useEffect(() => stopTicking, []);

  // One poll: the queued input first (readInput), then sendLongPress, all with the tick's time.
  const tick = () => {
    const now = Math.round((performance.now() - clockStart.current) / POLL_INTERVAL_MS) * POLL_INTERVAL_MS;
    const raws = pending.current;
    pending.current = [];
    const events: AppPointerEvent[] = [];
    for (const raw of raws) events.push(...tracker.current.handle(raw.kind, raw.point, now));
    events.push(...tracker.current.poll(now));
    const ended = raws.some((raw) => raw.kind === 'up');
    setRecording((current) => ({
      ...current,
      events: [...current.events, ...events],
      trail: [...current.trail, ...raws.filter((raw) => raw.kind === 'move').map((raw) => raw.point)].slice(-200),
      active: ended ? false : current.active,
      now,
    }));
    if (ended) stopTicking();
  };

  const toScreen = (event: React.PointerEvent<HTMLDivElement>): Point => {
    const box = event.currentTarget.getBoundingClientRect();
    const scale = box.width / PAD_WIDTH;
    return {
      x: ORIGIN.x + Math.round((event.clientX - box.left) / scale / SCALE),
      y: ORIGIN.y + Math.round((event.clientY - box.top) / scale / SCALE),
    };
  };

  const onDown = (event: React.PointerEvent<HTMLDivElement>) => {
    event.currentTarget.setPointerCapture(event.pointerId);
    stopTicking();
    tracker.current = new TouchTracker();
    // The clock starts with the touch, so the down is read at tick 0.
    clockStart.current = performance.now();
    const point = toScreen(event);
    pending.current = [{kind: 'down', point}];
    setRecording({events: [], start: point, trail: [point], active: true, now: 0});
    tick();
    timer.current = window.setInterval(tick, POLL_INTERVAL_MS);
  };
  const onMove = (event: React.PointerEvent<HTMLDivElement>) => {
    if (timer.current === null) return;
    pending.current.push({kind: 'move', point: toScreen(event)});
  };
  const onUp = (event: React.PointerEvent<HTMLDivElement>) => {
    if (timer.current === null) return;
    pending.current.push({kind: 'up', point: toScreen(event)});
  };

  const toPad = (point: Point) => ({x: (point.x - ORIGIN.x) * SCALE, y: (point.y - ORIGIN.y) * SCALE});
  const start = toPad(recording.start);
  const last = recording.trail[recording.trail.length - 1];
  const dragging = recording.events.some((event) => event.state === 'Drag');
  const longSent = recording.events.some((event) => event.state === 'Long');
  const held = Math.min(1, recording.now / LONG_PRESS_DELAY_MS);
  const raw = rawTouch(recording.start.x, recording.start.y);

  // Collapse each run of moves and drags into one chip.
  type Item = {state: AppPointerEvent['state']; at: number; point: Point; moves: number; drags: number};
  const list: Item[] = [];
  for (const event of recording.events) {
    const previous = list[list.length - 1];
    const moving = event.state === 'Move' || event.state === 'Drag';
    if (moving && previous && previous.state === 'Move') {
      if (event.state === 'Move') previous.moves += 1;
      else previous.drags += 1;
      continue;
    }
    list.push({
      state: moving ? 'Move' : event.state,
      at: event.at,
      point: event.point,
      moves: event.state === 'Move' ? 1 : 0,
      drags: event.state === 'Drag' ? 1 : 0,
    });
  }

  // The timeline.
  const span = Math.max(1500, recording.now + 100);
  const timelineWidth = 340;
  const xAt = (at: number) => 10 + (at / span) * (timelineWidth - 20);

  return (
    <div className={styles.stack}>
      <div
        className={styles.pad}
        style={{width: '100%', maxWidth: PAD_WIDTH, aspectRatio: `${PAD_WIDTH} / ${PAD_HEIGHT}`}}
        onPointerDown={onDown}
        onPointerMove={onMove}
        onPointerUp={onUp}
        onPointerCancel={onUp}>
        <svg viewBox={`0 0 ${PAD_WIDTH} ${PAD_HEIGHT}`} className={styles.svg} style={{position: 'absolute', inset: 0}} aria-hidden>
          <circle
            cx={start.x}
            cy={start.y}
            r={RADIUS * SCALE}
            fill={dragging ? 'var(--ink-blue-soft)' : 'var(--ink-accent-soft)'}
            stroke={dragging ? 'var(--ink-blue)' : 'var(--ink-accent)'}
            strokeDasharray="4 3"
          />
          {/* The long press clock: a ring that fills in 500 ms. */}
          {!dragging && (
            <circle
              cx={start.x}
              cy={start.y}
              r={RADIUS * SCALE + 6}
              fill="none"
              stroke="var(--ink-accent)"
              strokeWidth={3}
              strokeDasharray={`${held * 2 * Math.PI * (RADIUS * SCALE + 6)} 999`}
              transform={`rotate(-90 ${start.x} ${start.y})`}
            />
          )}
          <polyline
            points={recording.trail.map((point) => `${toPad(point).x},${toPad(point).y}`).join(' ')}
            fill="none"
            stroke="var(--ink-blue)"
            strokeWidth={2}
          />
          <circle cx={toPad(last).x} cy={toPad(last).y} r={5} fill="var(--ink-text)" />
          <text x={8} y={16} fontSize={11} fill="var(--ink-muted)">
            screen ({ORIGIN.x}, {ORIGIN.y})–({ORIGIN.x + PAD_WIDTH / SCALE}, {ORIGIN.y + PAD_HEIGHT / SCALE}), drawn 2×
          </text>
          <text x={8} y={PAD_HEIGHT - 8} fontSize={11} fill="var(--ink-muted)">
            {recording.active ? (longSent ? 'long press sent' : dragging ? 'dragging' : 'still: inside the dead zone') : ''}
          </text>
        </svg>
      </div>
      <svg viewBox={`0 0 ${timelineWidth} 56`} className={styles.svg} role="img" aria-label="When each event reached the app">
        <line x1={10} x2={timelineWidth - 10} y1={30} y2={30} stroke="var(--ink-line)" strokeWidth={2} />
        <line x1={xAt(LONG_PRESS_DELAY_MS)} x2={xAt(LONG_PRESS_DELAY_MS)} y1={20} y2={40} stroke="var(--ink-accent)" strokeDasharray="2 2" />
        <text x={xAt(LONG_PRESS_DELAY_MS)} y={52} textAnchor="middle" fontSize={9} fill="var(--ink-accent)">500 ms</text>
        <text x={10} y={52} fontSize={9} fill="var(--ink-muted)">0</text>
        {recording.events.map((event, index) => (
          <g key={index}>
            <circle cx={xAt(event.at)} cy={30} r={event.state === 'Move' ? 2 : 4} fill={stateColor[event.state]} />
            {event.state !== 'Move' && event.state !== 'Drag' && (
              <text x={xAt(event.at)} y={16} textAnchor="middle" fontSize={9} fill={stateColor[event.state]}>
                {event.state}
              </text>
            )}
          </g>
        ))}
      </svg>
      <div>
        {list.map((item, index) => (
          <span
            key={index}
            className={styles.chip + ' ' + (item.state === 'Long' ? styles.chipAccent : item.state === 'Hold' ? styles.chipWarn : item.drags ? styles.chipBlue : '')}>
            {item.at} ms · Pointer{item.state}
            {item.state === 'Move' ? ` ×${item.moves}${item.drags ? `, PointerDrag ×${item.drags}` : ''}` : ''}
            {item.state === 'Down' ? ` (${item.point.x}, ${item.point.y})` : ''}
          </span>
        ))}
      </div>
      <div className={styles.small + ' ' + styles.muted}>
        The firmware sent the down as raw ({raw.rawX}, {raw.rawY}); touchPoint made it ({recording.start.x}, {recording.start.y}): x = raw y, y = 1871 − raw x.
      </div>
      <Controls>
        <Button onClick={() => setRecording(example())}>Show the example again</Button>
      </Controls>
    </div>
  );
}
