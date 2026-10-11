// The download of a book of pages as a pipeline: the server, the fetchers, a queue of two, the
// preparers, and the files appearing in the book's folder. The model is src/lib/downloadSim.ts
// (shelf.Store.Download and its downloadRun).

import React, {useRef, useState} from 'react';
import clsx from 'clsx';
import {
  Button,
  Controls,
  Slider,
  useAnimationFrame,
} from '@site/src/components/kit';
import {
  fetchedQueue,
  newDownload,
  prepareStages,
  tick,
  type DownloadState,
} from '@site/src/lib/downloadSim';
import styles from './app.module.css';
import {BookFolder} from './BookFolder';

const pageCount = 12;
const speed = 0.6;

function warmed(fetchers: number, preparers: number): DownloadState {
  const state = newDownload(pageCount, fetchers, preparers);
  for (let step = 0; step < 140; step++) tick(state, 16);
  return state;
}

export default function DownloadPipeline() {
  const [fetchers, setFetchers] = useState(3);
  const [preparers, setPreparers] = useState(2);
  const download = useRef<DownloadState | null>(null);
  if (download.current === null) download.current = warmed(3, 2);
  const [, setFrame] = useState(0);
  const [playing, setPlaying] = useState(false);

  useAnimationFrame((elapsed) => {
    const current = download.current!;
    tick(current, Math.min(elapsed, 50) * speed);
    if (current.bookJson) setPlaying(false);
    setFrame((count) => count + 1);
  }, playing);

  const restart = (nextFetchers: number, nextPreparers: number, play: boolean) => {
    download.current = newDownload(pageCount, nextFetchers, nextPreparers);
    setFrame((count) => count + 1);
    setPlaying(play);
  };

  const state = download.current!;
  const rowHeight = 34;
  const lanes = Math.max(state.fetchers.length, state.preparers.length, 3);
  const height = 40 + lanes * rowHeight + 10;
  const left = state.pages - state.nextPage + 1;

  return (
    <div>
      <Controls>
        <Slider
          label="Fetchers"
          value={fetchers}
          min={1}
          max={6}
          onChange={(value) => {
            setFetchers(value);
            restart(value, preparers, false);
          }}
        />
        <Slider
          label="Preparers"
          value={preparers}
          min={1}
          max={8}
          onChange={(value) => {
            setPreparers(value);
            restart(fetchers, value, false);
          }}
        />
      </Controls>
      <Controls>
        {playing ? (
          <Button primary onClick={() => setPlaying(false)}>
            ❚❚ Pause
          </Button>
        ) : (
          <Button
            primary
            onClick={() => (state.bookJson ? restart(fetchers, preparers, true) : setPlaying(true))}>
            {state.bookJson ? '↺ Download again' : '▶ Download'}
          </Button>
        )}
        <Button onClick={() => restart(fetchers, preparers, false)}>Reset</Button>
        <span className={styles.mono}>
          {state.bookJson
            ? `Downloaded in ${(state.elapsed / 1000).toFixed(1)} s (toy time)`
            : `Downloading ${state.done} of ${state.pages}`}
        </span>
      </Controls>
      <svg viewBox={`0 0 640 ${height}`} className={styles.svg} style={{maxWidth: 720}} role="img" aria-label="Server, fetchers, queue and preparers">
        {[
          {x: 8, label: 'Server'},
          {x: 112, label: 'Fetchers'},
          {x: 292, label: 'Queue'},
          {x: 372, label: 'Preparers'},
        ].map((column) => (
          <text key={column.label} x={column.x} y={18} fontSize={13} fontWeight={700} fill="var(--ink-text)">
            {column.label}
          </text>
        ))}
        {/* The server, with the page numbers no fetcher has taken yet. */}
        <rect x={8} y={30} width={84} height={lanes * rowHeight} rx={8} fill="var(--ink-blue-soft)" stroke="var(--ink-blue)" />
        <text x={50} y={54} textAnchor="middle" fontSize={12} fill="var(--ink-text)">
          {left > 0 ? `${left} pages` : 'all sent'}
        </text>
        <text x={50} y={70} textAnchor="middle" fontSize={11} fill="var(--ink-muted)">
          {left > 0 ? `next: ${state.nextPage}` : ''}
        </text>
        {state.fetchers.map((fetcher, index) => {
          const y = 30 + index * rowHeight;
          return (
            <g key={`f${index}`}>
              <rect x={112} y={y} width={160} height={rowHeight - 6} rx={6} fill="var(--ink-paper-2)" stroke={fetcher.blocked ? 'var(--ink-warn)' : 'var(--ink-line)'} />
              <text x={120} y={y + 18} fontSize={12} fill="var(--ink-text)">
                {fetcher.page === null
                  ? fetcher.skipped
                    ? `skip ${fetcher.skipped}, on disk`
                    : 'idle'
                  : fetcher.blocked
                    ? `p${fetcher.page}: queue full`
                    : `p${fetcher.page}`}
              </text>
              {fetcher.page !== null && !fetcher.blocked && (
                <>
                  <rect x={180} y={y + 10} width={84} height={8} rx={4} fill="var(--ink-line)" />
                  <rect x={180} y={y + 10} width={84 * Math.min(1, fetcher.progress)} height={8} rx={4} fill="var(--ink-blue)" />
                </>
              )}
            </g>
          );
        })}
        {/* The queue of fetched pages. */}
        {Array.from({length: fetchedQueue}, (_, slot) => {
          const page = state.queue[slot];
          const y = 30 + slot * 40;
          return (
            <g key={`q${slot}`}>
              <rect x={292} y={y} width={56} height={32} rx={6} fill={page ? 'var(--ink-blue-soft)' : 'none'} stroke="var(--ink-text)" strokeDasharray="4 3" />
              <text x={320} y={y + 21} textAnchor="middle" fontSize={12} fill="var(--ink-text)">
                {page ? `p${page}` : ''}
              </text>
            </g>
          );
        })}
        {state.preparers.map((preparer, index) => {
          const y = 30 + index * rowHeight;
          return (
            <g key={`p${index}`}>
              <rect x={372} y={y} width={260} height={rowHeight - 6} rx={6} fill="var(--ink-paper-2)" stroke="var(--ink-line)" />
              <text x={380} y={y + 18} fontSize={12} fill="var(--ink-text)">
                {preparer.page === null ? 'idle' : `p${preparer.page}`}
              </text>
              {prepareStages.map((stage, stageIndex) => {
                const active = preparer.page !== null && stageIndex === preparer.stage;
                const past = preparer.page !== null && stageIndex < preparer.stage;
                return (
                  <g key={stage}>
                    <rect
                      x={414 + stageIndex * 43}
                      y={y + 5}
                      width={40}
                      height={18}
                      rx={4}
                      fill={active ? 'var(--ink-accent-soft)' : past ? 'var(--ink-ok-soft)' : 'none'}
                      stroke={active ? 'var(--ink-accent)' : 'var(--ink-line)'}
                    />
                    <text x={434 + stageIndex * 43} y={y + 18} textAnchor="middle" fontSize={10} fill="var(--ink-text)">
                      {stage}
                    </text>
                  </g>
                );
              })}
            </g>
          );
        })}
      </svg>
      <BookFolder files={state.files} pages={state.pages} bookJson={state.bookJson} />
      <div className={styles.bar} style={{marginTop: '0.5rem'}}>
        <div
          className={clsx(styles.barFill, state.bookJson && styles.barFillOk)}
          style={{width: `${(state.done / state.pages) * 100}%`}}
        />
      </div>
      <div className={styles.muted} style={{fontSize: '0.8rem', marginTop: '0.2rem'}}>
        The bar is the progress the app shows: the pages 1 to n all on disk.
      </div>
    </div>
  );
}
