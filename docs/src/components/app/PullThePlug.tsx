// Pull the plug in the middle of a download, then resume it. A book of pages keeps its whole
// .pg4 files and skips them (shelf's pageOnDisk), a .tmp cut short is never read and is written
// again; an EPUB keeps book.epub.part, which starts over, or book.epub once renamed, which is
// not fetched again (fetchFile). Wi-Fi dropping instead trips the 30 s watchdog (stallTimeout),
// which gives up and removes the .part. book.json comes last, so the app (Store.Book) sees the
// book only once it is whole.

import React, {useRef, useState} from 'react';
import clsx from 'clsx';
import {Button, Controls, Segmented, useAnimationFrame} from '@site/src/components/kit';
import {
  newDownload,
  pullPlug,
  tick,
  type DownloadState,
} from '@site/src/lib/downloadSim';
import {BookFolder} from './BookFolder';
import styles from './app.module.css';

type Mode = 'pages' | 'epub';

export default function PullThePlug() {
  const [mode, setMode] = useState<Mode>('pages');
  return (
    <div>
      <Controls>
        <Segmented
          options={[
            {value: 'pages', label: 'A comic (pages)'},
            {value: 'epub', label: 'An EPUB (one file)'},
          ]}
          value={mode}
          onChange={setMode}
        />
      </Controls>
      {mode === 'pages' ? <PagesPlug /> : <EpubPlug />}
    </div>
  );
}

function AppSees({whole, id = 'b42'}: {whole: boolean; id?: string}) {
  return (
    <div className={styles.card} style={{marginTop: '0.5rem'}}>
      <span className={styles.mono}>Store.Book("{id}")</span>{' '}
      {whole ? (
        <strong style={{color: 'var(--ink-ok)'}}>→ found: the list says “Downloaded”</strong>
      ) : (
        <strong style={{color: 'var(--ink-accent)'}}>
          → false: no book.json, so not downloaded
        </strong>
      )}
    </div>
  );
}

/* ---------- A book of pages ---------- */

type PagesPhase = 'ready' | 'downloading' | 'unplugged' | 'done';

const pageCount = 10;

/** cutShort returns a download stopped while a page was half written, as the widget opens. */
function cutShort(): DownloadState {
  const state = newDownload(pageCount, 3, 2);
  for (let step = 0; step < 2000; step++) {
    tick(state, 16);
    const files = [...state.files.values()];
    const whole = files.filter((file) => file.state === 'pg4').length;
    const half = files.some((file) => file.state === 'tmp' && file.progress > 0.4);
    if (whole >= 3 && half) break;
  }
  return state;
}

function PagesPlug() {
  const download = useRef<DownloadState | null>(null);
  if (download.current === null) download.current = cutShort();
  const [phase, setPhase] = useState<PagesPhase>('unplugged');
  const [skipped, setSkipped] = useState<Set<number>>(new Set());
  const [, setFrame] = useState(0);
  const [message, setMessage] = useState(
    'The reader died in the middle of this download: whole pages, a .tmp cut short, no book.json. Tap to download again, or start a new download and pull the plug yourself.',
  );

  useAnimationFrame((elapsed) => {
    const current = download.current!;
    tick(current, Math.min(elapsed, 50) * 0.5);
    if (current.bookJson) {
      setPhase('done');
      setMessage(
        skipped.size > 0
          ? `Done. ${skipped.size} pages were already on disk and skipped; book.json was written last.`
          : 'Done: every page is on disk, then book.json is written, last.',
      );
    }
    setFrame((count) => count + 1);
  }, phase === 'downloading');

  const state = download.current!;
  const tmpLeft = [...state.files.values()].filter((file) => file.state === 'tmp').length;

  return (
    <div>
      <Controls>
        {phase !== 'downloading' ? (
          <Button
            primary
            onClick={() => {
              download.current = newDownload(pageCount, 3, 2);
              setSkipped(new Set());
              setPhase('downloading');
              setMessage('Downloading: fetchers and preparers at work.');
            }}>
            ▶ New download
          </Button>
        ) : null}
        <Button
          primary
          disabled={phase !== 'downloading'}
          onClick={() => {
            setPhase('unplugged');
            setMessage(
              tmpLeft > 0
                ? 'The reader died. The whole pages stay; a .tmp was cut short, but no page has that name: nobody will read it.'
                : 'The reader died. The whole pages stay, and there is no book.json.',
            );
          }}>
          🔌 Pull the plug
        </Button>
        <Button
          disabled={phase !== 'unplugged'}
          onClick={() => {
            const left = pullPlug(download.current!);
            const onDisk = new Set(
              [...left].filter(([, file]) => file.state === 'pg4').map(([page]) => page),
            );
            download.current = newDownload(pageCount, 3, 2, left);
            setSkipped(onDisk);
            setPhase('downloading');
            setMessage(
              'Tapped again: Download resumes. The fetchers skip the pages on disk (blue) and fetch the others; a .tmp left over is simply written again.',
            );
          }}>
          ↻ Tap to download again
        </Button>
      </Controls>
      <div className={styles.verdict} style={{fontSize: '0.92rem'}}>
        {phase === 'unplugged' ? '⚡ Unplugged' : phase === 'done' ? '✓ Downloaded' : phase === 'downloading' ? `Downloading ${state.done} of ${state.pages}` : 'Not downloaded'}
      </div>
      <BookFolder files={state.files} pages={state.pages} bookJson={state.bookJson} skipped={skipped} />
      <AppSees whole={state.bookJson} />
      <p className={styles.muted} style={{fontSize: '0.88rem', margin: '0.5rem 0 0'}}>
        {message}
      </p>
    </div>
  );
}

/* ---------- An EPUB ---------- */

type EpubPhase =
  | 'ready'
  | 'fetching'
  | 'stalled'
  | 'preparing'
  | 'cover'
  | 'done'
  | 'unplugged'
  | 'failed';

const fetchMs = 4000;
const prepareMs = 1600;
const coverMs = 700;
const stallSeconds = 30;
const watchdogSpeed = 10; // the watchdog's 30 s pass in 3 s here

type Epub = {
  part: number | null; // book.epub.part, and how much of it is written
  epub: boolean;
  index: boolean;
  cover: boolean;
  bookJson: boolean;
  progress: number; // through the current phase, 0..1
  watchdog: number; // seconds left before the watchdog gives up
};

const emptyEpub: Epub = {
  part: null,
  epub: false,
  index: false,
  cover: false,
  bookJson: false,
  progress: 0,
  watchdog: stallSeconds,
};

function EpubFiles({epub}: {epub: Epub}) {
  const file = (name: string, extra?: string, fill?: number) => (
    <span key={name} className={clsx(styles.file, fill !== undefined ? styles.fileTmp : styles.fileNew, extra)}>
      {fill !== undefined && <span className={styles.fileFill} style={{width: `${fill * 100}%`}} />}
      <span className={styles.fileName}>{name}</span>
    </span>
  );
  const nothing = epub.part === null && !epub.epub && !epub.bookJson;
  return (
    <div className={styles.folder}>
      <div className={styles.folderTitle}>📁 books/e7/</div>
      <div className={styles.files}>
        {nothing && <span className={styles.muted}>(empty)</span>}
        {epub.part !== null && file('book.epub.part', undefined, epub.part)}
        {epub.epub && file('book.epub')}
        {epub.index && file('index.json')}
        {epub.cover && file('cover.pg4')}
        {epub.bookJson && file('book.json', styles.fileImportant)}
      </div>
    </div>
  );
}

function EpubPlug() {
  const [epub, setEpub] = useState<Epub>({...emptyEpub, part: 0.45});
  const [phase, setPhase] = useState<EpubPhase>('unplugged');
  const [message, setMessage] = useState(
    'The reader died while this EPUB arrived: book.epub.part is half written. Tap to download again, or make the Wi-Fi drop during a download.',
  );
  const running = phase === 'fetching' || phase === 'stalled' || phase === 'preparing' || phase === 'cover';

  useAnimationFrame((elapsed) => {
    const step = Math.min(elapsed, 50);
    setEpub((current) => {
      switch (phase) {
        case 'fetching': {
          const part = Math.min(1, (current.part ?? 0) + step / fetchMs);
          if (part < 1) return {...current, part, watchdog: stallSeconds};
          setPhase('preparing');
          setMessage('Every byte is in: book.epub.part is renamed book.epub. Now the book is read through to build its index.');
          return {...current, part: null, epub: true, progress: 0, watchdog: stallSeconds};
        }
        case 'stalled': {
          const watchdog = current.watchdog - (step / 1000) * watchdogSpeed;
          if (watchdog > 0) return {...current, watchdog};
          setPhase('failed');
          setMessage('30 s without a byte: the watchdog cancels the request. The download fails with “the download stalled”, and the .part is removed.');
          return {...current, watchdog: 0, part: null};
        }
        case 'preparing': {
          const progress = current.progress + step / prepareMs;
          if (progress < 1) return {...current, progress};
          setPhase('cover');
          return {...current, index: true, progress: 0};
        }
        case 'cover': {
          const progress = current.progress + step / coverMs;
          if (progress < 1) return {...current, progress};
          setPhase('done');
          setMessage('The cover is prepared, then book.json is written, last: the book is downloaded.');
          return {...current, cover: true, bookJson: true, progress: 0};
        }
        default:
          return current;
      }
    });
  }, running);

  const start = () => {
    if (epub.epub) {
      setPhase('preparing');
      setMessage('book.epub is there already: fetchFile returns at once, nothing is fetched again. The index is built again.');
      setEpub({...epub, progress: 0, index: false});
      return;
    }
    setPhase('fetching');
    setMessage(
      epub.part !== null
        ? 'book.epub.part is created again, empty: a half-fetched EPUB starts over from the first byte.'
        : 'Fetching the whole EPUB into book.epub.part. Each read resets the 30 s watchdog.',
    );
    setEpub({...emptyEpub, part: 0});
  };

  return (
    <div>
      <Controls>
        <Button primary disabled={running} onClick={start}>
          {phase === 'ready' ? '▶ Download' : '↻ Tap to download again'}
        </Button>
        <Button
          primary
          disabled={!running}
          onClick={() => {
            setPhase('unplugged');
            setMessage(
              epub.epub
                ? 'The reader died after the rename: book.epub is whole, but there is no book.json yet.'
                : 'The reader died mid-file: book.epub.part stays, half written. Its name says it is not a book.',
            );
          }}>
          🔌 Pull the plug
        </Button>
        {phase === 'stalled' ? (
          <Button
            onClick={() => {
              setPhase('fetching');
              setMessage('Bytes flow again: the watchdog is reset to 30 s on the next read.');
            }}>
            📶 Wi-Fi back
          </Button>
        ) : (
          <Button
            disabled={phase !== 'fetching'}
            onClick={() => {
              setPhase('stalled');
              setMessage('No more bytes. Without a watchdog the request would wait forever; here it counts down from 30 s (shown 10× faster).');
            }}>
            📵 Wi-Fi drops
          </Button>
        )}
      </Controls>
      <div className={styles.verdict} style={{fontSize: '0.92rem'}}>
        {
          {
            ready: 'Not downloaded',
            fetching: `Downloading ${Math.round((epub.part ?? 0) * 100)}%`,
            stalled: `Stalled: watchdog ${Math.ceil(epub.watchdog)} s`,
            preparing: 'Preparing',
            cover: 'Preparing',
            done: '✓ Downloaded',
            unplugged: '⚡ Unplugged',
            failed: 'Download failed: the download stalled',
          }[phase]
        }
      </div>
      {phase === 'stalled' && (
        <div className={styles.bar} style={{marginBottom: '0.5rem'}}>
          <div className={styles.barFill} style={{width: `${(epub.watchdog / stallSeconds) * 100}%`}} />
        </div>
      )}
      <EpubFiles epub={epub} />
      <AppSees whole={epub.bookJson} id="e7" />
      <p className={styles.muted} style={{fontSize: '0.88rem', margin: '0.5rem 0 0'}}>
        {message}
      </p>
    </div>
  );
}
