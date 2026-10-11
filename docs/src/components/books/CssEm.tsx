// From CSS lengths to pixels on the reader. A paragraph's font-size, text-indent and
// margin-left go through computeStyle (in hundredths of an em of the reader's font size, the
// font size clamped to 50–300%, margins capped at maxMargin), then layout turns hundredths into
// pixels at the reader's text size (Settings.em).

import React, {useEffect, useRef, useState} from 'react';
import clsx from 'clsx';
import {Controls, Segmented, Slider, Toggle} from '@site/src/components/kit';
import {
  computedFontSize,
  defaultTextSize,
  emPixels,
  fontSizeOf,
  lengthOf,
  maxFontSize,
  maxMargin,
  minFontSize,
  textSizes,
  textWidth,
} from '@site/src/lib/cssLength';
import styles from './Css.module.css';

type IndentUnit = '1.5em' | '24px' | '5%';

const barMaxPixels = 400;
const previewScale = 0.24;

function Bar({pixels, capped}: {pixels: number; capped?: boolean}) {
  return (
    <div className={styles.bar} aria-hidden>
      <div
        className={clsx(styles.barFill, capped && styles.barCapped)}
        style={{width: `${Math.min(100, (pixels / barMaxPixels) * 100)}%`}}
      />
    </div>
  );
}

export default function CssEm() {
  const [readerSize, setReaderSize] = useState<number>(defaultTextSize);
  const [fontEm, setFontEm] = useState(1.2);
  const [marginEm, setMarginEm] = useState(6);
  const [indentUnit, setIndentUnit] = useState<IndentUnit>('1.5em');
  const [fixedPage, setFixedPage] = useState(false);
  const inner = useRef<HTMLDivElement>(null);
  const [previewHeight, setPreviewHeight] = useState(160);

  const declaredFont = `${fontEm.toFixed(1)}em`;
  const unclamped = fontSizeOf(declaredFont, 100);
  const fontSize = computedFontSize(declaredFont, 100, fixedPage);
  const indent = Math.min(lengthOf(indentUnit, fontSize, textWidth) ?? 0, maxMargin);
  const marginWanted = lengthOf(`${marginEm}em`, fontSize, textWidth) ?? 0;
  const margin = Math.min(marginWanted, maxMargin);

  const fontPixels = emPixels(fontSize, readerSize);
  const indentPixels = emPixels(indent, readerSize);
  const marginPixels = emPixels(margin, readerSize);

  useEffect(() => {
    if (inner.current) setPreviewHeight(Math.ceil(inner.current.scrollHeight * previewScale));
  }, [fontPixels, indentPixels, marginPixels, readerSize]);

  let clampNote = '';
  if (fixedPage) clampNote = 'A pre-paginated page keeps the sizes it was designed with: no clamp.';
  else if (unclamped < minFontSize) clampNote = `${unclamped}% is too small to read: clamped up to ${minFontSize}%.`;
  else if (unclamped > maxFontSize) clampNote = `${unclamped}% is too big: clamped down to ${maxFontSize}%.`;

  return (
    <div>
      <Controls>
        <span style={{fontSize: '0.85rem'}}>The reader's text size (px):</span>
        <Segmented
          options={textSizes.map((size) => ({value: String(size), label: String(size)}))}
          value={String(readerSize)}
          onChange={(value) => setReaderSize(Number(value))}
        />
      </Controls>
      <Controls>
        <Slider label="p { font-size" value={fontEm} min={0.2} max={5} step={0.1} onChange={setFontEm} format={(value) => `${value.toFixed(1)}em }`} />
        <Slider label="margin-left" value={marginEm} min={0} max={12} step={0.5} onChange={setMarginEm} format={(value) => `${value}em`} />
      </Controls>
      <Controls>
        <span style={{fontSize: '0.85rem'}}>text-indent:</span>
        <Segmented
          options={[
            {value: '1.5em', label: '1.5em'},
            {value: '24px', label: '24px'},
            {value: '5%', label: '5%'},
          ]}
          value={indentUnit}
          onChange={setIndentUnit}
        />
        <Toggle label="on a pre-paginated page" checked={fixedPage} onChange={setFixedPage} />
      </Controls>
      <div className={styles.clampNote}>{clampNote}</div>
      <div className={styles.emRows}>
        <div className={styles.emRow}>
          <div>
            font size: <strong>{fontPixels} px</strong>
            <div className={styles.emMath}>
              {declaredFont} → {fontSize}% · {fontSize} × {readerSize} / 100
            </div>
          </div>
          <Bar pixels={fontPixels} capped={!fixedPage && unclamped !== fontSize} />
        </div>
        <div className={styles.emRow}>
          <div>
            text-indent: <strong>{indentPixels} px</strong>
            <div className={styles.emMath}>
              {indentUnit} → {indent}
              {indentUnit === '1.5em' && ` (1.5 × ${fontSize})`}
              {indentUnit === '24px' && ' (24 × 100 / 16, whatever the font size)'}
              {indentUnit === '5%' && ` (5% of the page, ${textWidth})`}
            </div>
          </div>
          <Bar pixels={indentPixels} />
        </div>
        <div className={styles.emRow}>
          <div>
            margin-left: <strong>{marginPixels} px</strong>
            <div className={styles.emMath}>
              {marginEm}em → {marginWanted}
              {marginWanted > maxMargin && ` → capped at ${maxMargin}`}
            </div>
          </div>
          <Bar pixels={marginPixels} capped={marginWanted > maxMargin} />
        </div>
      </div>
      <div className={styles.preview} style={{height: previewHeight}}>
        <div
          ref={inner}
          className={styles.previewInner}
          style={{transform: `scale(${previewScale})`, fontSize: readerSize}}>
          <p style={{margin: 0}}>
            A paragraph before, in the reader's own size, for comparison. It is long enough to
            wrap around once or twice on the page.
          </p>
          <p
            style={{
              margin: 0,
              fontSize: fontPixels,
              textIndent: indentPixels,
              marginLeft: marginPixels,
              lineHeight: 1.3,
            }}>
            The paragraph styled by the book: its size, its first-line indent and its left
            margin, in real InkPad 3 pixels.
          </p>
        </div>
      </div>
      <div className={styles.previewCaption}>
        The top of a 1404-pixel-wide page, drawn at {Math.round(previewScale * 100)}%.
      </div>
    </div>
  );
}
