// A pretend e-ink panel that turns pages. Each refresh takes as long as on an InkPad 3 (the
// emulator's measured times), partial refreshes leave ghosts that pile up, a full refresh
// flashes and wipes them, and the fast one only knows black and white. With auto-flash on, the
// page-turn button follows Inkshelf's rhythm: every 5th turn of a comic, every 6th of a text.

import React, {useCallback, useEffect, useRef, useState} from 'react';
import {
  Button,
  Controls,
  Segmented,
  Slider,
  Toggle,
  useAnimationFrame,
  usePrefersReducedMotion,
} from '@site/src/components/kit';
import {
  frameDuring,
  fullEvery,
  fullRefreshDue,
  makeComicPage,
  makeTextPage,
  newToyPanel,
  refreshMs,
  refreshToy,
  textFullEvery,
  visible,
  type RefreshKind,
  type ToyPanel,
} from '@site/src/lib/einkPanel';
import styles from './eink.module.css';

const width = 144;
const height = 192; // 3:4, like the InkPad 3's 1404×1872

type BookKind = 'comic' | 'text';

type Running = {
  kind: RefreshKind;
  from: Uint8ClampedArray;
  to: Uint8ClampedArray;
  after: ToyPanel;
  durationMs: number;
  elapsedMs: number;
};

const labels: Record<RefreshKind, string> = {
  partial: 'GL16, partial',
  full: 'GC16, full (flash)',
  fast: 'DU, partial',
};

function pageOf(kind: BookKind, number: number): Uint8ClampedArray {
  return kind === 'comic' ? makeComicPage(number, width, height) : makeTextPage(number, width, height);
}

/** How much of what a pixel showed a partial refresh leaves behind: exaggerated, to be seen. */
function leakOf(kind: BookKind): number {
  return kind === 'comic' ? 0.15 : 0.1; // text ghosts less (openbook/text.go, textFullEvery)
}

export default function PanelSimulator() {
  const reduced = usePrefersReducedMotion();
  const canvas = useRef<HTMLCanvasElement>(null);
  const [bookKind, setBookKind] = useState<BookKind>('comic');
  const [page, setPage] = useState(1);
  const panel = useRef<ToyPanel>(newToyPanel(width, height, pageOf('comic', 1)));
  const [running, setRunning] = useState<Running | null>(null);
  const [turns, setTurns] = useState(0);
  const [sinceFlash, setSinceFlash] = useState(0);
  const [autoFlash, setAutoFlash] = useState(true);
  const [every, setEvery] = useState(fullEvery);
  const [last, setLast] = useState<string>('Not refreshed yet: the book just opened.');

  const draw = useCallback((pixels: Uint8ClampedArray) => {
    const context = canvas.current?.getContext('2d');
    if (!context) return;
    const data = context.createImageData(width, height);
    for (let index = 0; index < pixels.length; index++) {
      data.data[index * 4] = pixels[index];
      data.data[index * 4 + 1] = pixels[index];
      data.data[index * 4 + 2] = pixels[index];
      data.data[index * 4 + 3] = 255;
    }
    context.putImageData(data, 0, 0);
  }, []);

  useEffect(() => draw(visible(panel.current)), [draw]);

  const open = (kind: BookKind) => {
    setBookKind(kind);
    setEvery(kind === 'comic' ? fullEvery : textFullEvery);
    setPage(1);
    setTurns(0);
    setSinceFlash(0);
    setRunning(null);
    panel.current = newToyPanel(width, height, pageOf(kind, 1));
    draw(visible(panel.current));
    setLast('Opened: a full refresh shows the first page clean.');
  };

  const start = (kind: RefreshKind, turn: boolean) => {
    if (running) return;
    const nextPage = turn ? page + 1 : page;
    const before = visible(panel.current);
    const after = refreshToy(panel.current, pageOf(bookKind, nextPage), kind, leakOf(bookKind));
    const durationMs = kind === 'full' ? refreshMs.full : kind === 'fast' ? refreshMs.fast : refreshMs.partial;
    setRunning({kind, from: before, to: visible(after), after, durationMs, elapsedMs: 0});
    if (turn) {
      setPage(nextPage);
      setTurns((count) => count + 1);
    }
    setSinceFlash((count) => (kind === 'full' ? 0 : count + (turn ? 1 : 0)));
  };

  const turnPage = () => {
    const flash = autoFlash && fullRefreshDue(turns, every);
    start(flash ? 'full' : 'partial', true);
  };

  useAnimationFrame((elapsedMs) => {
    if (!running) return;
    const elapsed = Math.min(running.durationMs, running.elapsedMs + elapsedMs);
    if (elapsed >= running.durationMs) {
      panel.current = running.after;
      draw(running.to);
      setLast(`${labels[running.kind]}: ${running.durationMs} ms`);
      setRunning(null);
      return;
    }
    if (!reduced) draw(frameDuring(running.from, running.to, running.kind, elapsed / running.durationMs));
    setRunning({...running, elapsedMs: elapsed});
  }, running !== null);

  const nextIsFlash = autoFlash && fullRefreshDue(turns, every);
  const cycle = turns % every;

  return (
    <div>
      <Controls>
        <Segmented
          options={[
            {value: 'comic', label: 'Comic'},
            {value: 'text', label: 'Novel'},
          ]}
          value={bookKind}
          onChange={open}
        />
      </Controls>
      <div className={styles.stage}>
        <div className={styles.panelFrame}>
          <canvas
            ref={canvas}
            width={width}
            height={height}
            className={styles.panelCanvas}
            aria-label={`Simulated e-ink panel showing page ${page}`}
          />
        </div>
        <div className={styles.panelSide}>
          <div className={styles.buttons}>
            <Button primary onClick={turnPage} disabled={running !== null}>
              Turn page {nextIsFlash ? '· this one flashes (GC16)' : '· partial (GL16)'}
            </Button>
            <Button onClick={() => start('full', true)} disabled={running !== null}>
              Turn page · full (GC16)
            </Button>
            <Button onClick={() => start('fast', true)} disabled={running !== null}>
              Turn page · fast (DU)
            </Button>
            <Button onClick={() => start('full', false)} disabled={running !== null}>
              Flash this page (GC16)
            </Button>
          </div>
          <div>
            <div className={styles.meter} aria-hidden>
              <div
                className={styles.meterFill}
                style={{width: running ? `${(running.elapsedMs / running.durationMs) * 100}%` : '0%'}}
              />
            </div>
            <div className={styles.readout}>
              {running ? (
                <>
                  <strong>{labels[running.kind]}</strong>: {Math.round(running.elapsedMs)} / {running.durationMs} ms
                </>
              ) : (
                last
              )}
            </div>
          </div>
          <div className={styles.readout}>
            Page <strong>{page}</strong> · turns since the last flash: <strong>{sinceFlash}</strong>
          </div>
          <div className={styles.counter} aria-label={`Turn ${cycle + 1} of a cycle of ${every}`}>
            {Array.from({length: every}, (_, index) => (
              <span
                key={index}
                className={`${styles.dot} ${index < cycle ? styles.dotDone : ''} ${index === every - 1 ? styles.dotFlash : ''}`}
                title={index === every - 1 ? 'this turn flashes' : 'a partial turn'}
              />
            ))}
            <span className={styles.readout}>&nbsp;the dashed turn flashes</span>
          </div>
          <Toggle label="Auto-flash, like Inkshelf" checked={autoFlash} onChange={setAutoFlash} />
          <Slider label="every" value={every} min={2} max={10} onChange={setEvery} format={(value) => `${value} turns`} />
        </div>
      </div>
    </div>
  );
}
