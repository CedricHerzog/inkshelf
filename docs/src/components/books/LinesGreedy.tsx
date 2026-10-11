// The greedy line breaker of internal/typesetting/layout/line_breaking.go (breakLines), run on a
// paragraph in the browser: a column width to drag, a step per word, and the branch of
// breakLines' switch that each step takes, lit in a small flowchart.

import React, {useEffect, useMemo, useState} from 'react';
import clsx from 'clsx';
import {Controls, Narration, Slider, StepperBar, Toggle, useStepper} from '@site/src/components/kit';
import {breakLines, placeLine, splitWords, type Measure, type Step, type Word} from '@site/src/lib/greedyLines';
import {points} from '@site/src/lib/liang';
import styles from './LinesWidgets.module.css';

const paragraph =
  'Inkshelf lays out a chapter again whenever the reader picks another font size, so its ' +
  'typesetting has to be quick. Each line takes as many words as fit; the next word starts a ' +
  'new line. A word that overflows may be hyphenated, and the narrow column of an e-reader ' +
  'makes that happen surprisingly often.';

const fontSize = 17;
const font = `${fontSize}px Georgia, 'Times New Roman', serif`;
const lineHeight = 27;
const pad = 14;
const indentPx = Math.round(fontSize * 1.5); // text-indent: 1.5em

/** useMeasure measures text in the widget's font: a guess while rendering on the server. */
function useMeasure(): Measure {
  const [measure, setMeasure] = useState<Measure>(() => (text: string) => Math.round(text.length * fontSize * 0.48));
  useEffect(() => {
    const context = document.createElement('canvas').getContext('2d');
    if (!context) return;
    const make = () => {
      context.font = font;
      const cache = new Map<string, number>();
      setMeasure(() => (text: string) => {
        let width = cache.get(text);
        if (width === undefined) {
          width = Math.round(context.measureText(text).width);
          cache.set(text, width);
        }
        return width;
      });
    };
    make();
    document.fonts?.ready.then(make).catch(() => undefined);
  }, []);
  return measure;
}

type Node = 'fits' | 'add' | 'hyph' | 'split' | 'has' | 'finish' | 'cut';

function litNodes(step: Step | undefined): Set<Node> {
  switch (step?.branch) {
    case 'fits':
      return new Set(['fits', 'add']);
    case 'hyphenate':
      return new Set(['fits', 'hyph', 'split']);
    case 'finish':
      return new Set(['fits', 'hyph', 'has', 'finish']);
    case 'cut':
      return new Set(['fits', 'hyph', 'has', 'cut']);
    default:
      return new Set();
  }
}

function Flowchart({step}: {step: Step | undefined}) {
  const lit = litNodes(step);
  const box = (id: Node, x: number, y: number, width: number, lines: string[]) => (
    <g key={id} className={clsx(styles.flowNode, lit.has(id) && styles.flowLit)}>
      <rect x={x} y={y} width={width} height={34} rx={id === 'fits' || id === 'hyph' || id === 'has' ? 14 : 4} />
      {lines.map((line, index) => (
        <text key={index} x={x + width / 2} y={y + 14 + index * 12 - (lines.length - 1) * 2} textAnchor="middle">
          {line}
        </text>
      ))}
    </g>
  );
  const edge = (from: Node, to: Node, d: string, label: string, lx: number, ly: number) => (
    <g key={`${from}-${to}`}>
      <path d={d} className={clsx(styles.flowEdge, lit.has(from) && lit.has(to) && styles.flowEdgeLit)} markerEnd="url(#lines-arrow)" />
      <text x={lx} y={ly} className={styles.flowLabel}>
        {label}
      </text>
    </g>
  );
  return (
    <svg viewBox="0 0 360 196" className={styles.svgFluid} style={{maxWidth: 460}} role="img" aria-label="The decision breakLines takes for each word">
      <defs>
        <marker id="lines-arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto">
          <path d="M0,0 L8,4 L0,8 z" fill="var(--ink-muted)" />
        </marker>
      </defs>
      {edge('fits', 'add', 'M150,19 L196,19', 'yes', 160, 14)}
      {edge('fits', 'hyph', 'M75,36 L75,52', 'no', 80, 48)}
      {edge('hyph', 'split', 'M150,71 L196,71', 'yes', 160, 66)}
      {edge('hyph', 'has', 'M75,88 L75,104', 'no', 80, 100)}
      {edge('has', 'finish', 'M150,123 L196,123', 'yes', 160, 118)}
      {edge('has', 'cut', 'M75,140 L75,158 L196,175', 'no', 80, 154)}
      {box('fits', 0, 2, 150, ['Does the word fit', 'on the line?'])}
      {box('add', 198, 2, 160, ['Add it to the line'])}
      {box('hyph', 0, 54, 150, ['Hyphenation on, and', 'word not tried yet?'])}
      {box('split', 198, 54, 160, ['Split at the last point', 'that fits with its hyphen'])}
      {box('has', 0, 106, 150, ['Does the line already', 'have words?'])}
      {box('finish', 198, 106, 160, ['Finish the line;', 'the word waits'])}
      {box('cut', 198, 158, 160, ['Wider than a line:', 'cut between characters'])}
    </svg>
  );
}

function narrate(step: Step | undefined, justify: boolean): string {
  if (!step) return '';
  const word = `“${step.candidate.text}”`;
  const room = step.available - step.used - step.gap;
  switch (step.branch) {
    case 'fits':
      return `${word} is ${step.candidate.width} px wide and ${Math.max(room, 0)} px are left after the ${step.gap ? 'space' : 'line start'}: it fits, so it goes on the line.`;
    case 'hyphenate': {
      const found = step.points ?? [];
      if (found.length === 0) {
        return `${word} does not fit (${Math.max(room, 0)} px left) and Liang's patterns find no point in it. It stays whole, marked as tried.`;
      }
      const marked = [...step.candidate.text].map((character, index) => (found.includes(index) ? '-' : '') + character).join('');
      if (!step.split) {
        return `${word} does not fit. It may break as ${marked}, but no start fits in ${Math.max(room, 0)} px with its hyphen. It stays whole, marked as tried.`;
      }
      return `${word} does not fit. It may break as ${marked}; the last start that fits in ${Math.max(room, 0)} px, hyphen included, is “${step.split[0].text}-”. Both parts go back to the front of the queue.`;
    }
    case 'finish':
      return step.movedBack
        ? `${word} does not fit. The line ended at a hyphen that did not fit after all, so its last part moves down with ${word}.`
        : `${word} does not fit and cannot be split here: the line is finished${justify ? ' and justified' : ''}. ${word} starts the next line.`;
    case 'cut': {
      const parts = step.cutParts ?? [];
      return parts.length > 1
        ? `${word} is alone on its line and still wider than the whole line (${step.available} px): it is cut after “${parts[0].text}”, the longest start that fits.`
        : `${word} is a single character wider than the line: it gets a line of its own anyway.`;
    }
  }
}

export default function LinesGreedy() {
  const measure = useMeasure();
  const [width, setWidth] = useState(300);
  const [justify, setJustify] = useState(true);
  const [hyphenate, setHyphenate] = useState(true);
  const [indent, setIndent] = useState(false);

  const words = useMemo(() => splitWords(paragraph, measure), [measure]);
  const result = useMemo(
    () =>
      breakLines(words, {
        width,
        indent: indent ? indentPx : 0,
        measure,
        hyphenPoints: hyphenate ? (text: string) => points('en', text) : null,
      }),
    [words, width, indent, hyphenate, measure],
  );
  const stepper = useStepper(result.steps.length, 450);
  const {seek} = stepper;
  useEffect(() => {
    // Open on the paragraph broken whole; Play starts it again from the first word.
    seek(result.steps.length - 1);
  }, [result]);

  const step = result.steps[stepper.index];
  const finalStep = stepper.index === result.steps.length - 1;
  const firstIndent = indent ? Math.min(indentPx, Math.floor(width / 2)) : 0;
  const rowLeft = (row: number) => pad + (row === 0 ? firstIndent : 0);
  const rowWidth = (row: number) => width - (row === 0 ? firstIndent : 0);
  const space = measure(' ');

  const finished = step?.lines ?? [];
  const current = step?.line ?? [];
  const rows = finished.length + (current.length > 0 ? 1 : 0) + 1;
  const svgWidth = pad * 2 + width + 120;
  const svgHeight = rows * lineHeight + 16;
  const baseline = (row: number) => 8 + row * lineHeight + fontSize;

  const drawLine = (line: Word[], row: number, justified: boolean, last: boolean) =>
    placeLine(line, {left: rowLeft(row), width: rowWidth(row), justify: justified, last, measure}).map((placed) => (
      <text
        key={`${row}-${placed.word.start}-${placed.word.text}`}
        x={placed.x}
        y={baseline(row)}
        className={placed.word === step?.candidate && step.branch === 'fits' ? styles.wordNew : styles.wordText}>
        {placed.word.text}
        {placed.hyphen ? '-' : ''}
      </text>
    ));

  // The word that does not fit, drawn where it would go.
  let ghost: React.ReactNode = null;
  if (step && step.branch !== 'fits') {
    const row = step.branch === 'finish' && !step.movedBack ? step.linesBefore + 1 : step.linesBefore;
    const x = step.branch === 'finish' ? rowLeft(row) : rowLeft(row) + step.used + step.gap;
    const right = pad + width;
    const prefix = (count: number) => measure([...step.candidate.text].slice(0, count).join(''));
    ghost = (
      <g>
        {step.branch !== 'finish' && x + step.candidate.width > right && (
          <rect x={Math.max(right, x)} y={baseline(row) - fontSize} width={x + step.candidate.width - Math.max(right, x)} height={fontSize + 6} fill="var(--ink-accent-soft)" />
        )}
        <text x={x} y={baseline(row)} className={styles.ghost} opacity={step.branch === 'finish' ? 0.55 : 1}>
          {step.candidate.text}
        </text>
        {step.branch === 'hyphenate' &&
          (step.points ?? []).map((point) => {
            const chosen = step.split && step.split[0].text.length === point;
            return (
              <g key={point}>
                <line x1={x + prefix(point)} x2={x + prefix(point)} y1={baseline(row) + 3} y2={baseline(row) + 9} stroke={chosen ? 'var(--ink-ok)' : 'var(--ink-accent)'} strokeWidth={chosen ? 3 : 1.5} />
              </g>
            );
          })}
        {step.branch === 'cut' && (step.cutParts?.length ?? 0) > 1 && (
          <line x1={x + prefix(step.cutParts![0].text.length)} x2={x + prefix(step.cutParts![0].text.length)} y1={baseline(row) - fontSize} y2={baseline(row) + 6} stroke="var(--ink-accent)" strokeWidth={2} strokeDasharray="3 2" />
        )}
      </g>
    );
  }

  const usedWidth = step ? (step.branch === 'fits' ? step.used + step.gap + step.candidate.width : step.used) : 0;

  return (
    <div className={styles.stage}>
      <Controls>
        <Slider label="Column" value={width} min={70} max={460} step={2} onChange={setWidth} format={(value) => `${value} px`} />
        <Toggle label="Justify" checked={justify} onChange={setJustify} />
        <Toggle label="Hyphenate" checked={hyphenate} onChange={setHyphenate} />
        <Toggle label="Indent first line" checked={indent} onChange={setIndent} />
      </Controls>
      <div style={{overflowX: 'auto'}}>
        <svg width={svgWidth} height={svgHeight} className={styles.svg} role="img" aria-label="The paragraph broken into lines">
          <rect x={pad} y={0} width={width} height={svgHeight} fill="var(--ink-paper-2)" />
          {firstIndent > 0 && <rect x={pad} y={8} width={firstIndent} height={lineHeight - 4} fill="var(--ink-blue-soft)" />}
          <line x1={pad} x2={pad} y1={0} y2={svgHeight} stroke="var(--ink-line)" strokeDasharray="4 3" />
          <line x1={pad + width} x2={pad + width} y1={0} y2={svgHeight} stroke="var(--ink-muted)" strokeDasharray="4 3" />
          <g style={{font}}>
            {finished.map((line, row) =>
              drawLine(line, row, justify, finalStep && current.length === 0 && row === finished.length - 1),
            )}
            {current.length > 0 && drawLine(current, finished.length, false, true)}
            {ghost}
          </g>
        </svg>
      </div>
      <StepperBar stepper={stepper} label="word step" />
      <Narration>{narrate(step, justify)}</Narration>
      <div className={styles.readout}>
        <span>
          line width <strong>{usedWidth}</strong> / {step?.available ?? width} px
        </span>
        <span>
          space <strong>{space}</strong> px
        </span>
        <span>
          lines <strong>{result.lines.length}</strong>
        </span>
        <span>
          words <strong>{words.length}</strong>
        </span>
      </div>
      <Flowchart step={step} />
    </div>
  );
}
