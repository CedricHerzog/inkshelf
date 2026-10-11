// The kit every chapter's widgets are built from: a frame (Figure), controls (Button, Slider,
// Toggle, Segmented), a player for step-by-step animations (useStepper and StepperBar), a grid
// of gray pixels, references into the Go code, glossary terms and a short quiz. Keeping them in
// one place makes every widget look and behave the same.

import React, {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import styles from './kit.module.css';
import {glossary} from '@site/src/data/glossary';

/* ---------- Frame ---------- */

export function Figure({
  title,
  tryThis,
  caption,
  badge = 'Interactive',
  children,
}: {
  title: string;
  tryThis?: ReactNode;
  caption?: ReactNode;
  badge?: string;
  children: ReactNode;
}) {
  return (
    <figure className={styles.figure}>
      <div className={styles.figureHead}>
        <span className={styles.figureBadge}>{badge}</span>
        <span className={styles.figureTitle}>{title}</span>
      </div>
      <div className={styles.figureBody}>{children}</div>
      {caption && <figcaption className={styles.caption}>{caption}</figcaption>}
      {tryThis && (
        <div className={styles.tryThis}>
          <strong>Try this:</strong> {tryThis}
        </div>
      )}
    </figure>
  );
}

export function OneSentence({children}: {children: ReactNode}) {
  return <div className="one-sentence">{children}</div>;
}

export function Analogy({icon = '💡', children}: {icon?: string; children: ReactNode}) {
  return (
    <div className={styles.analogy}>
      <span className={styles.analogyIcon} aria-hidden>
        {icon}
      </span>
      <div>{children}</div>
    </div>
  );
}

export function Pill({children}: {children: ReactNode}) {
  return <span className={styles.pill}>{children}</span>;
}

/* ---------- Controls ---------- */

export function Controls({children}: {children: ReactNode}) {
  return <div className={styles.controls}>{children}</div>;
}

export function Button({
  onClick,
  children,
  primary,
  active,
  disabled,
  title,
}: {
  onClick: () => void;
  children: ReactNode;
  primary?: boolean;
  active?: boolean;
  disabled?: boolean;
  title?: string;
}) {
  return (
    <button
      type="button"
      title={title}
      disabled={disabled}
      onClick={onClick}
      className={clsx(
        styles.button,
        primary && styles.buttonPrimary,
        active && styles.buttonActive,
      )}>
      {children}
    </button>
  );
}

export function Slider({
  label,
  value,
  min,
  max,
  step = 1,
  onChange,
  format = (value: number) => String(value),
}: {
  label: ReactNode;
  value: number;
  min: number;
  max: number;
  step?: number;
  onChange: (value: number) => void;
  format?: (value: number) => string;
}) {
  return (
    <label className={styles.slider}>
      <span>{label}</span>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
      />
      <span className={styles.sliderValue}>{format(value)}</span>
    </label>
  );
}

export function Toggle({
  label,
  checked,
  onChange,
}: {
  label: ReactNode;
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label className={styles.toggle}>
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
      />
      <span>{label}</span>
    </label>
  );
}

export function Segmented<Value extends string>({
  options,
  value,
  onChange,
}: {
  options: readonly {value: Value; label: ReactNode}[];
  value: Value;
  onChange: (value: Value) => void;
}) {
  return (
    <div className={styles.segmented} role="group">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          aria-pressed={option.value === value}
          onClick={() => onChange(option.value)}>
          {option.label}
        </button>
      ))}
    </div>
  );
}

/* ---------- Step-by-step animations ---------- */

export type Stepper = {
  index: number;
  count: number;
  playing: boolean;
  atEnd: boolean;
  play: () => void;
  pause: () => void;
  next: () => void;
  previous: () => void;
  reset: () => void;
  seek: (index: number) => void;
};

/**
 * useStepper plays an animation made of `count` frames, one every `intervalMs` while playing.
 * The widget draws frame `index`; it stops by itself on the last frame. When `count` changes
 * (new input), it goes back to frame 0.
 */
export function useStepper(count: number, intervalMs = 700): Stepper {
  const [index, setIndex] = useState(0);
  const [playing, setPlaying] = useState(false);

  useEffect(() => {
    setIndex(0);
    setPlaying(false);
  }, [count]);

  useEffect(() => {
    if (!playing) return undefined;
    const timer = window.setInterval(() => {
      setIndex((current) => {
        if (current >= count - 1) {
          setPlaying(false);
          return current;
        }
        return current + 1;
      });
    }, intervalMs);
    return () => window.clearInterval(timer);
  }, [playing, count, intervalMs]);

  const last = Math.max(0, count - 1);
  return {
    index: Math.min(index, last),
    count,
    playing,
    atEnd: index >= last,
    play: () => {
      if (index >= last) setIndex(0);
      setPlaying(true);
    },
    pause: () => setPlaying(false),
    next: () => {
      setPlaying(false);
      setIndex((current) => Math.min(last, current + 1));
    },
    previous: () => {
      setPlaying(false);
      setIndex((current) => Math.max(0, current - 1));
    },
    reset: () => {
      setPlaying(false);
      setIndex(0);
    },
    seek: (target: number) => {
      setPlaying(false);
      setIndex(Math.max(0, Math.min(last, target)));
    },
  };
}

/** StepperBar is the player for a useStepper: play, step back and forth, a seekable track. */
export function StepperBar({stepper, label}: {stepper: Stepper; label?: string}) {
  const track = useRef<HTMLDivElement>(null);
  const fraction = stepper.count <= 1 ? 1 : stepper.index / (stepper.count - 1);
  return (
    <div className={styles.stepper}>
      {stepper.playing ? (
        <Button primary onClick={stepper.pause}>
          ❚❚ Pause
        </Button>
      ) : (
        <Button primary onClick={stepper.play}>
          {stepper.atEnd ? '↺ Replay' : '▶ Play'}
        </Button>
      )}
      <Button onClick={stepper.previous} disabled={stepper.index === 0} title="Previous step">
        ‹
      </Button>
      <Button onClick={stepper.next} disabled={stepper.atEnd} title="Next step">
        ›
      </Button>
      <div
        ref={track}
        className={styles.stepperTrack}
        onClick={(event) => {
          const box = track.current?.getBoundingClientRect();
          if (!box) return;
          const at = (event.clientX - box.left) / box.width;
          stepper.seek(Math.round(at * (stepper.count - 1)));
        }}>
        <div className={styles.stepperFill} style={{width: `${fraction * 100}%`}} />
      </div>
      <span className={styles.stepperCount}>
        {label ?? 'step'} {stepper.index + 1}/{stepper.count}
      </span>
    </div>
  );
}

/** Narration is the sentence under a stepper that says what the current frame shows. */
export function Narration({children}: {children: ReactNode}) {
  return (
    <div className={styles.narration} aria-live="polite">
      {children}
    </div>
  );
}

/**
 * useAnimationFrame calls `frame` with the milliseconds since the previous frame, on every
 * display frame while `running` is true. For continuous animations (a ticking clock, a
 * countdown bar); step-by-step ones use useStepper.
 */
export function useAnimationFrame(frame: (elapsedMs: number) => void, running: boolean) {
  const latest = useRef(frame);
  latest.current = frame;
  useEffect(() => {
    if (!running) return undefined;
    let previous = performance.now();
    let handle = requestAnimationFrame(function tick(now) {
      latest.current(now - previous);
      previous = now;
      handle = requestAnimationFrame(tick);
    });
    return () => cancelAnimationFrame(handle);
  }, [running]);
}

/** usePrefersReducedMotion is true when the viewer asked for less motion. */
export function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    const query = window.matchMedia('(prefers-reduced-motion: reduce)');
    setReduced(query.matches);
    const listener = () => setReduced(query.matches);
    query.addEventListener('change', listener);
    return () => query.removeEventListener('change', listener);
  }, []);
  return reduced;
}

/* ---------- Pixels ---------- */

/**
 * PixelGrid draws `cells` (gray levels 0 = black to 255 = white, row by row) as squares. Cells
 * can carry a label, be highlighted with the accent color, and react to clicks and hovers.
 */
export function PixelGrid({
  columns,
  cells,
  cellSize = 28,
  labels,
  highlight,
  outline,
  onCellClick,
  onCellHover,
  gap = 1,
  ariaLabel,
}: {
  columns: number;
  cells: readonly number[];
  cellSize?: number;
  labels?: readonly (string | undefined)[];
  highlight?: ReadonlySet<number>;
  outline?: ReadonlySet<number>;
  onCellClick?: (index: number) => void;
  onCellHover?: (index: number | null) => void;
  gap?: number;
  ariaLabel?: string;
}) {
  const rows = Math.ceil(cells.length / columns);
  const width = columns * cellSize + (columns + 1) * gap;
  const height = rows * cellSize + (rows + 1) * gap;
  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      role="img"
      aria-label={ariaLabel}
      style={{maxWidth: '100%', height: 'auto', display: 'block'}}
      onMouseLeave={() => onCellHover?.(null)}>
      <rect width={width} height={height} fill="var(--ink-line)" />
      {cells.map((gray, index) => {
        const column = index % columns;
        const row = Math.floor(index / columns);
        const x = gap + column * (cellSize + gap);
        const y = gap + row * (cellSize + gap);
        const level = Math.max(0, Math.min(255, Math.round(gray)));
        const label = labels?.[index];
        return (
          <g
            key={index}
            onClick={onCellClick ? () => onCellClick(index) : undefined}
            onMouseEnter={onCellHover ? () => onCellHover(index) : undefined}
            style={{cursor: onCellClick ? 'pointer' : undefined}}>
            <rect
              x={x}
              y={y}
              width={cellSize}
              height={cellSize}
              fill={`rgb(${level},${level},${level})`}
            />
            {highlight?.has(index) && (
              <rect
                x={x + 1.5}
                y={y + 1.5}
                width={cellSize - 3}
                height={cellSize - 3}
                fill="none"
                stroke="var(--ink-accent)"
                strokeWidth={3}
              />
            )}
            {outline?.has(index) && (
              <rect
                x={x + 1}
                y={y + 1}
                width={cellSize - 2}
                height={cellSize - 2}
                fill="none"
                stroke="var(--ink-blue)"
                strokeWidth={2}
                strokeDasharray="3 2"
              />
            )}
            {label !== undefined && (
              <text
                x={x + cellSize / 2}
                y={y + cellSize / 2}
                textAnchor="middle"
                dominantBaseline="central"
                fontSize={Math.max(8, cellSize * 0.38)}
                fontFamily="var(--ink-mono)"
                fill={level < 128 ? '#fff' : '#000'}>
                {label}
              </text>
            )}
          </g>
        );
      })}
    </svg>
  );
}

/* ---------- Code and docs references ---------- */

function useRepoUrl(): string {
  const {siteConfig} = useDocusaurusContext();
  return String(siteConfig.customFields?.repoUrl ?? '');
}

/**
 * CodeRef names a place in the repository: a path, and optionally a symbol in it. It becomes a
 * link when customFields.repoUrl is set in docusaurus.config.ts.
 */
export function CodeRef({path, symbol}: {path: string; symbol?: string}) {
  const repoUrl = useRepoUrl();
  const text = symbol ? `${path} · ${symbol}` : path;
  if (!repoUrl) return <code className={styles.codeRef}>{text}</code>;
  return (
    <a href={`${repoUrl}/${path}`} target="_blank" rel="noreferrer">
      <code className={styles.codeRef}>{text}</code>
    </a>
  );
}

/** CodeMap is the "Where it lives in the code" list that ends each chapter. */
export function CodeMap({
  entries,
}: {
  entries: readonly {path: string; symbol?: string; what: ReactNode}[];
}) {
  return (
    <ul className={styles.codeMap}>
      {entries.map((entry) => (
        <li key={`${entry.path}${entry.symbol ?? ''}`}>
          <span>
            <CodeRef path={entry.path} symbol={entry.symbol} />
          </span>
          <span>{entry.what}</span>
        </li>
      ))}
    </ul>
  );
}

/**
 * RefDoc links to a page of the reference (docs/reference/<file>), the precise version of a
 * chapter: `file` is its file name, with an optional #anchor.
 */
export function RefDoc({file, children}: {file: string; children?: ReactNode}) {
  const [name, anchor] = file.split('#');
  const slug = name.replace(/\.md$/, '');
  return <Link to={`/reference/${slug}${anchor ? `#${anchor}` : ''}`}>{children ?? slug}</Link>;
}

/* ---------- Glossary terms ---------- */

/**
 * Term marks a word defined in the glossary: hovering shows the short definition, clicking
 * goes to the glossary. `id` is a key of src/data/glossary.ts; the children default to the
 * term's name.
 */
export function Term({id, children}: {id: string; children?: ReactNode}) {
  const entry = glossary.find((candidate) => candidate.id === id);
  if (!entry) {
    throw new Error(`Term: no glossary entry "${id}"`);
  }
  return (
    <Link to={`/glossary#${id}`} className={styles.term}>
      {children ?? entry.term}
      <span className={styles.termTip} role="tooltip">
        <strong>{entry.term}</strong>: {entry.short}
      </span>
    </Link>
  );
}

/* ---------- Quiz ---------- */

export type QuizQuestion = {
  question: ReactNode;
  choices: readonly ReactNode[];
  answer: number;
  why: ReactNode;
};

/** Quiz asks a few questions at the end of a chapter; a click reveals the answer and why. */
export function Quiz({questions}: {questions: readonly QuizQuestion[]}) {
  return (
    <div className={styles.quiz}>
      {questions.map((question, index) => (
        <QuizItem key={index} question={question} />
      ))}
    </div>
  );
}

function QuizItem({question}: {question: QuizQuestion}) {
  const [picked, setPicked] = useState<number | null>(null);
  const reveal = useCallback((choice: number) => setPicked(choice), []);
  return (
    <div className={styles.question}>
      <div className={styles.questionText}>{question.question}</div>
      <div className={styles.choices}>
        {question.choices.map((choice, index) => (
          <button
            key={index}
            type="button"
            className={clsx(
              styles.choice,
              picked !== null && index === question.answer && styles.choiceRight,
              picked === index && index !== question.answer && styles.choiceWrong,
            )}
            disabled={picked !== null}
            onClick={() => reveal(index)}>
            {choice}
          </button>
        ))}
      </div>
      {picked !== null && (
        <div className={styles.why}>
          {picked === question.answer ? '✓ Right. ' : '✗ Not quite. '}
          {question.why}
        </div>
      )}
    </div>
  );
}
