// The three layers between the app's drawing and the glass: the canvas (drawn on), the
// framebuffer (the canvas turned a quarter, written by copyRotated) and the panel (changed only
// by the MXCFB_SEND_UPDATE ioctl). Follows Refresh, FlashRefresh (eink_refresh.go),
// readerFirmware.flush and framebufferRect (framebuffer_copy.go), on a tiny 6x8 screen.

import React, {useEffect, useRef, useState} from 'react';
import {Button, Controls, Narration, PixelGrid, usePrefersReducedMotion} from '@site/src/components/kit';
import styles from './device.module.css';

// The tiny screen: 6 wide, 8 high, portrait. Its framebuffer is 8 wide, 6 high.
const WIDTH = 6;
const HEIGHT = 8;
const FB_WIDTH = HEIGHT;
const FB_HEIGHT = WIDTH;

const WHITE = 255;
const GRAY = 150;
const BLACK = 20;

type Rect = {minX: number; minY: number; maxX: number; maxY: number};

// A small arrow pointing up, drawn on the canvas but not refreshed yet.
const initialCanvas = (() => {
  const cells = new Array<number>(WIDTH * HEIGHT).fill(WHITE);
  const arrow = [
    [2, 1], [3, 1],
    [1, 2], [2, 2], [3, 2], [4, 2],
    [0, 3], [2, 3], [3, 3], [5, 3],
    [2, 4], [3, 4], [2, 5], [3, 5], [2, 6], [3, 6],
  ];
  for (const [x, y] of arrow) cells[y * WIDTH + x] = BLACK;
  return cells;
})();

function rectOfCells(cells: readonly number[], others: readonly number[]): Rect | null {
  let rect: Rect | null = null;
  cells.forEach((gray, index) => {
    if (gray === others[index]) return;
    const x = index % WIDTH;
    const y = Math.floor(index / WIDTH);
    rect = rect
      ? {minX: Math.min(rect.minX, x), minY: Math.min(rect.minY, y), maxX: Math.max(rect.maxX, x + 1), maxY: Math.max(rect.maxY, y + 1)}
      : {minX: x, minY: y, maxX: x + 1, maxY: y + 1};
  });
  return rect;
}

/** framebufferRect of framebuffer_copy.go: Rect(min.Y, H-max.X, max.Y, H-min.X). */
function framebufferRect(rect: Rect): Rect {
  return {minX: rect.minY, minY: FB_HEIGHT - rect.maxX, maxX: rect.maxY, maxY: FB_HEIGHT - rect.minX};
}

const fmt = (rect: Rect) => `(${rect.minX},${rect.minY})-(${rect.maxX},${rect.maxY})`;

type Phase =
  | {kind: 'idle'; text: string}
  | {kind: 'copy'; rect: Rect; column: number; full: boolean}
  | {kind: 'send'; rect: Rect; full: boolean; flash: 'black' | 'white' | 'done'};

export default function ThreeLayers() {
  const reduced = usePrefersReducedMotion();
  const [canvas, setCanvas] = useState<number[]>(initialCanvas);
  const [framebuffer, setFramebuffer] = useState<number[]>(() => new Array(FB_WIDTH * FB_HEIGHT).fill(WHITE));
  const [panel, setPanel] = useState<number[]>(() => new Array(WIDTH * HEIGHT).fill(WHITE));
  const [phase, setPhase] = useState<Phase>({
    kind: 'idle',
    text: 'The app drew an arrow on the canvas. The panel shows nothing yet: nobody asked for a refresh.',
  });
  const canvasRef = useRef(canvas);
  canvasRef.current = canvas;
  const framebufferRef = useRef(framebuffer);
  framebufferRef.current = framebuffer;

  // The animation: copy one canvas column per beat, then the ioctl, then (for a flash) black, white, image.
  useEffect(() => {
    if (phase.kind === 'idle') return undefined;
    const beat = reduced ? 0 : phase.kind === 'copy' ? 260 : 380;
    const timer = window.setTimeout(() => {
      if (phase.kind === 'copy') {
        const x = phase.rect.minX + phase.column;
        // Canvas column x becomes framebuffer row FB_HEIGHT-1-x, read down, written along.
        setFramebuffer((current) => {
          const next = [...current];
          for (let y = phase.rect.minY; y < phase.rect.maxY; y++) {
            next[(FB_HEIGHT - 1 - x) * FB_WIDTH + y] = canvasRef.current[y * WIDTH + x];
          }
          return next;
        });
        if (x + 1 < phase.rect.maxX) {
          setPhase({...phase, column: phase.column + 1});
        } else {
          setPhase({kind: 'send', rect: phase.rect, full: phase.full, flash: phase.full ? 'black' : 'done'});
        }
        return;
      }
      if (phase.flash === 'black') {
        setPhase({...phase, flash: 'white'});
        return;
      }
      if (phase.flash === 'white') {
        setPhase({...phase, flash: 'done'});
        return;
      }
      // The panel takes the framebuffer's pixels, turned back upright for your eyes.
      const current = framebufferRef.current;
      setPanel((before) =>
        before.map((gray, index) => {
          const x = index % WIDTH;
          const y = Math.floor(index / WIDTH);
          const inside = x >= phase.rect.minX && x < phase.rect.maxX && y >= phase.rect.minY && y < phase.rect.maxY;
          return inside ? current[(FB_HEIGHT - 1 - x) * FB_WIDTH + y] : gray;
        }),
      );
      setPhase({
        kind: 'idle',
        text: phase.full
          ? 'Done: a full GC16 refresh flashed the area black, then white, then drew it. No ghost is left.'
          : 'Done: a partial refresh drove only the pixels that changed. Draw again, and the panel waits for the next Refresh.',
      });
    }, beat);
    return () => window.clearTimeout(timer);
  }, [phase, reduced]);

  const busy = phase.kind !== 'idle';
  const dirty = rectOfCells(canvas, panel);

  const paint = (index: number) => {
    if (busy) return;
    setCanvas((current) => {
      const next = [...current];
      next[index] = current[index] === WHITE ? BLACK : current[index] === BLACK ? GRAY : WHITE;
      return next;
    });
    setPhase({kind: 'idle', text: 'You drew on the canvas. The framebuffer and the panel have not changed.'});
  };

  const refresh = (full: boolean) => {
    const rect: Rect = full ? {minX: 0, minY: 0, maxX: WIDTH, maxY: HEIGHT} : dirty ?? {minX: 0, minY: 0, maxX: 0, maxY: 0};
    if (rect.maxX === rect.minX) {
      setPhase({kind: 'idle', text: 'Nothing changed since the last refresh: an empty rectangle, so flush returns at once.'});
      return;
    }
    setPhase({kind: 'copy', rect, column: 0, full});
  };

  const copyHighlight = new Set<number>();
  const canvasHighlight = new Set<number>();
  let narration: React.ReactNode = phase.kind === 'idle' ? phase.text : '';
  if (phase.kind === 'copy') {
    const x = phase.rect.minX + phase.column;
    for (let y = phase.rect.minY; y < phase.rect.maxY; y++) {
      canvasHighlight.add(y * WIDTH + x);
      copyHighlight.add((FB_HEIGHT - 1 - x) * FB_WIDTH + y);
    }
    narration = (
      <>
        copyRotated: canvas <strong>column {x}</strong>, read top to bottom, becomes framebuffer{' '}
        <strong>row {FB_HEIGHT - 1 - x}</strong>, written left to right (row = {FB_HEIGHT - 1} − x).
      </>
    );
  } else if (phase.kind === 'send') {
    const fbRect = framebufferRect(phase.rect);
    narration =
      phase.flash === 'done' ? (
        <>
          ioctl MXCFB_SEND_UPDATE on framebuffer rect {fmt(fbRect)}, waveform {phase.full ? 'GC16, full' : 'GL16, partial'}.
          The panel’s controller drives the pixels.
        </>
      ) : (
        <>Full refresh: the panel flashes {phase.flash}…</>
      );
  }
  const shownPanel =
    phase.kind === 'send' && phase.flash !== 'done'
      ? panel.map(() => (phase.flash === 'black' ? BLACK : WHITE))
      : panel;

  return (
    <div className={styles.stack}>
      <div className={styles.row} style={{alignItems: 'flex-end'}}>
        <div>
          <div className={styles.label}>1 · Canvas (click to draw)</div>
          <PixelGrid
            columns={WIDTH}
            cells={canvas}
            cellSize={22}
            highlight={canvasHighlight}
            onCellClick={paint}
            ariaLabel="The canvas, 6 by 8"
          />
        </div>
        <div className={styles.arrow}>→ copyRotated →</div>
        <div>
          <div className={styles.label}>2 · Framebuffer (/dev/fb0)</div>
          <PixelGrid
            columns={FB_WIDTH}
            cells={framebuffer}
            cellSize={22}
            highlight={copyHighlight}
            ariaLabel="The framebuffer, 8 by 6"
          />
        </div>
        <div className={styles.arrow}>→ ioctl →</div>
        <div>
          <div className={styles.label}>3 · Panel (what you see)</div>
          <PixelGrid columns={WIDTH} cells={shownPanel} cellSize={22} ariaLabel="The panel, as you hold the reader" />
        </div>
      </div>
      <Controls>
        <Button primary onClick={() => refresh(false)} disabled={busy}>
          Refresh(rect, GL16)
        </Button>
        <Button onClick={() => refresh(true)} disabled={busy}>
          FlashRefresh()
        </Button>
        <Button
          onClick={() => {
            setCanvas(initialCanvas);
            setFramebuffer(new Array(FB_WIDTH * FB_HEIGHT).fill(WHITE));
            setPanel(new Array(WIDTH * HEIGHT).fill(WHITE));
            setPhase({kind: 'idle', text: 'Back to the start: an arrow on the canvas, a blank panel.'});
          }}
          disabled={busy}>
          Reset
        </Button>
        <span className={styles.small + ' ' + styles.muted}>
          Changed since last refresh: {dirty ? `rect ${fmt(dirty)}` : 'nothing'}
        </span>
      </Controls>
      <Narration>{narration}</Narration>
    </div>
  );
}
