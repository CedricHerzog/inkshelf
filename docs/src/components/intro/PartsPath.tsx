// The four parts of the site as a path, from the easiest to the hardest: a card each, with
// what it teaches and how steep it is.

import React from 'react';
import Link from '@docusaurus/Link';
import styles from './PartsPath.module.css';

const parts = [
  {
    number: 1,
    title: 'The big picture',
    to: '/big-picture/journey',
    teaches: 'A book’s whole trip, from the server to your eyes, and how the code is stacked in layers.',
    level: 1,
  },
  {
    number: 2,
    title: 'The screen',
    to: '/screen/eink',
    teaches: 'E-ink, 16 grays, drawing without the vendor’s SDK, the firmware, the event loop, the emulator.',
    level: 2,
  },
  {
    number: 3,
    title: 'Books',
    to: '/books/epub',
    teaches: 'What is inside an EPUB, how its CSS applies, how text is drawn in gray, how it becomes lines and lines become pages.',
    level: 3,
  },
  {
    number: 4,
    title: 'The app',
    to: '/app/tap',
    teaches: 'From a tap to a screen, one lock for many goroutines, three kinds of server, downloads, and syncing progress.',
    level: 3,
  },
] as const;

export default function PartsPath() {
  return (
    <ol className={styles.path}>
      {parts.map((part) => (
        <li key={part.number} className={styles.part}>
          <Link to={part.to} className={styles.card}>
            <span className={styles.number}>{part.number}</span>
            <span className={styles.body}>
              <span className={styles.title}>{part.title}</span>
              <span className={styles.teaches}>{part.teaches}</span>
            </span>
            <span className={styles.level} title={`Difficulty ${part.level} of 3`}>
              {'●'.repeat(part.level)}
              <span className={styles.levelOff}>{'●'.repeat(3 - part.level)}</span>
            </span>
          </Link>
        </li>
      ))}
    </ol>
  );
}
