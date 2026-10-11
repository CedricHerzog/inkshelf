// A short tour of the app: emulator screenshots in a reader-shaped frame, one at a time, with
// a sentence each. Changing shot fades through white, a nod to the e-ink flash.

import React, {useEffect, useState} from 'react';
import clsx from 'clsx';
import useBaseUrl from '@docusaurus/useBaseUrl';
import {Button, Controls, usePrefersReducedMotion} from '@site/src/components/kit';
import styles from './PhoneTour.module.css';

type Shot = {file: string; title: string; caption: string};

const shots: readonly Shot[] = [
  {
    file: 'servers__01-servers.png',
    title: 'Pick a server',
    caption: 'The servers listed in config.json, here two fake ones run by the emulator.',
  },
  {
    file: 'library.png',
    title: 'A library',
    caption: 'A Calibre-Web library: the book to continue first, then the series, with filters on top.',
  },
  {
    file: 'series.png',
    title: 'A series',
    caption: 'The books of a series. A tap downloads one; another tap opens it.',
  },
  {
    file: 'comics.png',
    title: 'Comics too',
    caption: 'A Komga library of manga and BD, with a progress bar under each series.',
  },
  {
    file: 'cover.png',
    title: 'A cover',
    caption: 'A cover shown whole, in the panel’s 16 grays.',
  },
  {
    file: 'chapter.png',
    title: 'Reading an EPUB',
    caption: 'An EPUB laid out by Inkshelf itself: its CSS, justified lines, hyphens, pages.',
  },
  {
    file: 'calibreweb-epub__06-toolbar.png',
    title: 'The toolbar',
    caption: 'A tap in the middle of a page shows the toolbar: back, contents, font size, where you are.',
  },
  {
    file: 'calibreweb-links__02-footnote.png',
    title: 'Footnotes',
    caption: 'A tap on a note call opens the note in a pop-up at the bottom of the page, without leaving it.',
  },
  {
    file: 'calibreweb-mathml__02-formulas.png',
    title: 'Formulas and tables',
    caption: 'MathML formulas, a bordered box and a table, all drawn by the app.',
  },
  {
    file: 'mealie__04-page-1.png',
    title: 'A recipe',
    caption: 'A Mealie recipe, laid out into pages by the app, then read like a comic.',
  },
];

const autoplayMs = 4000;

export default function PhoneTour() {
  const [index, setIndex] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [flash, setFlash] = useState(false);
  const reducedMotion = usePrefersReducedMotion();
  const base = useBaseUrl('/img/shots/');

  const go = (target: number) => {
    const next = (target + shots.length) % shots.length;
    if (next === index) return;
    if (!reducedMotion) {
      setFlash(true);
      window.setTimeout(() => setFlash(false), 180);
    }
    setIndex(next);
  };

  useEffect(() => {
    if (!playing) return undefined;
    const timer = window.setTimeout(() => go(index + 1), autoplayMs);
    return () => window.clearTimeout(timer);
    // go only depends on index, which is listed.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playing, index]);

  const shot = shots[index];
  return (
    <div className={styles.widget}>
      <div className={styles.reader}>
        <div className={styles.screen}>
          <img
            src={base + shot.file}
            alt={`${shot.title}: ${shot.caption}`}
            width={702}
            height={936}
            className={styles.image}
          />
          <div className={clsx(styles.flash, flash && styles.flashOn)} />
        </div>
        <div className={styles.buttons} aria-hidden>
          <span />
          <span />
          <span />
          <span />
        </div>
      </div>
      <div className={styles.side}>
        <div className={styles.counter}>
          {index + 1} / {shots.length}
        </div>
        <div className={styles.title}>{shot.title}</div>
        <p className={styles.caption}>{shot.caption}</p>
        <Controls>
          <Button onClick={() => go(index - 1)} title="Previous screenshot">
            ‹ Previous
          </Button>
          <Button primary onClick={() => go(index + 1)} title="Next screenshot">
            Next ›
          </Button>
          <Button onClick={() => setPlaying(!playing)} active={playing}>
            {playing ? '❚❚ Stop' : '▶ Play'}
          </Button>
        </Controls>
        <div className={styles.dots}>
          {shots.map((item, dot) => (
            <button
              key={item.file}
              type="button"
              aria-label={item.title}
              className={clsx(styles.dot, dot === index && styles.dotOn)}
              onClick={() => go(dot)}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
