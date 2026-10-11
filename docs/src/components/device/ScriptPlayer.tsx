// An emulator script played step by step against a sketch of the panel. The lines are those of
// emulator/scripts/smoke.txt, shortened; what each command does follows runScript and runStep of
// cmd/emulator/script.go (waitSettled before input, waitIdle before a shot, tapTime, swipeTime).
// The screens are drawn by hand: a sketch, not the app's real rendering.

import React from 'react';
import clsx from 'clsx';
import {Narration, StepperBar, useStepper} from '@site/src/components/kit';
import styles from './device.module.css';

type Screen = {kind: 'series'} | {kind: 'books'; page: number};

type Line = {
  text: string;
  comment?: string;
  /** The screen after the line ran. */
  screen?: Screen;
  touch?: {x: number; y: number; toX?: number};
  shot?: string;
  background?: boolean;
  explain: string;
};

const lines: Line[] = [
  {text: 'shot 01-series', explain: 'shot waits until the app is idle (input handled, background work over, panel unchanged for 500 ms), then saves build/shots/01-series.png.', shot: '01-series'},
  {
    text: 'tap 700 1048',
    comment: 'Beta Chronicles',
    touch: {x: 700, y: 1048},
    screen: {kind: 'books', page: 3},
    explain: 'An input command first waits until the app is settled. Then PointerDown at (700, 1048), 50 ms, PointerUp: two 32-byte messages, as from a finger.',
  },
  {text: 'shot 02-books', shot: '02-books', explain: 'The books screen opened on page 3, where the reading stopped. shot waits for the covers to load before saving.'},
  {
    text: 'tap 1200 1790',
    comment: 'Next list page',
    touch: {x: 1200, y: 1790},
    screen: {kind: 'books', page: 4},
    explain: 'The footer’s right third is Next ›.',
  },
  {text: 'shot 03-books-page-4', shot: '03-books-page-4', explain: 'Saved.'},
  {
    text: 'swipe 300 900 1100 900',
    comment: 'previous list page',
    touch: {x: 300, y: 900, toX: 1100},
    screen: {kind: 'books', page: 3},
    explain: 'A swipe: PointerDown, then 10 moves over 200 ms, then PointerUp. The moves leave the dead zone, so the app sees drags: a swipe right turns back a page.',
  },
  {text: 'shot 04-books-page-3', shot: '04-books-page-3', explain: 'Saved.'},
  {
    text: 'tap 150 84',
    comment: '‹ Back',
    touch: {x: 150, y: 84},
    screen: {kind: 'series'},
    explain: 'The header’s Back button, at (150, 84).',
  },
  {text: 'home', background: true, explain: 'SetForeground(false): the emulator drops input and posts task event 152. visible() turns false. Nothing else draws in the emulator, so the panel keeps its picture.'},
  {text: 'wait 300ms', background: true, explain: 'A plain sleep. Scripts rarely need one: input and shots wait by themselves.'},
  {text: 'resume', comment: 'a full redraw', explain: 'SetForeground(true) and event 151: followForeground sees the app in front again and calls Draw.'},
  {text: 'shot 05-resumed', shot: '05-resumed', explain: 'With -frozen-status the clock and free space are fixed, so this PNG is identical, byte for byte, on every run. shots-compare diffs it with the baseline.'},
];

// The sketch: the 1404x1872 screen at a tenth of its size.
function Panel({screen, touch, background, small}: {screen: Screen; touch?: Line['touch']; background?: boolean; small?: boolean}) {
  const rows =
    screen.kind === 'series'
      ? [
          {y: 396, height: 261, title: 'Continue: Alpha Saga 2'},
          {y: 657, height: 261, title: 'Alpha Saga'},
          {y: 918, height: 261, title: 'Beta Chronicles'},
          {y: 1179, height: 261, title: 'Gamma'},
          {y: 1440, height: 261, title: '…'},
        ]
      : [0, 1, 2, 3, 4].map((index) => ({
          y: 292 + index * 282,
          height: 282,
          title: `Beta, Vol. ${(screen.page - 1) * 5 + index + 1}`,
        }));
  return (
    <svg
      viewBox="0 0 1404 1872"
      width={small ? 56 : 140 * 1.4}
      style={{display: 'block', border: '2px solid var(--ink-text)', borderRadius: 6, background: '#fff', maxWidth: '100%', height: 'auto'}}
      role="img"
      aria-label={screen.kind === 'series' ? 'The series screen' : `Books, page ${screen.page}`}>
      <rect width={1404} height={170} fill="#ddd" />
      <text x={60} y={110} fontSize={70} fill="#000">
        ‹
      </text>
      <text x={702} y={110} fontSize={64} textAnchor="middle" fill="#000">
        {screen.kind === 'series' ? 'Fake' : 'Beta Chronicles'}
      </text>
      {screen.kind === 'books' && (
        <g>
          {[262, 702, 1142].map((x, index) => (
            <rect key={x} x={x - 200} y={195} width={400} height={70} rx={20} fill={index === 0 ? '#333' : '#eee'} />
          ))}
        </g>
      )}
      {rows.map((row) => (
        <g key={row.y}>
          <rect x={40} y={row.y + 20} width={150} height={row.height - 40} fill="#999" />
          <text x={230} y={row.y + row.height / 2 + 20} fontSize={60} fill="#000">
            {row.title}
          </text>
          <line x1={40} x2={1364} y1={row.y + row.height} y2={row.y + row.height} stroke="#ccc" strokeWidth={4} />
        </g>
      ))}
      <rect y={1730} width={1404} height={142} fill="#eee" />
      <text x={120} y={1810} fontSize={56} fill="#000">
        ‹ Prev
      </text>
      <text x={702} y={1810} fontSize={56} textAnchor="middle" fill="#000">
        {screen.kind === 'series' ? '1/1' : `${screen.page}/5`}
      </text>
      <text x={1290} y={1810} fontSize={56} textAnchor="end" fill="#000">
        Next ›
      </text>
      {touch && (
        <g>
          {touch.toX !== undefined && (
            <line x1={touch.x} x2={touch.toX} y1={touch.y} y2={touch.y} stroke="#a4452c" strokeWidth={24} strokeLinecap="round" opacity={0.6} />
          )}
          <circle cx={touch.x} cy={touch.y} r={60} fill="none" stroke="#a4452c" strokeWidth={16} />
        </g>
      )}
      {background && (
        <g>
          <rect x={300} y={830} width={804} height={180} rx={30} fill="#000" opacity={0.75} />
          <text x={702} y={945} fontSize={64} textAnchor="middle" fill="#fff">
            in the background
          </text>
        </g>
      )}
    </svg>
  );
}

export default function ScriptPlayer() {
  const stepper = useStepper(lines.length + 1, 1500);
  // Step 0 is before the first line; step k has run lines 0..k-1.
  const ran = stepper.index;
  const current = ran > 0 ? lines[ran - 1] : null;
  let screen: Screen = {kind: 'series'};
  const shots: {name: string; screen: Screen}[] = [];
  for (const line of lines.slice(0, ran)) {
    if (line.screen) screen = line.screen;
    if (line.shot) shots.push({name: line.shot, screen});
  }
  return (
    <div className={styles.stack}>
      <div className={styles.row}>
        <div className={styles.script} role="list">
          {lines.map((line, index) => (
            <div
              key={index}
              role="listitem"
              className={clsx(styles.scriptLine, index < ran - 1 && styles.scriptLineDone, index === ran - 1 && styles.scriptLineNow)}>
              {line.text.padEnd(24)}
              {line.comment && <span className={styles.scriptComment}># {line.comment}</span>}
            </div>
          ))}
        </div>
        <Panel screen={screen} touch={current?.touch} background={current?.background} />
      </div>
      <StepperBar stepper={stepper} label="line" />
      <Narration>
        {current
          ? current.explain
          : 'make shots runs: go run ./cmd/emulator -fake -fresh -script emulator/scripts/smoke.txt -out build/shots. The script starts once the app is idle.'}
      </Narration>
      <div>
        <div className={styles.label}>build/shots/</div>
        <div className={styles.film}>
          {shots.length === 0 && <span className={styles.small + ' ' + styles.muted}>no shot yet</span>}
          {shots.map((shot) => (
            <div key={shot.name} className={styles.filmFrame}>
              <Panel screen={shot.screen} small />
              {shot.name}.png
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
