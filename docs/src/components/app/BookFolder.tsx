// A book's folder on the reader, books/<id>/, as a listing: whole .pg4 pages, .tmp files being
// written (dashed, filling up), and book.json, written last.

import React from 'react';
import clsx from 'clsx';
import type {FileState} from '@site/src/lib/downloadSim';
import styles from './app.module.css';

export function pageName(page: number): string {
  return `${String(page).padStart(4, '0')}.pg4`;
}

export function BookFolder({
  files,
  pages,
  bookJson,
  title = 'books/b42/',
  skipped,
}: {
  files: ReadonlyMap<number, {state: FileState; progress: number}>;
  pages: number;
  bookJson: boolean;
  title?: string;
  /** Pages a resumed download found on disk and did not fetch again. */
  skipped?: ReadonlySet<number>;
}) {
  const listed = Array.from({length: pages}, (_, index) => index + 1).filter((page) => files.has(page));
  return (
    <div className={styles.folder}>
      <div className={styles.folderTitle}>📁 {title}</div>
      <div className={styles.files}>
        {listed.length === 0 && !bookJson && <span className={styles.muted}>(empty)</span>}
        {listed.map((page) => {
          const file = files.get(page)!;
          const temporary = file.state === 'tmp';
          return (
            <span
              key={`${page}-${file.state}`}
              className={clsx(
                styles.file,
                temporary ? styles.fileTmp : styles.fileNew,
                skipped?.has(page) && styles.fileSkip,
              )}
              title={temporary ? 'Being written: no one reads a .tmp file' : undefined}>
              {temporary && (
                <span className={styles.fileFill} style={{width: `${file.progress * 100}%`}} />
              )}
              <span className={styles.fileName}>
                {pageName(page)}
                {temporary ? '.tmp' : ''}
              </span>
            </span>
          );
        })}
        {bookJson && <span className={clsx(styles.file, styles.fileNew, styles.fileImportant)}>book.json</span>}
      </div>
    </div>
  );
}
