// A whole book as a bar of chapters, sized by their rune counts (epub.Index.Chapters[].Chars).
// Dragging a position computes, live, what internal/reading/textbook.go reports: the offset from
// the book's start (global), Percent, and VirtualPage with shelf.CharsPerPage.

import React, {useRef, useState} from 'react';
import clsx from 'clsx';
import {Narration} from '@site/src/components/kit';
import {charsPerPage, globalOffset, locationOf, percent, virtualPage, virtualPages} from '@site/src/lib/positions';
import styles from './PagesWidgets.module.css';

const chapters = [
  {title: 'Cover', chars: 1},
  {title: 'Contents', chars: 412},
  {title: '1. The shelf', chars: 18240},
  {title: '2. Ink', chars: 25110},
  {title: '3. Lines', chars: 9730},
  {title: '4. Pages', chars: 31400},
  {title: '5. The last page', chars: 14980},
  {title: 'Notes', chars: 3120},
];
const counts = chapters.map((chapter) => chapter.chars);
const total = counts.reduce((sum, chars) => sum + chars, 0);
const pages = virtualPages(total);
const colors = ['var(--ink-paper-2)', 'var(--ink-blue-soft)', 'var(--ink-warn-soft)', 'var(--ink-ok-soft)'];

const number = (value: number) => value.toLocaleString('en');

export default function PagesBook() {
  const [global, setGlobal] = useState(Math.floor(total * 0.42));
  const bar = useRef<HTMLDivElement>(null);
  const location = locationOf(counts, global);
  const before = counts.slice(0, location.chapter);
  const at = globalOffset(counts, location);
  const share = percent(counts, location);
  const page = virtualPage(counts, location);

  const fromPointer = (clientX: number) => {
    const box = bar.current?.getBoundingClientRect();
    if (!box) return;
    const fraction = Math.min(Math.max((clientX - box.left) / box.width, 0), 0.99999);
    setGlobal(Math.floor(fraction * total));
  };

  let left = 0;
  return (
    <div className={styles.stage}>
      <div
        ref={bar}
        className={styles.ruler}
        style={{height: 54}}
        role="slider"
        aria-label="The book; drag to move the reading position"
        aria-valuemin={0}
        aria-valuemax={total - 1}
        aria-valuenow={global}
        tabIndex={0}
        onKeyDown={(event) => {
          if (event.key === 'ArrowRight') setGlobal((value) => Math.min(total - 1, value + charsPerPage));
          if (event.key === 'ArrowLeft') setGlobal((value) => Math.max(0, value - charsPerPage));
        }}
        onPointerDown={(event) => {
          event.currentTarget.setPointerCapture(event.pointerId);
          fromPointer(event.clientX);
        }}
        onPointerMove={(event) => {
          if (event.buttons) fromPointer(event.clientX);
        }}>
        {chapters.map((chapter, index) => {
          const start = left;
          left += chapter.chars;
          const width = (chapter.chars / total) * 100;
          return (
            <div
              key={chapter.title}
              className={clsx(styles.chapter, index === location.chapter && styles.chapterHit)}
              style={{left: `${(start / total) * 100}%`, width: `max(${width}%, 3px)`, background: colors[index % colors.length]}}
              title={`${chapter.title}: ${number(chapter.chars)} runes`}>
              {width > 9 ? chapter.title : ''}
            </div>
          );
        })}
        {Array.from({length: pages - 1}, (_, index) => (
          <div key={index} className={styles.virtualTick} style={{left: `${(((index + 1) * charsPerPage) / total) * 100}%`}} />
        ))}
        <div className={styles.pin} style={{left: `${(global / total) * 100}%`}} />
      </div>
      <div className={styles.formula}>
        <div>
          Location = chapter <b>{location.chapter}</b> ({chapters[location.chapter].title}), offset <b>{number(location.offset)}</b>
        </div>
        <div>
          global = {before.length > 0 ? before.map(number).join(' + ') + ' + ' : ''}
          {number(location.offset)} = <b>{number(at)}</b>
        </div>
        <div>
          Percent = {number(at)} × 100 / {number(total)} = <b>{share}%</b>
        </div>
        <div>
          VirtualPage = {number(at)} / {charsPerPage} + 1 = <b>{page}</b> of {pages}
        </div>
      </div>
      <Narration>
        The book has {number(total)} runes, so {pages} virtual pages of {charsPerPage} (the thin ticks). The cover is one
        rune: a picture counts as one. Both numbers use integer division, as in Go, and stay the same at every font size.
      </Narration>
    </div>
  );
}
