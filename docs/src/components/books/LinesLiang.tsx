// Liang's hyphenation (internal/typesetting/hyphen, Hyphenator.hyphenate), step by step: the
// patterns found in ".word." line up under it one at a time, their digits drop into the gaps,
// each gap keeps the highest, odd digits become hyphens, and leftMin/rightMin keep the ends whole.

import React, {useMemo, useState} from 'react';
import clsx from 'clsx';
import {Button, Controls, Narration, Segmented, StepperBar, useStepper} from '@site/src/components/kit';
import {minimums, patternCount, traceWord, type Language} from '@site/src/lib/liang';
import styles from './LinesWidgets.module.css';

const presets: Record<Language, string[]> = {
  en: ['hyphenation', 'computer', 'algorithm', 'typesetting', 'typography', 'reflowable'],
  fr: ['ordinateur', 'hyphénation', 'bibliothèque'],
  de: ['Silbentrennung', 'Rechtschreibung', 'Donaudampfschifffahrt'],
};

const languages: {value: Language; label: string}[] = [
  {value: 'en', label: 'English'},
  {value: 'fr', label: 'French'},
  {value: 'de', label: 'German'},
];

const cell = 26;
const left = 16;
const rowHeight = 24;
const shownRows = 7;

export default function LinesLiang() {
  const [language, setLanguage] = useState<Language>('en');
  const [typed, setTyped] = useState('hyphenation');

  const letters = typed.trim().toLowerCase().slice(0, 24);
  const valid = /^[\p{L}\p{Mn}]+$/u.test(letters);
  const trace = useMemo(() => traceWord(language, valid ? letters : ''), [language, letters, valid]);
  const {leftMin, rightMin, file} = minimums[language];
  const count = [...letters].length;
  const matches = trace.matches;
  const stepper = useStepper(Math.max(matches.length + 2, 1), 900);
  const index = stepper.index;
  const shownMatches = Math.min(index + 1, matches.length);
  const summing = index >= matches.length; // all patterns found: the max, then the hyphens
  const final = index >= matches.length + 1 || trace.exception || trace.tooShort;

  // The level of each gap after the patterns shown so far.
  const levels = useMemo(() => {
    const running = new Array<number>(trace.dotted.length + 1).fill(0);
    for (const match of matches.slice(0, shownMatches)) {
      match.pattern.values.forEach((value, gap) => {
        running[match.at + gap] = Math.max(running[match.at + gap], value);
      });
    }
    return running;
  }, [matches, shownMatches, trace.dotted.length]);

  const current = matches[shownMatches - 1];
  const firstRow = Math.max(0, shownMatches - shownRows);
  const rows = matches.slice(firstRow, shownMatches);
  const dotted = trace.dotted;
  const original = [...typed.trim()].slice(0, 24);
  const gapX = (gap: number) => left + gap * cell;
  const letterX = (position: number) => left + position * cell + cell / 2;
  const top = 8;
  const lettersY = top + 22;
  const rowsTop = lettersY + 14 + (firstRow > 0 ? 16 : 0);
  const maxY = rowsTop + rows.length * rowHeight + 22;
  const height = maxY + 34;
  const width = left * 2 + dotted.length * cell;

  const allowed = (gap: number) => {
    const point = gap - 1; // the gap before dotted[gap] follows the word's first gap-1 letters
    return point >= leftMin && point <= count - rightMin;
  };

  let narration: string;
  if (!valid) {
    narration = 'Type one word of letters only: Points finds no point in a word with digits or a hyphen inside.';
  } else if (trace.tooShort) {
    narration = `“${letters}” has ${count} letters, fewer than leftMin + rightMin = ${leftMin + rightMin}: it is never hyphenated.`;
  } else if (trace.exception) {
    narration = `“${letters}” is in hyph-en-us.hyp.txt, the words hyphenated by hand: the patterns are not used.`;
  } else if (matches.length === 0) {
    narration = 'No pattern this page carries matches this word (the page has only a few of them): no hyphen.';
  } else if (!summing) {
    const changed = current.pattern.values.filter((value) => value > 0).length;
    narration = `Pattern ${shownMatches} of ${matches.length}: “${current.pattern.text}” is found at position ${current.at} of “${dotted.join('')}”. Its ${changed} digit${changed === 1 ? '' : 's'} drop${changed === 1 ? 's' : ''} into the gaps it covers; a gap keeps the higher digit.`;
  } else if (!final) {
    narration = 'Every pattern has spoken. Each gap kept the highest digit it was given: odd means a hyphen may go there, even (or none) means it may not.';
  } else {
    narration = `leftMin ${leftMin} and rightMin ${rightMin}: no hyphen leaves fewer than ${leftMin} letters before it or ${rightMin} after it. What is left: ${trace.points.length} point${trace.points.length === 1 ? '' : 's'}.`;
  }

  const result = original.map((character, position) => (
    <React.Fragment key={position}>
      {final && trace.points.includes(position) && <span className={styles.hyphen}>-</span>}
      {character}
    </React.Fragment>
  ));

  const carried = patternCount(language);
  const isPreset = presets[language].some((word) => word.toLowerCase() === letters);

  return (
    <div className={styles.stage}>
      <Controls>
        <Segmented
          options={languages}
          value={language}
          onChange={(value) => {
            setLanguage(value);
            setTyped(presets[value][0]);
          }}
        />
        <input
          className={styles.wordInput}
          value={typed}
          aria-label="A word to hyphenate"
          onChange={(event) => setTyped(event.target.value)}
          spellCheck={false}
        />
      </Controls>
      <Controls>
        {presets[language].map((word) => (
          <Button key={word} active={word.toLowerCase() === letters} onClick={() => setTyped(word)}>
            {word}
          </Button>
        ))}
      </Controls>
      <div style={{overflowX: 'auto'}}>
        <svg width={width} height={height} className={clsx(styles.svg, styles.liangGrid)} role="img" aria-label={`Liang's patterns over ${letters}`}>
          {final &&
            !trace.exception &&
            dotted.slice(1).map((_, offset) => {
              const gap = offset + 1;
              if (gap >= dotted.length || allowed(gap)) return null;
              return <rect key={`shade-${gap}`} x={gapX(gap) - cell / 2} y={top} width={cell} height={maxY - top + 8} className={styles.edgeShade} />;
            })}
          {dotted.map((character, position) => (
            <text key={`letter-${position}`} x={letterX(position)} y={lettersY} textAnchor="middle" className={character === '.' ? styles.cellDot : styles.cellLetter}>
              {character}
            </text>
          ))}
          {firstRow > 0 && (
            <text x={left} y={lettersY + 22} className={styles.digit}>
              … {firstRow} earlier pattern{firstRow === 1 ? '' : 's'}
            </text>
          )}
          {rows.map((match, row) => {
            const isCurrent = !summing && match === current;
            const y = rowsTop + row * rowHeight;
            const length = [...match.pattern.letters].length;
            return (
              <g key={`${match.at}-${match.pattern.text}`} className={clsx(styles.patternRow, isCurrent ? styles.patternCurrent : styles.patternOld, isCurrent && styles.slideIn)}>
                <rect x={gapX(match.at) + 2} y={y + 2} width={length * cell - 4} height={rowHeight - 4} rx={4} />
                {[...match.pattern.letters].map((character, position) => (
                  <text key={position} x={letterX(match.at + position)} y={y + 16} textAnchor="middle" fill="var(--ink-muted)">
                    {character}
                  </text>
                ))}
                {match.pattern.values.map((value, gap) =>
                  value > 0 ? (
                    <text key={`v${gap}`} x={gapX(match.at + gap)} y={y + 16} textAnchor="middle" className={clsx(styles.digit, value % 2 ? styles.digitOdd : styles.digitEven)}>
                      {value}
                    </text>
                  ) : null,
                )}
              </g>
            );
          })}
          <line x1={left} x2={width - left} y1={maxY - 16} y2={maxY - 16} stroke="var(--ink-line)" />
          <text x={2} y={maxY + 2} className={styles.digit}>
            max
          </text>
          {levels.map((level, gap) => {
            if (gap === 0 || gap >= dotted.length) return null;
            const odd = level % 2 === 1;
            const shown = summing ? odd : true;
            return (
              <text
                key={`max-${gap}-${level}`}
                x={gapX(gap)}
                y={maxY + 2}
                textAnchor="middle"
                className={clsx(styles.digit, level > 0 && (odd ? styles.digitOdd : styles.digitEven), level > 0 && styles.dropIn)}
                opacity={shown || level === 0 ? 1 : 0.35}>
                {level}
              </text>
            );
          })}
          {final &&
            trace.points.map((point) => (
              <text key={`hy-${point}`} x={gapX(point + 1)} y={maxY + 24} textAnchor="middle" fill="var(--ink-accent)" fontWeight={700} className={styles.dropIn}>
                -
              </text>
            ))}
        </svg>
      </div>
      <StepperBar stepper={stepper} label="step" />
      <Narration>{narration}</Narration>
      <div className={styles.result}>{valid ? result : typed}</div>
      <div className={styles.note}>
        {file}.pat.txt has {carried.total.toLocaleString('en')} patterns; this page carries the {carried.carried} that
        match its example words, so those hyphenate exactly as on the reader.
        {!isPreset && valid && ' Another word may miss a pattern here, and come out differently on the reader.'}
      </div>
    </div>
  );
}
