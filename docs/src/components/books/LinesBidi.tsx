// Rule L2 of the bidirectional algorithm, as reorder in internal/typesetting/layout/bidi.go applies
// it to a line's fragments and gaps: from the highest level down to the lowest odd one, every run
// at that level or higher is reversed. The levels below are those go-text's bidi package gives
// these lines (checked with it).

import React, {useEffect, useLayoutEffect, useMemo, useRef, useState} from 'react';
import clsx from 'clsx';
import {Narration, Segmented, StepperBar, usePrefersReducedMotion, useStepper} from '@site/src/components/kit';
import {reorderSteps} from '@site/src/lib/bidiReorder';
import styles from './LinesWidgets.module.css';

type Item = {id: number; text: string; level: number; gap?: boolean};

type Example = {label: string; paragraph: 'right to left' | 'left to right'; meaning: string; items: Item[]};

function items(parts: [string, number][]): Item[] {
  return parts.map(([text, level], id) => ({id, text, level, gap: text === ' '}));
}

const examples: Record<string, Example> = {
  hebrew: {
    label: 'Hebrew with English inside',
    paragraph: 'right to left',
    meaning: '“I have a new InkPad 3”',
    items: items([
      ['יש', 1], [' ', 1], ['לי', 1], [' ', 1], ['InkPad', 2], [' ', 2], ['3', 2], [' ', 1], ['חדש', 1],
    ]),
  },
  english: {
    label: 'English with Hebrew inside',
    paragraph: 'left to right',
    meaning: 'He said “hello world” twice',
    items: items([
      ['He', 0], [' ', 0], ['said', 0], [' ', 0], ['שלום', 1], [' ', 1], ['עולם', 1], [' ', 0], ['twice', 0],
    ]),
  },
  number: {
    label: 'A number in Hebrew in English',
    paragraph: 'left to right',
    meaning: 'He wrote “peace 123 world” today',
    items: items([
      ['He', 0], [' ', 0], ['wrote', 0], [' ', 0], ['שלום', 1], [' ', 1], ['123', 2], [' ', 1], ['עולם', 1], [' ', 0], ['today', 0],
    ]),
  },
};

const options = Object.entries(examples).map(([value, example]) => ({value, label: example.label}));

// useLayoutEffect warns when rendered on the server, where nothing moves anyway.
const useClientLayoutEffect = typeof window === 'undefined' ? useEffect : useLayoutEffect;

function Row({order, reversed, caption}: {order: Item[]; reversed: Set<number>; caption: string}) {
  // The items slide from their old place to their new one (first, last, invert, play).
  const elements = useRef(new Map<number, HTMLDivElement>());
  const lefts = useRef(new Map<number, number>());
  const reduced = usePrefersReducedMotion();
  useClientLayoutEffect(() => {
    for (const [id, element] of elements.current) {
      const left = element.getBoundingClientRect().left;
      const old = lefts.current.get(id);
      lefts.current.set(id, left);
      if (old === undefined || old === left || reduced) continue;
      element.style.transition = 'none';
      element.style.transform = `translateX(${old - left}px)`;
      void element.offsetWidth; // apply the start position before the transition
      element.style.transition = 'transform 0.5s ease';
      element.style.transform = '';
    }
  });
  return (
    <div>
      <div className={styles.bidiLabel}>{caption}</div>
      <div className={styles.bidiRow}>
        {order.map((item) => (
          <div
            key={item.id}
            className={styles.bidiItem}
            ref={(element) => {
              if (element) elements.current.set(item.id, element);
              else elements.current.delete(item.id);
            }}>
            <span className={clsx(styles.bidiText, item.gap && styles.bidiGap, reversed.has(item.id) && styles.bidiReversed)} dir="auto">
              {item.gap ? ' ' : item.text}
            </span>
            <span className={clsx(styles.levelBadge, styles[`level${item.level}`])}>{item.level}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

export default function LinesBidi() {
  const [which, setWhich] = useState('hebrew');
  const example = examples[which];
  const steps = useMemo(() => reorderSteps(example.items, (item) => item.level), [example]);
  const stepper = useStepper(steps.length + 1, 1400);
  const index = stepper.index; // 0: the logical order; then one pass per level
  const order = index === 0 ? example.items : steps[index - 1].order;
  const reversed = new Set<number>();
  if (index > 0) {
    const before = index === 1 ? example.items : steps[index - 2].order;
    for (const [start, end] of steps[index - 1].runs) {
      for (const item of before.slice(start, end)) reversed.add(item.id);
    }
  }
  const highest = Math.max(...example.items.map((item) => item.level));
  const lowestOdd = Math.min(...example.items.map((item) => item.level | 1));

  let narration: string;
  if (index === 0) {
    narration = `The line in logical order, as it is stored and typed: a ${example.paragraph} paragraph meaning ${example.meaning}. Each fragment and gap has a level: even runs left to right, odd right to left. Highest level ${highest}, lowest odd level ${lowestOdd}.`;
  } else {
    const step = steps[index - 1];
    const runs = step.runs.length;
    narration = `Level ${step.level}: every run of items at level ${step.level} or higher is reversed (${runs} run${runs === 1 ? '' : 's'}, highlighted).`;
    if (index === steps.length) {
      narration += ' That was the lowest odd level: this is the order to draw, left to right. Each Hebrew word is still drawn from its right, by shaping.';
    }
  }

  return (
    <div className={styles.stage}>
      <Segmented options={options} value={which} onChange={setWhich} />
      <div style={{overflowX: 'auto'}}>
        <Row key={`${which}-logical`} order={example.items} reversed={new Set()} caption="logical order (offsets grow to the right)" />
        <Row key={which} order={order} reversed={reversed} caption={index === 0 ? 'before reordering' : `after the pass at level ${steps[index - 1].level}`} />
      </div>
      <StepperBar stepper={stepper} label="pass" />
      <Narration>{narration}</Narration>
    </div>
  );
}
