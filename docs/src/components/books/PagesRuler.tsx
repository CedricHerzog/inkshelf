// A chapter as a ruler of runes. The font size changes where its pages start (a rough model of
// the app's page, src/lib/positions.ts), while a bookmark stays at its rune offset; the page that
// shows it is found as layout.PageAt does (internal/typesetting/layout/page.go).

import React, {useRef, useState} from 'react';
import clsx from 'clsx';
import {Button, Controls, Narration, Slider} from '@site/src/components/kit';
import {charsPerLine, defaultTextSize, linesPerPage, modelPageStarts, pageAt, textSizes} from '@site/src/lib/positions';
import styles from './PagesWidgets.module.css';

/** The lengths of the chapter's paragraphs, in runes: long ones, and short lines of dialogue. */
const paragraphs = [
  420, 85, 640, 310, 1020, 150, 75, 530, 880, 260, 95, 700, 390, 1150, 210, 60, 480, 820, 340,
  130, 590, 970, 280, 110, 450, 760, 190, 520, 66, 905,
];
const total = paragraphs.reduce((sum, length) => sum + length, 0);
const chapter = 4;

const number = (value: number) => value.toLocaleString('en');

export default function PagesRuler() {
  const [sizeIndex, setSizeIndex] = useState(textSizes.indexOf(defaultTextSize));
  const [pin, setPin] = useState(6000);
  const ruler = useRef<HTMLDivElement>(null);
  const size = textSizes[sizeIndex];
  const starts = modelPageStarts(paragraphs, size);
  const hit = pageAt(starts, pin);
  const hitStart = starts[hit];
  const percentOf = (offset: number) => `${(offset / total) * 100}%`;

  const pinFromPointer = (clientX: number) => {
    const box = ruler.current?.getBoundingClientRect();
    if (!box) return;
    const at = Math.min(Math.max((clientX - box.left) / box.width, 0), 0.9999);
    setPin(Math.floor(at * total));
  };

  const ticks = [];
  for (let offset = 0; offset < total; offset += 2000) ticks.push(offset);

  return (
    <div className={styles.stage}>
      <Controls>
        <Slider label="Font size" value={sizeIndex} min={0} max={textSizes.length - 1} onChange={setSizeIndex} format={(value) => `${textSizes[value]} px`} />
        <Slider label="Bookmark" value={pin} min={0} max={total - 1} onChange={setPin} format={(value) => `rune ${number(value)}`} />
      </Controls>
      <div>
        <div
          ref={ruler}
          className={styles.ruler}
          role="slider"
          aria-label="The chapter's runes; click to move the bookmark"
          aria-valuemin={0}
          aria-valuemax={total - 1}
          aria-valuenow={pin}
          tabIndex={0}
          onKeyDown={(event) => {
            if (event.key === 'ArrowRight') setPin((value) => Math.min(total - 1, value + 100));
            if (event.key === 'ArrowLeft') setPin((value) => Math.max(0, value - 100));
          }}
          onPointerDown={(event) => {
            event.currentTarget.setPointerCapture(event.pointerId);
            pinFromPointer(event.clientX);
          }}
          onPointerMove={(event) => {
            if (event.buttons) pinFromPointer(event.clientX);
          }}>
          {starts.map((start, page) => {
            const end = starts[page + 1] ?? total;
            return (
              <div
                // Keyed by page number, so that a span slides to its new place when the size changes.
                key={page}
                className={clsx(styles.span, page % 2 ? styles.spanOdd : styles.spanEven, page === hit && styles.spanHit)}
                style={{left: percentOf(start), width: percentOf(end - start)}}>
                {(end - start) / total > 0.035 ? page + 1 : ''}
              </div>
            );
          })}
          <div className={styles.startMark} style={{left: percentOf(hitStart)}} title="The start of the page shown" />
          <div className={styles.pin} style={{left: percentOf(pin)}} title="The bookmark" />
        </div>
        <div className={styles.ticks}>
          {ticks.map((offset) => (
            <span key={offset} style={{left: percentOf(offset)}}>
              {number(offset)}
            </span>
          ))}
        </div>
      </div>
      <Controls>
        <Button onClick={() => setPin(hitStart)} disabled={pin === hitStart}>
          Save the page shown (move the bookmark to its start)
        </Button>
      </Controls>
      <Narration>
        The bookmark is <code>{`Location{Chapter: ${chapter}, Offset: ${number(pin)}}`}</code>. At {size} px the chapter
        has {starts.length} pages; <code>PageAt</code> finds page {hit + 1}, the last one starting at or before rune{' '}
        {number(pin)}: it starts at rune {number(hitStart)}
        {pin > hitStart ? `, ${number(pin - hitStart)} runes before the bookmark` : ', right on the bookmark'}.
      </Narration>
      <div className={styles.readout}>
        <span>
          chapter <strong>{number(total)}</strong> runes
        </span>
        <span>
          model: <strong>{linesPerPage(size)}</strong> lines × <strong>{charsPerLine(size)}</strong> characters ≈{' '}
          <strong>{number(linesPerPage(size) * charsPerLine(size))}</strong> per page
        </span>
      </div>
    </div>
  );
}
