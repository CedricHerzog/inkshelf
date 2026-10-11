// The deadlock game: choose the order in which two goroutines take three locks, then play.
// src/lib/deadlock.ts tries every interleaving of their steps: if one deadlocks, it is played,
// with the cycle of waits highlighted; otherwise the two finish.

import React, {useMemo, useState} from 'react';
import clsx from 'clsx';
import {
  Button,
  Controls,
  Narration,
  Segmented,
  StepperBar,
  useStepper,
} from '@site/src/components/kit';
import {explore, lockNames, type LockId, type Program} from '@site/src/lib/deadlock';
import styles from './app.module.css';

const locks: LockId[] = ['S', 'M', 'R'];
const lockX: Record<LockId, number> = {S: 150, M: 300, R: 450};
const lockY = 200;
const goroutineX = [100, 500];
const goroutineY = 62;
const colors = ['var(--ink-blue)', 'var(--ink-ok)'];

type Preset = 'rule' | 'forbidden' | 'three';

const presets: Record<Preset, {names: [string, string]; programs: [LockId[], LockId[]]}> = {
  rule: {
    names: ['Syncer.PushNow', 'Draw of an EPUB page'],
    programs: [
      ['S', 'M'],
      ['M', 'R'],
    ],
  },
  forbidden: {
    names: ['Syncer.PushNow', 'a tap calling PushNow inside Update'],
    programs: [
      ['S', 'M'],
      ['M', 'S'],
    ],
  },
  three: {
    names: ['Goroutine 1', 'Goroutine 2'],
    programs: [
      ['S', 'M', 'R'],
      ['R', 'M', 'S'],
    ],
  },
};

function LockPicker({
  index,
  name,
  program,
  onChange,
}: {
  index: number;
  name: string;
  program: Program;
  onChange: (program: LockId[]) => void;
}) {
  return (
    <div className={styles.pickerRow}>
      <span className={styles.pickerLabel} style={{color: colors[index]}}>
        {index + 1}. {name}
      </span>
      {locks.map((lock) => {
        const position = program.indexOf(lock);
        return (
          <Button
            key={lock}
            active={position >= 0}
            title={position >= 0 ? 'Click to drop this lock' : 'Click to take this lock next'}
            onClick={() =>
              onChange(position >= 0 ? program.filter((other) => other !== lock) : [...program, lock])
            }>
            {position >= 0 ? `${position + 1}· ` : ''}
            {lockNames[lock]}
          </Button>
        );
      })}
      <span className={styles.order}>
        {program.length === 0 ? 'no lock' : program.map((lock) => lockNames[lock]).join(' → ')}
      </span>
    </div>
  );
}

function Padlock({id, holder, inCycle}: {id: LockId; holder: 0 | 1 | null; inCycle: boolean}) {
  const x = lockX[id];
  const fill = holder === null ? 'var(--ink-paper)' : colors[holder];
  return (
    <g>
      <path
        d={`M ${x - 13} ${lockY - 8} v -10 a 13 13 0 0 1 26 0 v ${holder === null ? -4 : 10}`}
        fill="none"
        stroke={inCycle ? 'var(--ink-accent)' : 'var(--ink-text)'}
        strokeWidth={4}
        style={{transition: 'all 0.3s'}}
      />
      <rect
        x={x - 22}
        y={lockY - 8}
        width={44}
        height={34}
        rx={6}
        fill={fill}
        stroke={inCycle ? 'var(--ink-accent)' : 'var(--ink-text)'}
        strokeWidth={inCycle ? 3 : 1.5}
        style={{transition: 'fill 0.3s'}}
      />
      <text x={x} y={lockY + 14} textAnchor="middle" fontSize={14} fontWeight={700} fill={holder === null ? 'var(--ink-text)' : 'var(--ink-paper)'}>
        {id}
      </text>
      <text x={x} y={lockY + 44} textAnchor="middle" fontSize={12} fill="var(--ink-muted)">
        {lockNames[id]}
      </text>
    </g>
  );
}

export default function DeadlockGame() {
  const [names, setNames] = useState<[string, string]>(presets.forbidden.names);
  const [programs, setPrograms] = useState<[LockId[], LockId[]]>(presets.forbidden.programs);
  const [preset, setPreset] = useState<Preset | 'custom'>('forbidden');
  const exploration = useMemo(() => explore(programs), [programs]);
  // The stepper restarts when the number of frames changes; the key below restarts it on any
  // change of the programs.
  return (
    <div>
      <Controls>
        <Segmented
          options={[
            {value: 'forbidden', label: 'The forbidden call'},
            {value: 'rule', label: 'Inkshelf’s order'},
            {value: 'three', label: 'Opposite orders'},
            {value: 'custom', label: 'Your own'},
          ]}
          value={preset}
          onChange={(value) => {
            setPreset(value);
            if (value !== 'custom') {
              setNames(presets[value].names);
              setPrograms(presets[value].programs);
            } else {
              setNames(['Goroutine 1', 'Goroutine 2']);
            }
          }}
        />
      </Controls>
      {[0, 1].map((index) => (
        <LockPicker
          key={index}
          index={index}
          name={names[index]}
          program={programs[index]}
          onChange={(program) => {
            const next: [LockId[], LockId[]] = [...programs];
            next[index] = program;
            setPrograms(next);
            setPreset('custom');
            setNames(['Goroutine 1', 'Goroutine 2']);
          }}
        />
      ))}
      <Play key={JSON.stringify(programs)} exploration={exploration} names={names} />
    </div>
  );
}

function Play({
  exploration,
  names,
}: {
  exploration: ReturnType<typeof explore>;
  names: [string, string];
}) {
  const stepper = useStepper(exploration.frames.length, 1100);
  const frame = exploration.frames[stepper.index];
  const frozen = exploration.deadlocked && stepper.atEnd;
  const cycleLocks = new Set<LockId>(
    frozen ? (frame.waitsFor.filter((lock) => lock !== null) as LockId[]) : [],
  );
  return (
    <>
      <svg viewBox="0 0 600 270" className={styles.svg} role="img" aria-label="Two goroutines and three locks">
        {[0, 1].map((goroutine) => {
          const finished = frame.finished[goroutine];
          return (
            <g key={goroutine} opacity={finished ? 0.45 : 1}>
              <circle
                cx={goroutineX[goroutine]}
                cy={goroutineY}
                r={30}
                fill={frozen ? 'var(--ink-blue-soft)' : 'var(--ink-paper-2)'}
                stroke={colors[goroutine]}
                strokeWidth={frame.actor === goroutine ? 5 : 2.5}
              />
              <text x={goroutineX[goroutine]} y={goroutineY + 6} textAnchor="middle" fontSize={18} fontWeight={700} fill={colors[goroutine]}>
                {frozen ? '❄' : finished ? '✓' : goroutine + 1}
              </text>
              <text x={goroutineX[goroutine]} y={goroutineY - 40} textAnchor="middle" fontSize={12} fill="var(--ink-text)">
                {names[goroutine].length > 30 ? `${names[goroutine].slice(0, 29)}…` : names[goroutine]}
              </text>
            </g>
          );
        })}
        {/* Who holds what: solid lines. */}
        {locks.map((lock) => {
          const holder = frame.holder[lock];
          if (holder === null) return null;
          return (
            <line
              key={`h${lock}`}
              x1={lockX[lock]}
              y1={lockY - 24}
              x2={goroutineX[holder]}
              y2={goroutineY + 30}
              stroke={cycleLocks.has(lock) ? 'var(--ink-accent)' : colors[holder]}
              strokeWidth={cycleLocks.has(lock) ? 4 : 3}
            />
          );
        })}
        {/* Who waits for what: dashed arrows. */}
        {[0, 1].map((goroutine) => {
          const wanted = frame.waitsFor[goroutine];
          if (!wanted) return null;
          const fromX = goroutineX[goroutine];
          const toX = lockX[wanted] + (goroutine === 0 ? -14 : 14);
          return (
            <g key={`w${goroutine}`}>
              <line
                x1={fromX + (goroutine === 0 ? 18 : -18)}
                y1={goroutineY + 24}
                x2={toX}
                y2={lockY - 30}
                stroke={frozen ? 'var(--ink-accent)' : 'var(--ink-warn)'}
                strokeWidth={frozen ? 4 : 2.5}
                strokeDasharray="7 5"
              />
              <circle cx={toX} cy={lockY - 30} r={4} fill={frozen ? 'var(--ink-accent)' : 'var(--ink-warn)'} />
            </g>
          );
        })}
        {locks.map((lock) => (
          <Padlock key={lock} id={lock} holder={frame.holder[lock]} inCycle={cycleLocks.has(lock)} />
        ))}
        {frozen && (
          <text x={300} y={120} textAnchor="middle" fontSize={20} fontWeight={800} fill="var(--ink-accent)">
            DEADLOCK: frozen forever
          </text>
        )}
        {!exploration.deadlocked && stepper.atEnd && (
          <text x={300} y={120} textAnchor="middle" fontSize={18} fontWeight={800} fill="var(--ink-ok)">
            Both finished
          </text>
        )}
      </svg>
      <StepperBar stepper={stepper} />
      <Narration>
        <span className={clsx(frozen && styles.verdict)}>{frame.action}</span>
        {frame.waitsFor.some(Boolean) && !frozen && (
          <span>
            {' '}
            {frame.waitsFor
              .map((lock, goroutine) =>
                lock ? `Goroutine ${goroutine + 1} waits for the ${lockNames[lock]}.` : '',
              )
              .join(' ')}
          </span>
        )}
      </Narration>
    </>
  );
}
