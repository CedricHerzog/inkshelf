// A page button held down, as the event loop times it (KeyRepeater, after repeatHeldKey of
// keys.go): a press, a repeat after 600 ms, then one every 300 ms, checked at each 20 ms poll,
// and the release.

import React, {useEffect, useRef, useState} from 'react';
import clsx from 'clsx';
import {
  KeyRepeater,
  POLL_INTERVAL_MS,
  REPEAT_DELAY_MS,
  REPEAT_INTERVAL_MS,
  type KeyEvent,
} from '@site/src/lib/touchGestures';
import styles from './device.module.css';

function example(): KeyEvent[] {
  const repeater = new KeyRepeater();
  const events: KeyEvent[] = [repeater.press(0)];
  for (let tick = POLL_INTERVAL_MS; tick < 1640; tick += POLL_INTERVAL_MS) {
    const repeat = repeater.poll(tick);
    if (repeat) events.push(repeat);
  }
  events.push(repeater.release(1640));
  return events;
}

const color: Record<KeyEvent['state'], string> = {
  Press: 'var(--ink-text)',
  Repeat: 'var(--ink-accent)',
  Release: 'var(--ink-muted)',
};

export default function KeyRepeat() {
  const [events, setEvents] = useState<KeyEvent[]>(example);
  const [down, setDown] = useState(false);
  const [now, setNow] = useState(1640);
  const repeater = useRef(new KeyRepeater());
  const timer = useRef<number | null>(null);
  const start = useRef(0);

  const stop = () => {
    if (timer.current !== null) window.clearInterval(timer.current);
    timer.current = null;
  };
  useEffect(() => stop, []);

  const tickTime = () => Math.round((performance.now() - start.current) / POLL_INTERVAL_MS) * POLL_INTERVAL_MS;

  const press = () => {
    if (timer.current !== null) return;
    start.current = performance.now();
    repeater.current = new KeyRepeater();
    setEvents([repeater.current.press(0)]);
    setNow(0);
    setDown(true);
    timer.current = window.setInterval(() => {
      const at = tickTime();
      setNow(at);
      const repeat = repeater.current.poll(at);
      if (repeat) setEvents((current) => [...current, repeat]);
    }, POLL_INTERVAL_MS);
  };
  const release = () => {
    if (timer.current === null) return;
    stop();
    const at = tickTime();
    setNow(at);
    setEvents((current) => [...current, repeater.current.release(at)]);
    setDown(false);
  };

  const span = Math.max(2000, now + 200);
  const width = 340;
  const xAt = (at: number) => 10 + (at / span) * (width - 20);
  const repeats = events.filter((event) => event.state === 'Repeat').length;

  return (
    <div className={styles.stack}>
      <div className={styles.row} style={{alignItems: 'center'}}>
        <button
          type="button"
          className={clsx(styles.bigKey, down && styles.bigKeyDown)}
          onPointerDown={(event) => {
            event.currentTarget.setPointerCapture(event.pointerId);
            press();
          }}
          onPointerUp={release}
          onPointerCancel={release}
          onKeyDown={(event) => {
            if ((event.key === ' ' || event.key === 'Enter') && !event.repeat) {
              event.preventDefault();
              press();
            }
          }}
          onKeyUp={(event) => {
            if (event.key === ' ' || event.key === 'Enter') release();
          }}>
          Next ›
        </button>
        <span className={styles.small + ' ' + styles.muted}>
          {down ? `held for ${now} ms` : 'hold it down'} · {repeats} repeat{repeats === 1 ? '' : 's'}
        </span>
      </div>
      <svg viewBox={`0 0 ${width} 60`} className={styles.svg} role="img" aria-label="When the app got the press, the repeats and the release">
        <line x1={10} x2={width - 10} y1={32} y2={32} stroke="var(--ink-line)" strokeWidth={2} />
        {down && <line x1={xAt(0)} x2={xAt(now)} y1={32} y2={32} stroke="var(--ink-accent-soft)" strokeWidth={8} />}
        <line x1={xAt(REPEAT_DELAY_MS)} x2={xAt(REPEAT_DELAY_MS)} y1={22} y2={42} stroke="var(--ink-accent)" strokeDasharray="2 2" />
        <text x={xAt(REPEAT_DELAY_MS)} y={55} textAnchor="middle" fontSize={9} fill="var(--ink-accent)">
          600 ms
        </text>
        <text x={10} y={55} fontSize={9} fill="var(--ink-muted)">0</text>
        {events.map((event, index) => (
          <g key={index}>
            <circle cx={xAt(event.at)} cy={32} r={4.5} fill={color[event.state]} />
            <text x={xAt(event.at)} y={18} textAnchor="middle" fontSize={8.5} fontFamily="var(--ink-mono)" fill={color[event.state]}>
              {event.state === 'Repeat' ? event.at : event.state}
            </text>
          </g>
        ))}
      </svg>
      <div className={styles.small + ' ' + styles.muted}>
        First repeat at {REPEAT_DELAY_MS} ms, then every {REPEAT_INTERVAL_MS} ms. Hold it under 600 ms: only a press and a release.
      </div>
    </div>
  );
}
