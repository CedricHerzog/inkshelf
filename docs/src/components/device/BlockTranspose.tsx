// copyTileWords' 4x4 block transpose (framebuffer_copy.go), step by step: four row words in,
// interleaveBytes twice, interleaveHalves twice, four column words out. The pixels are the
// letters a to p, so each byte's value is the letter's code (a = 0x61).

import React from 'react';
import clsx from 'clsx';
import {Narration, PixelGrid, StepperBar, useStepper} from '@site/src/components/kit';
import {bytesOfWord, hexWord, transposeBlock, wordFromBytes} from '@site/src/lib/framebufferRotation';
import styles from './device.module.css';

const letters = 'abcdefghijklmnop'.split('');
const code = (letter: string) => letter.charCodeAt(0);

const rows = [0, 1, 2, 3].map((row) => wordFromBytes(letters.slice(row * 4, row * 4 + 4).map(code))) as [
  number,
  number,
  number,
  number,
];
const block = transposeBlock(rows);

type WordName =
  | 'row0' | 'row1' | 'row2' | 'row3'
  | 'even01' | 'odd01' | 'even23' | 'odd23'
  | 'column0' | 'column1' | 'column2' | 'column3';

const values: Record<WordName, number> = {
  row0: rows[0], row1: rows[1], row2: rows[2], row3: rows[3],
  even01: block.even01, odd01: block.odd01, even23: block.even23, odd23: block.odd23,
  column0: block.column0, column1: block.column1, column2: block.column2, column3: block.column3,
};

type Frame = {
  made: WordName[];
  from: WordName[];
  code: string;
  text: React.ReactNode;
};

const frames: Frame[] = [
  {
    made: ['row0', 'row1', 'row2', 'row3'],
    from: [],
    code: 'row0 := *(*uint32)(source)\nrow1 := *(*uint32)(unsafe.Add(source, canvasStride))  // and row2, row3',
    text: (
      <>
        Four loads read the block’s four canvas rows, four pixels each. ARM is little-endian: the first pixel, <em>a</em>,
        lands in the <strong>low</strong> byte, so row0 prints as {hexWord(rows[0])}, “dcba” backwards.
      </>
    ),
  },
  {
    made: ['even01', 'odd01'],
    from: ['row0', 'row1'],
    code: 'even = first&0x00ff00ff | second<<8&0xff00ff00\nodd  = first>>8&0x00ff00ff | second&0xff00ff00',
    text: 'interleaveBytes(row0, row1): shift row1 one byte up and mask, so bytes 0 and 2 come from row0 and bytes 1 and 3 from row1. Pairs of pixels from two rows now sit side by side.',
  },
  {
    made: ['even23', 'odd23'],
    from: ['row2', 'row3'],
    code: 'even23, odd23 := interleaveBytes(row2, row3)',
    text: 'The same for rows 2 and 3.',
  },
  {
    made: ['column0', 'column2'],
    from: ['even01', 'even23'],
    code: 'low  = first&0x0000ffff | second<<16\nhigh = first>>16 | second&0xffff0000',
    text: 'interleaveHalves(even01, even23): take the low half of one and the low half of the other, then the high halves. a e + i m = a e i m: the block’s first column, top to bottom.',
  },
  {
    made: ['column1', 'column3'],
    from: ['odd01', 'odd23'],
    code: 'column1, column3 := interleaveHalves(odd01, odd23)',
    text: 'The odd words give columns 1 and 3. Four columns, from four rows: the block is transposed, with shifts, masks and ORs only.',
  },
  {
    made: [],
    from: ['column0', 'column1', 'column2', 'column3'],
    code: '*(*uint32)(destination) = column0\n*(*uint32)(unsafe.Add(destination, -framebufferStride)) = column1  // …2, 3',
    text: (
      <>
        Four stores write each column as one word into its framebuffer row: canvas column x goes to row 1403 − x, so
        column1 goes one row <em>up</em> (−stride). Then source moves 4 canvas rows down and destination 4 bytes right.
      </>
    ),
  },
  {
    made: [],
    from: [],
    code: '// per 4x4 block: 4 word loads, 4 word stores, a few shifts, masks and ORs\n// byte by byte:    16 byte loads, 16 byte stores into slow fb memory',
    text: 'Why bother: writing fb0 costs about 19 ns a byte on an InkPad 3, and Go on 32-bit ARM never merges byte stores into words. With four word stores instead of sixteen byte stores, a page turn’s copy takes about 22 ms, against 48 ms byte by byte.',
  },
];

const order: WordName[][] = [
  ['row0', 'row1', 'row2', 'row3'],
  ['even01', 'odd01', 'even23', 'odd23'],
  ['column0', 'column1', 'column2', 'column3'],
];

function Word({name, lit, source}: {name: WordName; lit: boolean; source: boolean}) {
  const bytes = bytesOfWord(values[name]);
  return (
    <div className={styles.word}>
      <span className={styles.wordName}>
        {name}
        {source ? ' ◂' : ''}
      </span>
      <span className={styles.wordBytes} style={source ? {outline: '2px dashed var(--ink-blue)'} : undefined}>
        {bytes.map((byte, index) => (
          <span key={index} className={clsx(styles.byte, lit && styles.byteLit)} title={`byte ${index}`}>
            {String.fromCharCode(byte)}
          </span>
        ))}
      </span>
      <span className={styles.wordHex}>{hexWord(values[name])}</span>
    </div>
  );
}

export default function BlockTranspose() {
  const stepper = useStepper(frames.length, 1800);
  const frame = frames[stepper.index];
  const madeSoFar = new Set(frames.slice(0, stepper.index + 1).flatMap((candidate) => candidate.made));
  const writing = stepper.index >= 5;

  // The framebuffer block, top row first: column3 is the row highest up (1403 - x - 3).
  const fbLetters = writing
    ? (['column3', 'column2', 'column1', 'column0'] as WordName[]).flatMap((name) =>
        bytesOfWord(values[name]).map((byte) => String.fromCharCode(byte)),
      )
    : new Array<string | undefined>(16).fill(undefined);

  return (
    <div className={styles.stack}>
      <div className={styles.row}>
        <div>
          <div className={styles.label}>Canvas block</div>
          <PixelGrid columns={4} cells={new Array(16).fill(235)} labels={letters} cellSize={26} ariaLabel="A 4 by 4 block of the canvas" />
        </div>
        <div>
          <div className={styles.label}>Framebuffer block</div>
          <PixelGrid
            columns={4}
            cells={new Array(16).fill(writing ? 235 : 255)}
            labels={fbLetters}
            cellSize={26}
            ariaLabel="The same block in the framebuffer"
          />
        </div>
      </div>
      <div className={styles.small + ' ' + styles.muted}>Each word: byte 0 (low) on the left, byte 3 (high) on the right; its value in hex below.</div>
      {order.map((group, groupIndex) => (
        <div key={groupIndex} className={styles.row} style={{gap: 0}}>
          {group
            .filter((name) => madeSoFar.has(name))
            .map((name) => (
              <Word key={name} name={name} lit={frame.made.includes(name)} source={frame.from.includes(name)} />
            ))}
        </div>
      ))}
      <div className={styles.code}>{frame.code}</div>
      <StepperBar stepper={stepper} label="step" />
      <Narration>{frame.text}</Narration>
    </div>
  );
}
