// An XHTML snippet read token by token, as Inkshelf's chapter builder reads a chapter. The
// stack of open elements grows and shrinks; when an element opens, what its selectors may look
// at lights up (its ancestors, its previous siblings) and what follows is still unread.

import React, {useMemo, type ReactNode} from 'react';
import clsx from 'clsx';
import {Narration, StepperBar, usePrefersReducedMotion, useStepper} from '@site/src/components/kit';
import {knownRelatives, stream, tokenize, type Frame, type StreamElement} from '@site/src/lib/xhtmlStream';
import {matches, parseSelector} from '@site/src/lib/cssCascade';
import styles from './Css.module.css';

const source = `<body>
  <h1>Chapter 1</h1>
  <p class="first">Once upon a time.</p>
  <p>It was <em>dark</em>.</p>
  <p>The end.</p>
</body>`;

type Mark = 'future' | 'current' | 'ancestor' | 'sibling' | '';

function marksOf(frame: Frame): Mark[] {
  const marks: Mark[] = Array.from(source, () => '');
  const paint = (from: number, to: number, mark: Mark) => {
    for (let index = from; index < to; index++) marks[index] = mark;
  };
  if (frame.opened) {
    const {ancestors, previous} = knownRelatives(frame.opened);
    for (const sibling of previous) paint(sibling.from, sibling.to ?? sibling.startTo, 'sibling');
    for (const ancestor of ancestors) paint(ancestor.from, ancestor.startTo, 'ancestor');
  }
  paint(frame.token.from, frame.token.to, 'current');
  paint(frame.token.to, source.length, 'future');
  return marks;
}

function Source({frame}: {frame: Frame}) {
  const marks = marksOf(frame);
  const pieces: ReactNode[] = [];
  let start = 0;
  for (let index = 1; index <= source.length; index++) {
    if (index === source.length || marks[index] !== marks[start]) {
      const mark = marks[start];
      pieces.push(
        <span key={start} className={mark ? styles[mark] : undefined}>
          {source.slice(start, index)}
        </span>,
      );
      start = index;
    }
  }
  return <pre className={styles.source}>{pieces}</pre>;
}

function tag(element: StreamElement) {
  return `<${element.name}${element.classes.length ? ` class="${element.classes.join(' ')}"` : ''}>`;
}

type Verdict = 'yes' | 'no' | 'unknown';

function verdicts(element: StreamElement): {selector: string; verdict: Verdict}[] {
  const name = element.name;
  return [`${name}:first-child`, `h1 + ${name}`, `body > ${name}`, `${name}:last-child`, `${name}:empty`].map(
    (selector) => {
      const parsed = parseSelector(selector);
      if ('dropped' in parsed) return {selector, verdict: 'unknown'};
      return {selector, verdict: matches(parsed.selector, element) ? 'yes' : 'no'};
    },
  );
}

function narrate(frame: Frame, index: number, count: number): ReactNode {
  const {token, opened, closed} = frame;
  if (opened) {
    const {ancestors, previous} = knownRelatives(opened);
    return (
      <>
        <code>{tag(opened)}</code> opens and is pushed on the stack. Its style is computed{' '}
        <em>now</em>, before anything inside it or after it is read. Its selectors can see{' '}
        {ancestors.length} ancestor{ancestors.length === 1 ? '' : 's'} and {previous.length}{' '}
        previous sibling{previous.length === 1 ? '' : 's'}. Whether it is the last child, or
        empty, is unknown yet.
      </>
    );
  }
  if (token.kind === 'text') {
    const text = source.slice(token.from, token.to);
    return (
      <>
        Text “{text.trim()}” goes straight into a run, in the style of{' '}
        <code>&lt;{frame.stack[frame.stack.length - 1]?.name}&gt;</code>, the top of the stack.
        Then it is forgotten: the builder never keeps the tree.
      </>
    );
  }
  if (closed) {
    const last = index === count - 1;
    return last ? (
      <>
        <code>&lt;/body&gt;</code> closes. Only now could anyone tell that “The end.” was the
        last child of <code>&lt;body&gt;</code>: far too late, its paragraph was styled and
        built a few steps ago. That is why Inkshelf drops <code>:last-child</code> and{' '}
        <code>:empty</code>.
      </>
    ) : (
      <>
        <code>&lt;/{closed.name}&gt;</code> closes: popped off the stack. It becomes its
        parent's last child, so the next element to open there will see it as its previous
        sibling. Its own children are forgotten.
      </>
    );
  }
  return null;
}

export default function CssStream() {
  const {frames} = useMemo(() => stream(tokenize(source)), []);
  const stepper = useStepper(frames.length, 1500);
  const reduced = usePrefersReducedMotion();
  const frame = frames[stepper.index];
  const top = frame.stack[frame.stack.length - 1];

  return (
    <div>
      <div className={styles.streamGrid}>
        <div>
          <Source frame={frame} />
          <div className={styles.legend}>
            <span>
              <span className={styles.swatch} style={{background: 'var(--ink-accent)'}} />
              the token read
            </span>
            <span>
              <span className={styles.swatch} style={{background: 'var(--ink-blue-soft)', outline: '1px solid var(--ink-blue)'}} />
              ancestors
            </span>
            <span>
              <span className={styles.swatch} style={{background: 'var(--ink-ok-soft)'}} />
              previous siblings
            </span>
            <span>
              <span className={styles.swatch} style={{background: 'var(--ink-line)', opacity: 0.5}} />
              not read yet
            </span>
          </div>
        </div>
        <div>
          <div className={styles.stackTitle}>The stack</div>
          <div className={styles.stack}>
            <div className={clsx(styles.stackItem, styles.stackBottom)}>#document</div>
            {frame.stack.map((element) => (
              <div
                key={element.key}
                className={clsx(styles.stackItem, element === top && styles.stackItemTop, reduced && styles.noMotion)}>
                {tag(element)}
              </div>
            ))}
          </div>
        </div>
      </div>
      {frame.opened && (
        <div className={styles.badges}>
          {verdicts(frame.opened).map(({selector, verdict}) => (
            <span
              key={selector}
              className={clsx(
                styles.badge,
                verdict === 'yes' && styles.badgeYes,
                verdict === 'no' && styles.badgeNo,
                verdict === 'unknown' && styles.badgeUnknown,
              )}>
              {selector} {verdict === 'yes' ? '✓' : verdict === 'no' ? '✗' : '? unknown yet'}
            </span>
          ))}
        </div>
      )}
      <StepperBar stepper={stepper} label="token" />
      <Narration>{narrate(frame, stepper.index, frames.length)}</Narration>
    </div>
  );
}
