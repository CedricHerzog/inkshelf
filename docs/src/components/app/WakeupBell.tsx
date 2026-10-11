// The doorbell of a background loop, as state.Wakeup (internal/app/state/work.go) works: a
// channel that holds one wake-up. Wake puts one in if there is room and otherwise does nothing,
// so it never blocks; the loop (here openbook.Preloader) takes one, does its work, calls Done,
// and waits again.

import React, {useEffect, useRef, useState} from 'react';
import clsx from 'clsx';
import {Button, Controls, Toggle, useAnimationFrame} from '@site/src/components/kit';
import styles from './app.module.css';

const workMs = 1800;

type Sim = {
  slot: boolean; // a wake-up waits in the channel
  working: boolean;
  progress: number; // 0..1 through the loop's work
  rings: number;
  covered: number; // rings that found the slot full
  runs: number; // wake-ups taken by the loop
  work: number; // state.Work's count for "preloading pages"
  log: string[];
};

const initial: Sim = {
  slot: false,
  working: true,
  progress: 0.55,
  rings: 1,
  covered: 0,
  runs: 1,
  work: 1,
  log: ['Wake(): slot empty, the wake-up goes in', 'loop takes it: preloading pages'],
};

function withLog(sim: Sim, line: string): string[] {
  return [line, ...sim.log].slice(0, 6);
}

export default function WakeupBell() {
  const [sim, setSim] = useState<Sim>(initial);
  const [auto, setAuto] = useState(false);
  const [ringing, setRinging] = useState(false);
  const ringTimer = useRef<number | undefined>(undefined);

  const ring = () => {
    setRinging(true);
    window.clearTimeout(ringTimer.current);
    ringTimer.current = window.setTimeout(() => setRinging(false), 180);
    setSim((current) => {
      // Wake: Begin, then try to send without blocking.
      if (!current.slot) {
        return {
          ...current,
          slot: true,
          rings: current.rings + 1,
          work: current.work + 1,
          log: withLog(current, 'Wake(): slot empty, the wake-up goes in'),
        };
      }
      return {
        ...current,
        rings: current.rings + 1,
        covered: current.covered + 1,
        log: withLog(current, 'Wake(): slot full, the waiting wake-up covers this one'),
      };
    });
  };

  useEffect(() => {
    if (!auto) return undefined;
    const timer = window.setInterval(ring, 250);
    return () => window.clearInterval(timer);
  }, [auto]);

  // The loop: take a wake-up when waiting, work, Done.
  useEffect(() => {
    if (!sim.working && sim.slot) {
      setSim((current) =>
        current.working || !current.slot
          ? current
          : {
              ...current,
              slot: false,
              working: true,
              progress: 0,
              runs: current.runs + 1,
              log: withLog(current, 'loop takes it: preloading pages'),
            },
      );
    }
  }, [sim.working, sim.slot]);

  useAnimationFrame((elapsed) => {
    setSim((current) => {
      if (!current.working) return current;
      const progress = current.progress + elapsed / workMs;
      if (progress < 1) return {...current, progress};
      return {
        ...current,
        working: false,
        progress: 0,
        work: current.work - 1,
        log: withLog(current, 'loop: Done(), waits on Woken()'),
      };
    });
  }, sim.working);

  return (
    <div>
      <Controls>
        <Button primary onClick={ring}>
          🔔 Wake()
        </Button>
        <Toggle label="Ring every 250 ms" checked={auto} onChange={setAuto} />
        <Button
          onClick={() => {
            setAuto(false);
            setSim({...initial, working: false, progress: 0, rings: 0, runs: 0, work: 0, log: []});
          }}>
          Reset
        </Button>
      </Controls>
      <svg viewBox="0 0 600 170" className={styles.svg} role="img" aria-label="A doorbell with room for one wake-up, and the loop it wakes">
        <g transform={ringing ? 'rotate(-12 70 70)' : undefined} style={{transition: 'transform 0.1s'}}>
          <text x={70} y={88} textAnchor="middle" fontSize={54}>
            🔔
          </text>
        </g>
        <text x={70} y={130} textAnchor="middle" fontSize={12} fill="var(--ink-text)">
          the main thread
        </text>
        <text x={70} y={146} textAnchor="middle" fontSize={11} fill="var(--ink-ok)">
          never waits
        </text>
        <line x1={120} y1={70} x2={235} y2={70} stroke="var(--ink-line)" strokeWidth={3} />
        <rect x={240} y={40} width={70} height={60} rx={8} fill="none" stroke="var(--ink-text)" strokeDasharray="5 4" strokeWidth={2} />
        {sim.slot && <circle cx={275} cy={70} r={16} fill="var(--ink-accent)" />}
        <text x={275} y={122} textAnchor="middle" fontSize={12} fill="var(--ink-text)">
          channel, holds 1
        </text>
        <line x1={315} y1={70} x2={400} y2={70} stroke="var(--ink-line)" strokeWidth={3} />
        <rect x={405} y={25} width={180} height={95} rx={10} fill="var(--ink-paper-2)" stroke={sim.working ? 'var(--ink-accent)' : 'var(--ink-line)'} strokeWidth={2} />
        <text x={495} y={48} textAnchor="middle" fontSize={13} fontWeight={700} fill="var(--ink-text)">
          Preloader goroutine
        </text>
        <text x={495} y={70} textAnchor="middle" fontSize={12} fill="var(--ink-muted)">
          {sim.working ? 'preloading pages…' : 'asleep on Woken()'}
        </text>
        <rect x={425} y={84} width={140} height={10} rx={5} fill="var(--ink-line)" />
        <rect x={425} y={84} width={140 * sim.progress} height={10} rx={5} fill="var(--ink-accent)" />
      </svg>
      <div className={styles.counters}>
        <span className={styles.counter}>
          <strong>{sim.rings}</strong>rings
        </span>
        <span className={styles.counter}>
          <strong>{sim.covered}</strong>covered by the one waiting
        </span>
        <span className={clsx(styles.counter, styles.counterAccent)}>
          <strong>{sim.runs}</strong>times the loop ran
        </span>
        <span className={clsx(styles.counter, styles.counterOk)}>
          <strong>0 ms</strong>the ringer waited
        </span>
        <span className={styles.counter}>
          <strong>{sim.work}</strong>work in progress
        </span>
      </div>
      <div className={styles.log} aria-live="polite">
        {sim.log.map((line, index) => (
          <div key={`${sim.rings}-${sim.runs}-${index}`} style={{opacity: 1 - index * 0.13}}>
            {line}
          </div>
        ))}
      </div>
    </div>
  );
}
