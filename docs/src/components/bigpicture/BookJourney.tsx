// A book's journey: a book token travels from the server to the reader's panel and its progress
// goes back, one station per step. Comics and EPUBs take different roads at three stations;
// the narration names the real functions and constants (internal/shelf, app/openbook,
// device/pocketbook, progress).

import React, {useState, type ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import {Controls, Narration, Segmented, StepperBar, usePrefersReducedMotion, useStepper} from '@site/src/components/kit';
import styles from './BookJourney.module.css';

type Kind = 'comic' | 'epub';

type Road = {
  text: ReactNode;
  visual?: ReactNode;
  links: readonly {to: string; label: string}[];
};

type Station = {
  icon: string;
  label: string;
  comic: Road;
  epub?: Road; // when the EPUB takes another road than the comic
};

function Files({names}: {names: readonly string[]}) {
  return (
    <div className={styles.files}>
      {names.map((name, index) => (
        <div key={name} className={clsx(styles.file, index === names.length - 1 && styles.fileLast)}>
          {name}
        </div>
      ))}
    </div>
  );
}

function Chain({steps}: {steps: readonly string[]}) {
  return (
    <div className={styles.chain}>
      {steps.map((step, index) => (
        <React.Fragment key={step}>
          {index > 0 && <span className={styles.chainArrow}>→</span>}
          <span className={styles.chainStep}>{step}</span>
        </React.Fragment>
      ))}
    </div>
  );
}

const stations: readonly Station[] = [
  {
    icon: '🗄️',
    label: 'Server',
    comic: {
      text: (
        <>
          The book starts on a server: here a comic on Komga, sent page by page as images. The app does not
          know Komga itself: it only talks to the <code>servers.Server</code> interface (<code>Books</code>,{' '}
          <code>OpenBook</code>, <code>OpenFile</code>, <code>SetProgress</code>…).
        </>
      ),
      links: [{to: '/big-picture/layers', label: 'The layer cake'}],
    },
    epub: {
      text: (
        <>
          The book starts on a server: here an EPUB on Calibre-Web, a single file. The app does not know
          Calibre-Web itself: it only talks to the <code>servers.Server</code> interface (<code>Books</code>,{' '}
          <code>OpenFile</code>, <code>SetProgress</code>…).
        </>
      ),
      links: [{to: '/big-picture/layers', label: 'The layer cake'}],
    },
  },
  {
    icon: '👆',
    label: 'Tap',
    comic: {
      text: (
        <>
          You tap the book in its list. The event loop reads the touch from the firmware (the kernel wakes it as it arrives),
          finds the target the screen registered there as it drew, and runs it: the book joins the download queue
          (package <code>downloadqueue</code>), which downloads on a goroutine of its own.
        </>
      ),
      links: [
        {to: '/screen/event-loop', label: 'The event loop'},
        {to: '/app/tap', label: 'From a tap to a screen'},
      ],
    },
  },
  {
    icon: '⬇️',
    label: 'Fetch',
    comic: {
      text: (
        <>
          <code>shelf.Store.Download</code> asks for the pages three at a time (<code>fetchWorkers = 3</code>: more
          only slows Komga down). Fetched pages wait in a small queue of two (<code>fetchedQueue</code>) for a
          preparer.
        </>
      ),
      links: [{to: '/app/downloads', label: 'Downloads'}],
    },
    epub: {
      text: (
        <>
          <code>shelf.Store.Download</code> fetches the EPUB whole into <code>book.epub.part</code>, renamed to{' '}
          <code>book.epub</code> once complete. A watchdog gives up when nothing arrives for 30 seconds: a dropped
          Wi-Fi would otherwise hang forever.
        </>
      ),
      links: [{to: '/app/downloads', label: 'Downloads'}],
    },
  },
  {
    icon: '⚙️',
    label: 'Prepare',
    comic: {
      text: (
        <>
          Each page is prepared now, once, so that turning pages later is fast. <code>pageimage.Prepare</code>{' '}
          decodes it to gray, fits it to the screen, tunes its tones for e-ink, dithers it to the panel’s 16 grays
          and packs it into a <code>.pg4</code> file, two pixels a byte. Preparers run one per core (2 to 8).
        </>
      ),
      visual: <Chain steps={['decode', 'fit', 'tone', 'dither', '.pg4']} />,
      links: [{to: '/screen/grays', label: '16 grays and dithering'}],
    },
    epub: {
      text: (
        <>
          The EPUB stays an EPUB: its text is laid out only when read, at the font size you choose. Now it is read
          through once: <code>epub.BuildIndex</code> counts each chapter’s characters into <code>index.json</code>,
          and the cover becomes <code>cover.pg4</code>. Its page count is a guess, 1500 characters a page (
          <code>CharsPerPage</code>).
        </>
      ),
      visual: <Chain steps={['open the zip', 'index chapters', 'cover.pg4']} />,
      links: [{to: '/books/epub', label: 'What is inside an EPUB'}],
    },
  },
  {
    icon: '📚',
    label: 'Shelf',
    comic: {
      text: (
        <>
          Everything lands in the book’s folder, <code>books/&lt;id&gt;/</code>. Each file is written under a
          temporary name, then renamed. <code>book.json</code> comes last: a folder without it is an interrupted
          download, which the next try resumes, skipping the pages already there.
        </>
      ),
      visual: <Files names={['0001.pg4', '0002.pg4', '…', '0224.pg4', 'book.json']} />,
      links: [{to: '/app/downloads', label: 'Downloads'}],
    },
    epub: {
      text: (
        <>
          Everything lands in the book’s folder, <code>books/&lt;id&gt;/</code>. Each file is written under a
          temporary name, then renamed. <code>book.json</code> comes last: a folder without it is an interrupted
          download.
        </>
      ),
      visual: <Files names={['book.epub', 'index.json', 'cover.pg4', 'book.json']} />,
      links: [{to: '/app/downloads', label: 'Downloads'}],
    },
  },
  {
    icon: '📖',
    label: 'Open',
    comic: {
      text: (
        <>
          You tap the downloaded book. <code>openbook.OpenLocked</code> opens it where it was left. If you read
          further on another device, the server’s progress wins, unless the reader has progress the server has not
          seen yet: <code>progress.Merge</code> decides.
        </>
      ),
      links: [
        {to: '/app/tap', label: 'From a tap to a screen'},
        {to: '/app/sync', label: 'Syncing progress'},
      ],
    },
  },
  {
    icon: '🧩',
    label: 'Page',
    comic: {
      text: (
        <>
          <code>Store.LoadPage</code> reads <code>0004.pg4</code> and <code>pageimage.Unpack</code> turns each
          4-bit number back into a gray with a table: no decoding, no scaling. That is the reward for the work done
          at download time.
        </>
      ),
      visual: <Chain steps={['0004.pg4', 'Unpack', 'gray page']} />,
      links: [{to: '/screen/grays', label: '16 grays and dithering'}],
    },
    epub: {
      text: (
        <>
          The chapter’s XHTML and CSS become a document of styled blocks (<code>Book.Chapter</code>), then{' '}
          <code>layout.Paginate</code> breaks it into lines and pages for the current font size, and the page is
          drawn in gray. The engine in package <code>reading</code> keeps the chapters around the one you read.
        </>
      ),
      visual: <Chain steps={['XHTML + CSS', 'document', 'lines', 'pages', 'gray page']} />,
      links: [
        {to: '/books/css', label: 'CSS'},
        {to: '/books/lines', label: 'Lines'},
        {to: '/books/pages', label: 'Pages'},
      ],
    },
  },
  {
    icon: '🖼️',
    label: 'Canvas',
    comic: {
      text: (
        <>
          The reader screen draws the page into the canvas: a gray image of the whole screen, which the firmware
          lends the app in shared memory. To show it, <code>copyRotated</code> copies the area that changed into
          the framebuffer <code>/dev/fb0</code>, turned a quarter, as the panel is landscape inside: about 22 ms a
          page turn.
        </>
      ),
      links: [{to: '/screen/no-sdk', label: 'Drawing without the SDK'}],
    },
  },
  {
    icon: '⚡',
    label: 'Refresh',
    comic: {
      text: (
        <>
          An <code>ioctl</code> asks the e-ink driver to refresh that area with the GL16 waveform: no flash, only
          the pixels that changed move. Every fifth page turn (<code>fullEvery</code>) is a full GC16 refresh
          instead: the panel flashes, and the ghosts of earlier pages are gone.
        </>
      ),
      links: [
        {to: '/screen/eink', label: 'How e-ink works'},
        {to: '/screen/firmware', label: 'Talking to the firmware'},
      ],
    },
    epub: {
      text: (
        <>
          An <code>ioctl</code> asks the e-ink driver to refresh that area with the GL16 waveform: no flash, only
          the pixels that changed move. For text, every sixth page turn (<code>textFullEvery</code>) is a full GC16
          refresh instead: the panel flashes, and the ghosts are gone.
        </>
      ),
      links: [
        {to: '/screen/eink', label: 'How e-ink works'},
        {to: '/screen/firmware', label: 'Talking to the firmware'},
      ],
    },
  },
  {
    icon: '🔄',
    label: 'Sync back',
    comic: {
      text: (
        <>
          Each page turn saves the progress on the reader and asks the <code>progress.Syncer</code> to send it in 3
          seconds (<code>SyncDelay</code>). Turning again restarts the wait, so ten quick turns send one request:{' '}
          <code>SetProgress</code> on the server. Offline, it tries again on the next page turn or start.
        </>
      ),
      links: [
        {to: '/app/sync', label: 'Syncing progress'},
        {to: '/app/locks', label: 'One lock, many goroutines'},
      ],
    },
  },
];

// The stations where the two kinds of book take different roads.
const forkStations = new Set([2, 3, 6]);

// Where the stations stand: a snake of two rows of five, left to right, then right to left.
const mapWidth = 640;
const mapHeight = 240;
function stationAt(index: number): {x: number; y: number} {
  const column = index % 5;
  const row = Math.floor(index / 5);
  const x = 64 + (row === 0 ? column : 4 - column) * 128;
  return {x, y: row === 0 ? 70 : 182};
}

const trackPath = (() => {
  const points = stations.map((_, index) => stationAt(index));
  let path = `M ${points[0].x} ${points[0].y}`;
  for (let index = 1; index < points.length; index++) {
    const previous = points[index - 1];
    const point = points[index];
    if (point.y !== previous.y) {
      path += ` C ${previous.x + 70} ${previous.y}, ${point.x + 70} ${point.y}, ${point.x} ${point.y}`;
    } else {
      path += ` L ${point.x} ${point.y}`;
    }
  }
  return path;
})();

export default function BookJourney() {
  const [kind, setKind] = useState<Kind>('comic');
  const stepper = useStepper(stations.length, 2600);
  const reducedMotion = usePrefersReducedMotion();
  const station = stations[stepper.index];
  const road = (kind === 'epub' && station.epub) || station.comic;
  const at = stationAt(stepper.index);

  return (
    <div className={styles.widget}>
      <Controls>
        <Segmented
          options={[
            {value: 'comic', label: '🗯️ A comic (Komga)'},
            {value: 'epub', label: '📘 An EPUB (Calibre-Web)'},
          ]}
          value={kind}
          onChange={setKind}
        />
      </Controls>

      <div className={styles.mapScroll}>
        <svg viewBox={`0 0 ${mapWidth} ${mapHeight}`} className={styles.map} role="img" aria-label="The stations of a book's journey">
          <path d={trackPath} className={styles.track} />
          {stations.map((item, index) => {
            const point = stationAt(index);
            const forks = forkStations.has(index);
            return (
              <g
                key={item.label}
                className={clsx(
                  styles.station,
                  index === stepper.index && styles.stationCurrent,
                  index < stepper.index && styles.stationPassed,
                )}
                onClick={() => stepper.seek(index)}>
                <circle cx={point.x} cy={point.y} r={22} />
                <text x={point.x} y={point.y + 1} textAnchor="middle" dominantBaseline="central" className={styles.icon}>
                  {item.icon}
                </text>
                <text x={point.x} y={point.y + 38} textAnchor="middle" className={styles.label}>
                  {index + 1}. {item.label}
                </text>
                {forks && (
                  <text x={point.x + 20} y={point.y - 18} className={styles.fork}>
                    ⑂
                  </text>
                )}
              </g>
            );
          })}
          <g
            className={clsx(styles.token, !reducedMotion && styles.tokenMoving)}
            style={{transform: `translate(${at.x - 22}px, ${at.y - 64}px)`}}>
            <rect width={44} height={22} rx={3} className={kind === 'comic' ? styles.tokenComic : styles.tokenEpub} />
            <text x={22} y={12} textAnchor="middle" dominantBaseline="central" className={styles.tokenText}>
              {kind === 'comic' ? 'pages' : 'EPUB'}
            </text>
            <path d="M 18 22 L 22 27 L 26 22 z" className={kind === 'comic' ? styles.tokenComic : styles.tokenEpub} />
          </g>
        </svg>
      </div>

      <StepperBar stepper={stepper} label="station" />
      <Narration>
        <strong>
          {station.icon} {station.label}.
        </strong>{' '}
        {road.text}
      </Narration>
      {road.visual}
      <div className={styles.links}>
        {road.links.map((link) => (
          <Link key={link.to + link.label} to={link.to} className={styles.link}>
            {link.label} →
          </Link>
        ))}
      </div>
    </div>
  );
}
