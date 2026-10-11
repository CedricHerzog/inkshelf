// The nested tree of a chapter's XHTML morphs into the flat list of blocks of runs that
// Inkshelf's builder makes of it. The blocks, runs and offsets below are what
// epub.Book.Chapter returned for this XHTML and stylesheet (printed from the Go code), not a
// guess: note how the space before an inline element goes into that element's run.

import React, {useState} from 'react';
import clsx from 'clsx';
import {Button, Controls, usePrefersReducedMotion} from '@site/src/components/kit';
import styles from './Css.module.css';

const source = `<div class="chapter">
  <h1>Chapter 1</h1>
  <p class="first">Once upon a <em>time</em>, it rained.</p>
  <p>Hello <span class="it">world</span>!</p>
  <div class="box"><p>A boxed <b>note</b>.</p></div>
  <hr/>
</div>`;

const stylesheet = `p { text-indent: 1.5em; margin: 0 }
p.first { text-indent: 0 }
span.it { font-style: italic }
.box { border: 1px solid black; padding: 0.5em }`;

type Style = 'Bold' | 'Italic' | '';

// A text node of the tree and the run it becomes. treeText and runText differ only by where
// a space is; inline is the inline element the text is in, if any.
type Piece = {treeText: string; runText: string; style: Style; inline?: string};

type BlockSource = {
  tag: string; // the block element's start tag
  meta: string; // the Block's fields, as the builder set them
  start: number;
  pieces: Piece[];
  inBox?: boolean;
};

const blocks: BlockSource[] = [
  {
    tag: '<h1>',
    meta: 'Block 0 · Heading, Level 1 · Start 0 · FontScale 170 · MarginTop 100',
    start: 0,
    pieces: [{treeText: 'Chapter 1', runText: 'Chapter 1', style: 'Bold'}],
  },
  {
    tag: '<p class="first">',
    meta: 'Block 1 · Paragraph · Start 9 · Indent 0 · MarginTop 0',
    start: 9,
    pieces: [
      {treeText: 'Once upon a ', runText: 'Once upon a', style: ''},
      {treeText: 'time', runText: ' time', style: 'Italic', inline: '<em>'},
      {treeText: ', it rained.', runText: ', it rained.', style: ''},
    ],
  },
  {
    tag: '<p>',
    meta: 'Block 2 · Paragraph · Start 37 · Indent 150',
    start: 37,
    pieces: [
      {treeText: 'Hello ', runText: 'Hello', style: ''},
      {treeText: 'world', runText: ' world', style: 'Italic', inline: '<span class="it">'},
      {treeText: '!', runText: '!', style: ''},
    ],
  },
  {
    tag: '<p>',
    meta: 'Block 3 · Paragraph · Start 49 · Indent 150 · Left 56',
    start: 49,
    inBox: true,
    pieces: [
      {treeText: 'A boxed ', runText: 'A boxed', style: ''},
      {treeText: 'note', runText: ' note', style: 'Bold', inline: '<b>'},
      {treeText: '.', runText: '.', style: ''},
    ],
  },
  {tag: '<hr/>', meta: 'Block 4 · Rule · Start 62 · MarginTop 50 · MarginBottom 50 (counts 1)', start: 62, pieces: []},
];

const charWidth = 7.6; // of the stage's 12.5 px monospace font
const labelCharWidth = 6.4; // of the 10.5 px labels
const stageWidth = 480;

// Spaces at a run's edges are shown, to see which run took them.
const visible = (text: string) => text.replace(/^ | $/g, '␣');
const chipWidth = (text: string) => text.length * charWidth + 8;
const labelWidth = (text: string) => text.length * labelCharWidth + 12;

type Box = {x: number; y: number; width: number; height: number};
type FrameView = {key: string; label: string; kind: 'root' | 'block' | 'inline' | 'box'; box: Box; hidden?: boolean; labelRight?: boolean};
type ChipView = {key: string; text: string; style: Style; x: number; y: number};

// The source ranges of each piece's text, found in order.
const pieceRanges: [number, number][] = (() => {
  const ranges: [number, number][] = [];
  let cursor = 0;
  for (const block of blocks) {
    for (const piece of block.pieces) {
      const at = source.indexOf(piece.treeText, cursor);
      ranges.push([at, at + piece.treeText.length]);
      cursor = at + piece.treeText.length;
    }
  }
  return ranges;
})();

function layoutTree(): {frames: FrameView[]; chips: ChipView[]; height: number} {
  const frames: FrameView[] = [];
  const chips: ChipView[] = [];
  let y = 22;
  let chipIndex = 0;
  blocks.forEach((block, blockIndex) => {
    let x = 12;
    let width = stageWidth - 24;
    if (block.inBox) {
      frames.push({key: 'box', label: '<div class="box">', kind: 'box', box: {x: 12, y, width, height: 58}});
      x += 12;
      width -= 24;
      y += 20;
    }
    const rowHeight = block.pieces.length ? 32 : 22;
    frames.push({key: `block-${blockIndex}`, label: block.tag, kind: 'block', box: {x, y, width, height: rowHeight}});
    let cursor = x + labelWidth(block.tag);
    for (const piece of block.pieces) {
      if (piece.inline) {
        const inlineLabel = labelWidth(piece.inline);
        frames.push({
          key: `inline-${chipIndex}`,
          label: piece.inline,
          kind: 'inline',
          box: {x: cursor, y: y + 3, width: inlineLabel + chipWidth(visible(piece.treeText)) + 4, height: 26},
        });
        cursor += inlineLabel;
      }
      chips.push({key: `chip-${chipIndex}`, text: visible(piece.treeText), style: piece.style, x: cursor, y: y + 6});
      cursor += chipWidth(visible(piece.treeText)) + (piece.inline ? 10 : 4);
      chipIndex++;
    }
    y += rowHeight + 6 + (block.inBox ? 12 : 0);
  });
  frames.unshift({key: 'root', label: '<div class="chapter">', kind: 'root', box: {x: 0, y: 0, width: stageWidth, height: y + 6}});
  return {frames, chips, height: y + 8};
}

function layoutFlat(): {frames: FrameView[]; chips: ChipView[]; height: number} {
  const frames: FrameView[] = [];
  const chips: ChipView[] = [];
  let y = 24;
  let chipIndex = 0;
  blocks.forEach((block, blockIndex) => {
    const rowHeight = block.pieces.length ? 42 : 22;
    if (block.inBox) {
      frames.push({
        key: 'box',
        label: 'BorderBox · Start 49 · End 62',
        kind: 'box',
        box: {x: 6, y: y - 4, width: stageWidth - 12, height: rowHeight + 8},
        labelRight: true,
      });
    }
    frames.push({key: `block-${blockIndex}`, label: block.meta, kind: 'block', box: {x: 12, y, width: stageWidth - 24, height: rowHeight}});
    let x = 20;
    for (const piece of block.pieces) {
      if (piece.inline) {
        frames.push({key: `inline-${chipIndex}`, label: piece.inline, kind: 'inline', box: {x, y: y + 16, width: chipWidth(visible(piece.runText)), height: 20}, hidden: true});
      }
      chips.push({key: `chip-${chipIndex}`, text: visible(piece.runText), style: piece.style, x, y: y + 17});
      x += chipWidth(visible(piece.runText)) + 3;
      chipIndex++;
    }
    y += rowHeight + 8;
  });
  frames.unshift({key: 'root', label: 'Document · Chars 63 · Boxes: 1', kind: 'root', box: {x: 0, y: 0, width: stageWidth, height: y + 4}});
  return {frames, chips, height: y + 6};
}

const treeLayout = layoutTree();
const flatLayout = layoutFlat();

// Each chip's run, with its offsets in the document.
const runs = (() => {
  const all: {block: number; text: string; style: Style; from: number; to: number}[] = [];
  blocks.forEach((block, blockIndex) => {
    let offset = block.start;
    for (const piece of block.pieces) {
      const length = [...piece.runText].length;
      all.push({block: blockIndex, text: piece.runText, style: piece.style, from: offset, to: offset + length});
      offset += length;
    }
  });
  return all;
})();

const allPieces = blocks.flatMap((block) => block.pieces);

function SourceView({picked}: {picked: number}) {
  // The run's text in the source: a trailing space goes to the next run, which takes it.
  const piece = allPieces[picked];
  const [from, end] = pieceRanges[picked];
  const to = piece.treeText.endsWith(' ') && !piece.runText.endsWith(' ') ? end - 1 : end;
  const ranges: [number, number][] = [[from, to]];
  if (piece.runText.startsWith(' ') && !piece.treeText.startsWith(' ')) {
    const previousEnd = pieceRanges[picked - 1][1];
    ranges.push([previousEnd - 1, previousEnd]);
  }
  const hot = (index: number) => ranges.some(([rangeFrom, rangeTo]) => index >= rangeFrom && index < rangeTo);
  const pieces: React.ReactNode[] = [];
  let start = 0;
  for (let index = 1; index <= source.length; index++) {
    if (index === source.length || hot(index) !== hot(start)) {
      pieces.push(
        <span key={start} className={hot(start) ? styles.sourceHot : undefined}>
          {source.slice(start, index)}
        </span>,
      );
      start = index;
    }
  }
  return <pre className={styles.flatSource}>{pieces}</pre>;
}

export default function CssFlatten() {
  const [flat, setFlat] = useState(false);
  const [picked, setPicked] = useState(5); // " world"
  const reduced = usePrefersReducedMotion();
  const layout = flat ? flatLayout : treeLayout;
  const noMotion = reduced ? {transition: 'none'} : undefined;
  const run = runs[picked];

  return (
    <div>
      <Controls>
        <Button primary onClick={() => setFlat(!flat)}>
          {flat ? '◂ Back to the tree' : 'Flatten ▸'}
        </Button>
        <span style={{fontSize: '0.85rem', color: 'var(--ink-muted)'}}>
          {flat ? 'The document: a flat list of blocks of runs.' : 'The XHTML: elements nested in elements.'}
        </span>
      </Controls>
      <div className={styles.flatStage} style={{height: layout.height, width: stageWidth, ...noMotion}}>
        {layout.frames.map((frame) => (
          <div
            key={frame.key}
            className={clsx(
              styles.frame,
              frame.kind === 'inline' && styles.frameInline,
              frame.kind === 'block' && styles.frameBlock,
              frame.kind === 'box' && styles.frameBox,
            )}
            style={{
              left: frame.box.x,
              top: frame.box.y,
              width: frame.box.width,
              height: frame.box.height,
              opacity: frame.hidden ? 0 : 1,
              ...noMotion,
            }}>
            <span
              className={styles.frameLabel}
              style={{
                ...(frame.kind === 'inline' || (frame.kind === 'block' && !flat) ? {top: '50%', transform: 'translateY(-50%)'} : {}),
                ...(frame.labelRight ? {left: 'auto', right: 6, top: -1} : {}),
                ...(frame.kind === 'box' && !flat ? {top: 2} : {}),
              }}>
              {frame.label}
            </span>
          </div>
        ))}
        {layout.chips.map((chip, index) => (
          <button
            key={chip.key}
            type="button"
            className={clsx(
              styles.chip,
              chip.style === 'Italic' && styles.chipItalic,
              chip.style === 'Bold' && styles.chipBold,
              index === picked && styles.chipPicked,
            )}
            style={{left: chip.x, top: chip.y, ...noMotion}}
            onClick={() => setPicked(index)}>
            {chip.text}
          </button>
        ))}
      </div>
      <div className={styles.runDetail} style={{marginTop: '0.5rem'}}>
        Run{'{'}Text: "{run.text}"{run.style && `, Style: ${run.style}`}{'}'} in block {run.block}, runes [{run.from},{' '}
        {run.to}) of the chapter
      </div>
      <SourceView picked={picked} />
      <pre className={styles.flatSource} style={{marginTop: '0.4rem'}}>{stylesheet}</pre>
    </div>
  );
}
