// The event loop's heartbeat, slowed down: a 20 ms ticker (pollInterval) runs poll, whose steps
// light up in their real order (event_loop.go), while the Go channels of app.go (repaints,
// sleepRequests) wake the loop at once. Messages on the firmware's queues wait for the next tick.

import React, {useState} from 'react';
import clsx from 'clsx';
import {Button, Controls, Segmented, useAnimationFrame, usePrefersReducedMotion} from '@site/src/components/kit';
import {POLL_INTERVAL_MS} from '@site/src/lib/touchGestures';
import styles from './device.module.css';

const steps = ['readReplies', 'readTaskMessages', 'followForeground', 'readInput', 'repeatHeldKey', 'sendLongPress'] as const;

type MarkKind = 'input' | 'task' | 'channel';

type Mark = {id: number; kind: MarkKind; label: string; arrived: number; handled?: number};

const markColor: Record<MarkKind, string> = {
  input: 'var(--ink-accent)',
  task: 'var(--ink-blue)',
  channel: 'var(--ink-ok)',
};

// How long the firmware takes to answer a request, in this toy.
const REPLY_DELAY_MS = 27;
// Visible window of the strip, in simulated ms.
const WINDOW_MS = 120;

type Sim = {
  now: number;
  lastTick: number;
  marks: Mark[];
  hwevent: number[]; // ids of marks waiting in /hwevent
  tasks: number[]; // ids of marks waiting in the task queue
  repaintSlot: boolean;
  pendingRequests: number;
  replyAt: number | null;
  log: string[];
};

const initial: Sim = {
  now: 0,
  lastTick: 0,
  marks: [],
  hwevent: [],
  tasks: [],
  repaintSlot: false,
  pendingRequests: 0,
  replyAt: null,
  log: ['The loop sleeps between ticks. Press a button: watch what waits for the next tick, and what does not.'],
};

let nextId = 1;

export default function TickClock() {
  const reduced = usePrefersReducedMotion();
  const [running, setRunning] = useState<boolean | null>(null);
  const [slowdown, setSlowdown] = useState<'50' | '200'>('50');
  const [sim, setSim] = useState<Sim>(initial);
  const playing = running ?? !reduced;

  const addLog = (state: Sim, line: string): string[] => [...state.log.slice(-20), `[${state.now.toFixed(1)} ms] ${line}`];

  useAnimationFrame((elapsed) => {
    setSim((state) => advance(state, Math.min(elapsed, 100) / Number(slowdown)));
  }, playing);

  function advance(state: Sim, delta: number): Sim {
    let next: Sim = {...state, now: state.now + delta};
    // The firmware's reply lands in the task queue.
    if (next.replyAt !== null && next.now >= next.replyAt) {
      next = {...next, replyAt: null, log: addLog(next, 'The firmware’s reply lands in the task queue. Nobody is woken.')};
    }
    // A tick: poll runs, in order.
    const tickAt = Math.floor(next.now / POLL_INTERVAL_MS) * POLL_INTERVAL_MS;
    if (tickAt > state.lastTick) {
      next = poll({...next, lastTick: tickAt}, tickAt);
    }
    // Drop marks that left the window.
    next.marks = next.marks.filter((mark) => (mark.handled ?? next.now) > next.now - WINDOW_MS);
    return next;
  }

  function poll(state: Sim, at: number): Sim {
    let next = state;
    const handle = (ids: number[]) =>
      next.marks.map((mark) => (ids.includes(mark.id) ? {...mark, handled: at} : mark));
    if (next.pendingRequests > 0 && next.replyAt === null) {
      next = {...next, pendingRequests: 0, log: addLog(next, 'readReplies: the reply is read, and gone from the queue.')};
    }
    if (next.tasks.length > 0) {
      const waited = at - (next.marks.find((mark) => mark.id === next.tasks[0])?.arrived ?? at);
      next = {...next, marks: handle(next.tasks), tasks: [], log: addLog(next, `readTaskMessages: a firmware event, after waiting ${waited.toFixed(1)} ms → App.System.`)};
    }
    if (next.hwevent.length > 0) {
      const waited = at - (next.marks.find((mark) => mark.id === next.hwevent[0])?.arrived ?? at);
      next = {...next, marks: handle(next.hwevent), hwevent: [], log: addLog(next, `readInput: the touch, after waiting ${waited.toFixed(1)} ms in /hwevent → app.Pointer.`)};
    }
    return next;
  }

  const touch = () =>
    setSim((state) => {
      const id = nextId++;
      return {
        ...state,
        marks: [...state.marks, {id, kind: 'input', label: 'touch', arrived: state.now}],
        hwevent: [...state.hwevent, id],
        log: addLog(state, 'A finger touches: the firmware posts to /hwevent. The loop is not woken.'),
      };
    });

  const firmwareEvent = () =>
    setSim((state) => {
      const id = nextId++;
      return {
        ...state,
        marks: [...state.marks, {id, kind: 'task', label: 'event', arrived: state.now}],
        tasks: [...state.tasks, id],
        log: addLog(state, 'The firmware posts a task event (say, Wi-Fi up). It waits in the queue.'),
      };
    });

  const repaint = (times: number) =>
    setSim((state) => {
      let log = state.log;
      const marks = [...state.marks];
      for (let count = 0; count < times; count++) {
        const id = nextId++;
        if (count === 0 && !state.repaintSlot) {
          marks.push({id, kind: 'channel', label: 'Repaint', arrived: state.now, handled: state.now});
          log = [...log.slice(-20), `[${state.now.toFixed(1)} ms] A goroutine calls Repaint(): the channel wakes the select at once → app.Draw().`];
        } else {
          log = [...log.slice(-20), `[${state.now.toFixed(1)} ms] A second Repaint() finds the 1-slot channel full: dropped, since one Draw does both.`];
        }
      }
      return {...state, marks, log};
    });

  const preventSleep = () =>
    setSim((state) => {
      const id = nextId++;
      return {
        ...state,
        marks: [...state.marks, {id, kind: 'channel', label: 'sleep', arrived: state.now, handled: state.now}],
        pendingRequests: state.pendingRequests + 1,
        replyAt: state.now + REPLY_DELAY_MS,
        log: addLog(state, 'PreventSleepFor(1 min): the select wakes at once and sends request 0x123. Its reply will come later.'),
      };
    });

  // Which step of the poll is lit: they light one after another during the first 12 ms after a tick.
  const sinceTick = sim.now - sim.lastTick;
  const lit = sim.lastTick > 0 ? Math.floor(sinceTick / 2) : -1;

  // The strip.
  const width = 340;
  const left = sim.now - WINDOW_MS + 20;
  const xOf = (time: number) => ((time - left) / WINDOW_MS) * (width - 20) + 10;
  const ticks: number[] = [];
  for (let tick = Math.ceil(left / POLL_INTERVAL_MS) * POLL_INTERVAL_MS; tick <= left + WINDOW_MS; tick += POLL_INTERVAL_MS) {
    ticks.push(tick);
  }

  return (
    <div className={styles.stack}>
      <svg viewBox={`0 0 ${width} 92`} className={styles.svg} role="img" aria-label="A timeline of ticks every 20 ms and messages">
        <line x1={10} x2={width - 10} y1={58} y2={58} stroke="var(--ink-line)" strokeWidth={2} />
        {ticks.map((tick) => (
          <g key={tick}>
            <line x1={xOf(tick)} x2={xOf(tick)} y1={44} y2={72} stroke="var(--ink-text)" strokeWidth={tick === sim.lastTick ? 2.5 : 1} />
            <text x={xOf(tick)} y={86} textAnchor="middle" fontSize={9} fontFamily="var(--ink-mono)" fill="var(--ink-muted)">
              {tick} ms
            </text>
          </g>
        ))}
        <line x1={xOf(sim.now)} x2={xOf(sim.now)} y1={4} y2={74} stroke="var(--ink-accent)" strokeDasharray="3 2" />
        <text x={xOf(sim.now) - 3} y={12} textAnchor="end" fontSize={9} fill="var(--ink-accent)">
          now
        </text>
        {sim.marks.map((mark, index) => {
          const y = 22 + (index % 3) * 9;
          const end = mark.handled ?? sim.now;
          return (
            <g key={mark.id}>
              <line x1={xOf(mark.arrived)} x2={xOf(end)} y1={y} y2={y} stroke={markColor[mark.kind]} strokeWidth={3} strokeLinecap="round" opacity={0.6} />
              <circle cx={xOf(mark.arrived)} cy={y} r={3.5} fill={markColor[mark.kind]} />
              {mark.handled !== undefined && <rect x={xOf(mark.handled) - 3} y={y - 3} width={6} height={6} fill="var(--ink-text)" />}
            </g>
          );
        })}
      </svg>
      <div className={styles.small + ' ' + styles.muted}>
        <span style={{color: 'var(--ink-accent)'}}>●</span> touch in /hwevent ·{' '}
        <span style={{color: 'var(--ink-blue)'}}>●</span> firmware event in the task queue ·{' '}
        <span style={{color: 'var(--ink-ok)'}}>●</span> Go channel · ■ handled. The bar is the wait.
      </div>
      <div>
        <div className={styles.label}>poll(now), at every tick</div>
        {steps.map((step, index) => (
          <span
            key={step}
            className={clsx(styles.chip, index === lit && styles.chipAccent)}
            style={step === 'readReplies' && sim.pendingRequests === 0 ? {opacity: 0.45} : undefined}
            title={step === 'readReplies' ? 'only while requests wait for a reply' : undefined}>
            {index + 1}. {step}
          </span>
        ))}
      </div>
      <Controls>
        <Button primary onClick={() => setRunning(!playing)}>
          {playing ? '❚❚ Pause' : '▶ Run'}
        </Button>
        <Segmented
          options={[
            {value: '50', label: '50× slower'},
            {value: '200', label: '200× slower'},
          ]}
          value={slowdown}
          onChange={setSlowdown}
        />
      </Controls>
      <Controls>
        <Button onClick={touch}>Touch</Button>
        <Button onClick={firmwareEvent}>Firmware event</Button>
        <Button onClick={() => repaint(1)}>Repaint()</Button>
        <Button onClick={() => repaint(2)}>Repaint() ×2</Button>
        <Button onClick={preventSleep}>PreventSleepFor</Button>
      </Controls>
      <div className={styles.log} aria-live="polite">
        {sim.log.slice(-8).map((line, index, shown) => (
          <div key={`${line}-${index}`} className={index === shown.length - 1 ? styles.logNew : styles.logLine}>
            {line}
          </div>
        ))}
      </div>
    </div>
  );
}
