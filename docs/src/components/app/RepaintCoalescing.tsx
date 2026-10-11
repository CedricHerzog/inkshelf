// Many Updates, few Draws: three goroutines change the state in a burst; only the first Update
// finding Repaint.Queued false calls pocketbook.Repaint, and the event loop draws once for all of
// them. The model is src/lib/repaintSim.ts (state.Update, pocketbook.Repaint, app.Draw).

import React, {useMemo, useState} from 'react';
import clsx from 'clsx';
import {
  Button,
  Controls,
  Segmented,
  Toggle,
  useAnimationFrame,
} from '@site/src/components/kit';
import {simulateRepaints, type UpdateCall} from '@site/src/lib/repaintSim';
import styles from './app.module.css';

const goroutines = ['downloader', 'cover loader', 'session loader'];
const laneY = [40, 120, 200];
const stateBox = {x: 250, y: 85, width: 130, height: 90};
const slot = {x: 410, y: 113, width: 34, height: 34};
const loopBox = {x: 470, y: 70, width: 120, height: 120};
const travelMs = 380;
const slowMotion = 0.35; // the animation plays at this fraction of real time

type Pattern = 'burst' | 'spread';

const patterns: Record<Pattern, UpdateCall[]> = {
  burst: [
    {at: 0, goroutine: 0},
    {at: 70, goroutine: 1},
    {at: 120, goroutine: 2},
    {at: 200, goroutine: 0},
    {at: 260, goroutine: 1},
    {at: 330, goroutine: 0},
    {at: 400, goroutine: 2},
  ],
  spread: [
    {at: 0, goroutine: 0},
    {at: 900, goroutine: 1},
    {at: 1800, goroutine: 2},
  ],
};

function lerp(from: number, to: number, fraction: number): number {
  const clamped = Math.max(0, Math.min(1, fraction));
  return from + (to - from) * clamped;
}

export default function RepaintCoalescing() {
  const [pattern, setPattern] = useState<Pattern>('burst');
  const [busy, setBusy] = useState(true);
  const drawMs = 500;
  const run = useMemo(
    () => simulateRepaints(patterns[pattern], {loopBusyUntil: busy ? 650 : 0, drawMs}),
    [pattern, busy],
  );
  const startAt = -travelMs - 100;
  const endAt = run.end + 300;
  // Opens on the last frame, so the result shows before anything moves.
  const [now, setNow] = useState<number>(Number.POSITIVE_INFINITY);
  const [playing, setPlaying] = useState(false);
  const time = Math.min(now, endAt);

  useAnimationFrame((elapsed) => {
    setNow((current) => {
      const next = current + elapsed * slowMotion;
      if (next >= endAt) setPlaying(false);
      return next;
    });
  }, playing);

  const play = () => {
    setNow(startAt);
    setPlaying(true);
  };

  const updates = run.updates.filter((update) => update.locked <= time).length;
  const repaints = run.updates.filter((update) => update.repainted && update.locked <= time).length;
  const draws = run.draws.filter((draw) => draw.start <= time).length;
  const drawing = run.draws.find((draw) => draw.start <= time && time < draw.end);
  const queued = run.updates.some(
    (update) => update.locked <= time && run.draws[update.drawIndex]?.start > time,
  );
  const token = run.updates.some(
    (update) =>
      update.repainted && update.locked + 120 <= time && run.draws[update.drawIndex]?.start > time,
  );
  const loopBusy = busy && time < 650 && time >= 0;
  const centerY = stateBox.y + stateBox.height / 2;

  return (
    <div>
      <Controls>
        <Segmented
          options={[
            {value: 'burst', label: '7 updates at once'},
            {value: 'spread', label: '3 updates, spread out'},
          ]}
          value={pattern}
          onChange={(value) => {
            setPattern(value);
            setPlaying(false);
            setNow(Number.POSITIVE_INFINITY);
          }}
        />
        <Toggle
          label="Event loop busy when they arrive"
          checked={busy}
          onChange={(value) => {
            setBusy(value);
            setPlaying(false);
            setNow(Number.POSITIVE_INFINITY);
          }}
        />
        <Button primary onClick={play}>
          {playing ? '↺ Restart' : '▶ Fire the updates'}
        </Button>
      </Controls>
      <svg viewBox="0 0 600 250" className={styles.svg} role="img" aria-label="Goroutines sending updates to the state, and the event loop drawing">
        {goroutines.map((name, index) => (
          <g key={name}>
            <rect x={6} y={laneY[index] - 18} width={124} height={36} rx={8} fill="var(--ink-blue-soft)" stroke="var(--ink-blue)" />
            <text x={68} y={laneY[index] + 5} textAnchor="middle" fontSize={13} fill="var(--ink-text)">
              {name}
            </text>
          </g>
        ))}
        <rect {...stateBox} rx={10} fill="var(--ink-paper-2)" stroke="var(--ink-line)" strokeWidth={2} />
        <text x={stateBox.x + stateBox.width / 2} y={stateBox.y + 20} textAnchor="middle" fontSize={13} fontWeight={700} fill="var(--ink-text)">
          State
        </text>
        <text x={stateBox.x + stateBox.width / 2} y={stateBox.y + 44} textAnchor="middle" fontSize={12} fill={drawing ? 'var(--ink-warn)' : 'var(--ink-ok)'}>
          {drawing ? '🔒 Draw holds it' : '🔓 mutex free'}
        </text>
        <circle cx={stateBox.x + 22} cy={stateBox.y + 70} r={7} fill={queued ? 'var(--ink-accent)' : 'var(--ink-line)'} />
        <text x={stateBox.x + 34} y={stateBox.y + 74} fontSize={12} fill="var(--ink-text)">
          Repaint.Queued
        </text>
        <rect {...slot} rx={6} fill="none" stroke="var(--ink-text)" strokeDasharray="4 3" />
        <text x={slot.x + slot.width / 2} y={slot.y - 8} textAnchor="middle" fontSize={11} fill="var(--ink-muted)">
          repaints
        </text>
        <text x={slot.x + slot.width / 2} y={slot.y + slot.height + 14} textAnchor="middle" fontSize={11} fill="var(--ink-muted)">
          holds 1
        </text>
        {token && <circle cx={slot.x + slot.width / 2} cy={slot.y + slot.height / 2} r={9} fill="var(--ink-accent)" />}
        <rect {...loopBox} rx={10} fill="var(--ink-paper-2)" stroke="var(--ink-line)" strokeWidth={2} />
        <text x={loopBox.x + loopBox.width / 2} y={loopBox.y + 20} textAnchor="middle" fontSize={13} fontWeight={700} fill="var(--ink-text)">
          Event loop
        </text>
        <text x={loopBox.x + loopBox.width / 2} y={loopBox.y + 40} textAnchor="middle" fontSize={11} fill="var(--ink-muted)">
          {drawing ? 'Draw()' : loopBusy ? 'busy (a touch)' : 'waiting'}
        </text>
        <rect
          x={loopBox.x + 30}
          y={loopBox.y + 52}
          width={60}
          height={56}
          rx={4}
          fill={drawing ? 'var(--ink-text)' : '#e9e6dd'}
          stroke="var(--ink-text)"
          style={{transition: 'fill 0.2s'}}
        />
        <text x={loopBox.x + 60} y={loopBox.y + 84} textAnchor="middle" fontSize={10} fill={drawing ? 'var(--ink-paper)' : '#333'}>
          panel
        </text>
        {/* The updates on their way to the state. */}
        {run.updates.map((update, index) => {
          if (time < update.at - travelMs) return null;
          const startX = 132;
          const startY = laneY[update.goroutine];
          const waitX = stateBox.x - 10;
          if (time <= update.at) {
            const fraction = (time - (update.at - travelMs)) / travelMs;
            return <circle key={index} cx={lerp(startX, waitX, fraction)} cy={lerp(startY, centerY, fraction)} r={7} fill="var(--ink-blue)" />;
          }
          if (time < update.locked) {
            // Waiting for the mutex, which Draw holds.
            return <circle key={index} cx={waitX - (index % 3) * 6} cy={centerY - 14 + (index % 3) * 14} r={7} fill="var(--ink-warn)" />;
          }
          if (time < update.locked + 200) {
            const fraction = (time - update.locked) / 200;
            return (
              <g key={index} opacity={1 - fraction}>
                <circle cx={lerp(waitX, stateBox.x + 60, fraction)} cy={centerY} r={7} fill="var(--ink-blue)" />
                <text x={stateBox.x + 65} y={stateBox.y + stateBox.height + 16} textAnchor="middle" fontSize={11} fill={update.repainted ? 'var(--ink-accent)' : 'var(--ink-muted)'}>
                  {update.repainted ? 'Repaint()!' : 'already queued'}
                </text>
              </g>
            );
          }
          return null;
        })}
        {/* The token from the state to the channel, then to the event loop. */}
        {run.updates.map((update, index) => {
          if (!update.repainted || time < update.locked) return null;
          const draw = run.draws[update.drawIndex];
          if (time < update.locked + 120) {
            const fraction = (time - update.locked) / 120;
            return <circle key={`t${index}`} cx={lerp(stateBox.x + stateBox.width, slot.x + slot.width / 2, fraction)} cy={centerY} r={9} fill="var(--ink-accent)" />;
          }
          if (draw && time >= draw.start && time < draw.start + 150) {
            const fraction = (time - draw.start) / 150;
            return <circle key={`t${index}`} cx={lerp(slot.x + slot.width / 2, loopBox.x + 20, fraction)} cy={centerY} r={9} fill="var(--ink-accent)" opacity={1 - fraction} />;
          }
          return null;
        })}
      </svg>
      <div className={styles.counters}>
        <span className={styles.counter}>
          <strong>{updates}</strong>Updates
        </span>
        <span className={clsx(styles.counter, styles.counterAccent)}>
          <strong>{repaints}</strong>Repaint() calls
        </span>
        <span className={clsx(styles.counter, styles.counterOk)}>
          <strong>{draws}</strong>Draws
        </span>
      </div>
    </div>
  );
}
