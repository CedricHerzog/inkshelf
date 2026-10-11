// "Where do I look for…": a task → package map, searchable and filtered by area. It extends
// the table at the end of docs/reference/architecture.md; every path and symbol was checked in the code.

import React, {useMemo, useState} from 'react';
import Link from '@docusaurus/Link';
import {CodeRef, RefDoc, Segmented} from '@site/src/components/kit';
import styles from './WhereToLook.module.css';

type Area = 'screen' | 'books' | 'app' | 'servers' | 'tools';

type Entry = {
  task: string;
  area: Area;
  places: readonly {path: string; symbol?: string}[];
  chapter?: {to: string; label: string};
  doc?: string;
  keywords?: string;
};

const entries: readonly Entry[] = [
  {
    task: 'What happens on a page turn',
    area: 'app',
    places: [
      {path: 'internal/app/openbook/openbook.go', symbol: 'TurnPage'},
      {path: 'internal/app/screens/reader/reader.go', symbol: 'DrawAndRefreshLocked'},
    ],
    chapter: {to: '/app/tap', label: 'Life of a tap'},
    doc: 'app-threading.md',
    keywords: 'next page turn reader',
  },
  {
    task: 'A panel refresh, waveforms, ghosting',
    area: 'screen',
    places: [{path: 'internal/device/pocketbook/eink_refresh.go', symbol: 'Refresh, FlashRefresh'}],
    chapter: {to: '/screen/eink', label: 'How e-ink works'},
    doc: 'eink-and-firmware.md',
    keywords: 'gc16 gl16 du flash mxcfb ioctl',
  },
  {
    task: 'Copying the canvas to the framebuffer',
    area: 'screen',
    places: [
      {path: 'internal/device/pocketbook/framebuffer_copy.go', symbol: 'copyRotated'},
      {path: 'internal/device/pocketbook/framebuffer_linux_arm.go', symbol: 'flush'},
    ],
    chapter: {to: '/screen/no-sdk', label: 'Running Go on a PocketBook'},
    doc: 'eink-and-firmware.md',
    keywords: 'fb0 rotate hot path',
  },
  {
    task: 'Talking to the firmware: messages, foreground, shared memory',
    area: 'screen',
    places: [
      {path: 'internal/device/pocketbook/firmware_messages.go'},
      {path: 'internal/device/pocketbook/firmware_shared_memory.go'},
    ],
    chapter: {to: '/screen/firmware', label: 'Talking to the firmware'},
    doc: 'eink-and-firmware.md',
    keywords: 'ipc queue background',
  },
  {
    task: 'Touch, keys and the event loop',
    area: 'screen',
    places: [
      {path: 'internal/device/pocketbook/event_loop.go'},
      {path: 'internal/device/pocketbook/touch.go'},
      {path: 'internal/device/pocketbook/keys.go'},
    ],
    chapter: {to: '/screen/event-loop', label: 'The event loop and touch'},
    doc: 'eink-and-firmware.md',
    keywords: 'input poll swipe button',
  },
  {
    task: 'Wi-Fi',
    area: 'screen',
    places: [{path: 'internal/device/pocketbook/wifi_linux_arm.go'}],
    doc: 'eink-and-firmware.md',
    keywords: 'network',
  },
  {
    task: 'The emulator, its scripts and screenshots',
    area: 'tools',
    places: [
      {path: 'cmd/emulator/script.go'},
      {path: 'internal/device/pocketbook/emulated_firmware.go', symbol: 'NewEmulator'},
      {path: 'emulator/scripts'},
    ],
    chapter: {to: '/screen/emulator', label: 'The emulator'},
    doc: 'emulator.md',
    keywords: 'shots baseline fake',
  },
  {
    task: 'A download',
    area: 'app',
    places: [
      {path: 'internal/app/downloadqueue/downloadqueue.go', symbol: 'EnqueueLocked'},
      {path: 'internal/shelf/download.go', symbol: 'Store.Download'},
    ],
    chapter: {to: '/app/downloads', label: 'Downloads and the shelf'},
    doc: 'image-pipeline.md',
    keywords: 'queue fetch workers',
  },
  {
    task: 'How a page image is prepared',
    area: 'screen',
    places: [
      {path: 'internal/graphics/pageimage/prepare.go', symbol: 'Prepare'},
      {path: 'internal/graphics/pageimage/dither.go', symbol: 'ditherAndPack'},
      {path: 'internal/graphics/pageimage/pg4.go', symbol: 'Unpack'},
    ],
    chapter: {to: '/screen/grays', label: 'From a scan to 16 grays'},
    doc: 'image-pipeline.md',
    keywords: 'decode fit tone dither pg4 comic',
  },
  {
    task: 'Opening an EPUB: zip, package, spine, navigation',
    area: 'books',
    places: [
      {path: 'internal/ebook/epub/container/package_document.go'},
      {path: 'internal/ebook/epub/container/navigation.go'},
    ],
    chapter: {to: '/books/epub', label: 'What is an EPUB?'},
    doc: 'epub-to-page.md',
    keywords: 'opf manifest nav toc drm',
  },
  {
    task: 'What a chapter becomes',
    area: 'books',
    places: [
      {path: 'internal/ebook/epub/chapter.go', symbol: 'Book.Chapter'},
      {path: 'internal/typesetting/document'},
    ],
    chapter: {to: '/books/epub', label: 'What is an EPUB?'},
    doc: 'document-model.md',
    keywords: 'xhtml blocks runs document model',
  },
  {
    task: 'A CSS property in a book',
    area: 'books',
    places: [
      {path: 'internal/ebook/epub/style.go', symbol: 'computeStyle'},
      {path: 'internal/web/css/cascade.go'},
    ],
    chapter: {to: '/books/css', label: 'From XHTML and CSS to a document'},
    doc: 'epub-to-page.md',
    keywords: 'stylesheet cascade selector specificity',
  },
  {
    task: 'Drawing text, glyphs and fonts',
    area: 'books',
    places: [
      {path: 'internal/graphics/fonts/draw.go', symbol: 'DrawSpaced, blend'},
      {path: 'internal/graphics/fonts/open.go', symbol: 'Open'},
    ],
    chapter: {to: '/books/fonts', label: 'Text in gray'},
    doc: 'epub-to-page.md',
    keywords: 'font glyph mask hinting antialias truetype',
  },
  {
    task: 'A character drawn as a box (font fallback)',
    area: 'books',
    places: [
      {path: 'internal/typesetting/layout/faces.go', symbol: 'forEachCoveredPart, fallbacksOf'},
      {path: 'internal/graphics/fonts/open.go', symbol: 'OpenFallbacks'},
    ],
    chapter: {to: '/books/fonts', label: 'Text in gray'},
    doc: 'epub-to-page.md',
    keywords: 'fallback notdef tofu missing character dejavu',
  },
  {
    task: 'Arabic, Hebrew, Indic text (shaping)',
    area: 'books',
    places: [
      {path: 'internal/graphics/fonts/shape.go', symbol: 'Font.Shape'},
      {path: 'internal/typesetting/document/scripts.go', symbol: 'NeedsShaping'},
    ],
    chapter: {to: '/books/fonts', label: 'Text in gray'},
    doc: 'epub-to-page.md',
    keywords: 'harfbuzz go-text ligature rtl',
  },
  {
    task: 'A book’s embedded fonts (WOFF, obfuscation)',
    area: 'books',
    places: [
      {path: 'internal/web/webfont'},
      {path: 'internal/ebook/epub/container/resources.go', symbol: 'Book.Font'},
    ],
    chapter: {to: '/books/fonts', label: 'Text in gray'},
    doc: 'epub-to-page.md',
    keywords: 'woff woff2 font-face deobfuscate',
  },
  {
    task: 'Line breaking and justification',
    area: 'books',
    places: [
      {path: 'internal/typesetting/layout/line_breaking.go'},
      {path: 'internal/typesetting/layout/line_placement.go'},
    ],
    chapter: {to: '/books/lines', label: 'Breaking lines'},
    doc: 'pagination.md',
    keywords: 'justify words spaces',
  },
  {
    task: 'Hyphenation',
    area: 'books',
    places: [{path: 'internal/typesetting/hyphen'}],
    chapter: {to: '/books/lines', label: 'Breaking lines'},
    doc: 'pagination.md',
    keywords: 'liang patterns',
  },
  {
    task: 'Pagination',
    area: 'books',
    places: [{path: 'internal/typesetting/layout/paginate.go', symbol: 'Paginate'}],
    chapter: {to: '/books/pages', label: 'Pages and positions'},
    doc: 'pagination.md',
    keywords: 'pages widows orphans',
  },
  {
    task: 'Right-to-left text (bidi)',
    area: 'books',
    places: [{path: 'internal/typesetting/layout/bidi.go'}],
    doc: 'pagination.md',
    keywords: 'arabic hebrew',
  },
  {
    task: 'Fonts, fallbacks, Arabic shaping',
    area: 'books',
    places: [
      {path: 'internal/graphics/fonts/shape.go'},
      {path: 'internal/graphics/fonts/draw.go'},
    ],
    keywords: 'truetype harfbuzz glyph text',
  },
  {
    task: 'Images, SVG and MathML in a book',
    area: 'books',
    places: [
      {path: 'internal/web/webimage'},
      {path: 'internal/web/svgimage'},
      {path: 'internal/web/mathml'},
    ],
    keywords: 'picture formula',
  },
  {
    task: 'Fixed-layout EPUBs and CBZ comics',
    area: 'books',
    places: [
      {path: 'internal/ebook/fixedlayout'},
      {path: 'internal/shelf/prepare_fixed_layout.go'},
      {path: 'internal/ebook/comicarchive'},
    ],
    doc: 'epub-conformance.md',
    keywords: 'prepaginated cbz zip',
  },
  {
    task: 'How much of EPUB 3.3 works',
    area: 'books',
    places: [{path: 'internal/ebook/epub'}],
    doc: 'epub-conformance.md',
    keywords: 'support spec',
  },
  {
    task: 'Reading an EPUB: positions, links, notes, contents',
    area: 'books',
    places: [
      {path: 'internal/reading/textbook.go', symbol: 'TextBook'},
      {path: 'internal/reading/links.go'},
      {path: 'internal/reading/contents.go'},
    ],
    keywords: 'footnote toc renderer',
  },
  {
    task: 'The Server interface',
    area: 'servers',
    places: [{path: 'internal/servers/server.go', symbol: 'Server'}],
    doc: 'servers.md',
    chapter: {to: '/app/servers', label: 'Three servers, one interface'},
    keywords: 'interface format',
  },
  {
    task: 'A server’s API (Komga, Mealie, Calibre-Web)',
    area: 'servers',
    places: [
      {path: 'internal/servers/komga'},
      {path: 'internal/servers/mealie'},
      {path: 'internal/servers/calibreweb'},
    ],
    doc: 'servers.md',
    chapter: {to: '/app/servers', label: 'Three servers, one interface'},
    keywords: 'http api recipes opds',
  },
  {
    task: 'Adding a kind of server',
    area: 'servers',
    places: [{path: 'internal/servers/registry/registry.go', symbol: 'KindOf, Opener'}],
    doc: 'servers.md',
    chapter: {to: '/app/servers', label: 'Three servers, one interface'},
    keywords: 'new server checklist',
  },
  {
    task: 'config.json',
    area: 'servers',
    places: [{path: 'internal/servers/config.go', symbol: 'LoadConfig'}],
    doc: 'on-disk-formats.md',
    chapter: {to: '/app/servers', label: 'Three servers, one interface'},
    keywords: 'configuration api key',
  },
  {
    task: 'Sorting titles ("Tome 2" before "Tome 10")',
    area: 'servers',
    places: [
      {path: 'internal/servers/sorting.go', symbol: 'CompareTitles'},
      {path: 'internal/naturalorder'},
    ],
    chapter: {to: '/app/servers', label: 'Three servers, one interface'},
    keywords: 'natural order sort',
  },
  {
    task: 'Progress sync',
    area: 'app',
    places: [
      {path: 'internal/progress/progress.go', symbol: 'Merge'},
      {path: 'internal/progress/syncer.go', symbol: 'Syncer'},
      {path: 'internal/app/openbook/openbook.go', symbol: 'SyncDelay'},
    ],
    chapter: {to: '/app/sync', label: 'Syncing reading progress'},
    doc: 'progress-sync.md',
    keywords: 'debounce server progress',
  },
  {
    task: 'A screen',
    area: 'app',
    places: [
      {path: 'internal/app/screens.go', symbol: 'Screen'},
      {path: 'internal/app/screens'},
    ],
    chapter: {to: '/app/tap', label: 'Life of a tap'},
    doc: 'app-threading.md',
    keywords: 'ui list reader contents',
  },
  {
    task: 'Taps and targets',
    area: 'app',
    places: [
      {path: 'internal/app/input.go', symbol: 'Pointer'},
      {path: 'internal/app/widgets/targets.go', symbol: 'Targets'},
    ],
    chapter: {to: '/app/tap', label: 'Life of a tap'},
    doc: 'app-threading.md',
    keywords: 'touch hit test button',
  },
  {
    task: 'Locks, goroutines, repaints',
    area: 'app',
    places: [{path: 'internal/app/state/state.go', symbol: 'Update, Go, GoLoop'}],
    chapter: {to: '/app/locks', label: 'Threads and locks'},
    doc: 'app-threading.md',
    keywords: 'mutex locked thread deadlock',
  },
  {
    task: 'Sizes in the UI (InkPad 3 pixels)',
    area: 'app',
    places: [{path: 'internal/app/widgets/scale.go', symbol: 'Px'}],
    keywords: 'pixels scale widget',
  },
  {
    task: 'Files on the reader',
    area: 'app',
    places: [
      {path: 'internal/shelf/store.go', symbol: 'Store'},
      {path: 'internal/app/state/preferences.go', symbol: 'filters.json'},
      {path: 'cmd/inkshelf'},
    ],
    doc: 'on-disk-formats.md',
    keywords: 'book.json disk folder pg4 index.json',
  },
  {
    task: 'Covers',
    area: 'app',
    places: [{path: 'internal/app/covers'}, {path: 'internal/shelf/covers.go'}],
    keywords: 'thumbnail',
  },
  {
    task: 'The log on the reader',
    area: 'tools',
    places: [{path: 'cmd/inkshelf/logging.go'}],
    keywords: 'inkshelf.log usb debug',
  },
  {
    task: 'Something is slow: timing, profiles, benchmarks',
    area: 'tools',
    places: [
      {path: 'internal/timing/span.go', symbol: 'Start'},
      {path: 'internal/timing/profile.go', symbol: 'Capture'},
      {path: 'cmd/inkshelf/profiling.go'},
    ],
    doc: 'profiling.md',
    keywords: 'performance speed pprof trace benchmark benchstat bottleneck',
  },
];

const areas = [
  {value: 'all', label: 'All'},
  {value: 'screen', label: 'Screen'},
  {value: 'books', label: 'Books'},
  {value: 'app', label: 'App'},
  {value: 'servers', label: 'Servers'},
  {value: 'tools', label: 'Tools'},
] as const;

type AreaFilter = (typeof areas)[number]['value'];

export default function WhereToLook() {
  const [query, setQuery] = useState('');
  const [area, setArea] = useState<AreaFilter>('all');

  const shown = useMemo(() => {
    const words = query.toLowerCase().split(/\s+/).filter(Boolean);
    return entries.filter((entry) => {
      if (area !== 'all' && entry.area !== area) return false;
      const haystack = [
        entry.task,
        entry.keywords ?? '',
        ...entry.places.map((place) => `${place.path} ${place.symbol ?? ''}`),
      ]
        .join(' ')
        .toLowerCase();
      return words.every((word) => haystack.includes(word));
    });
  }, [query, area]);

  return (
    <div className={styles.widget}>
      <input
        type="search"
        className={styles.search}
        placeholder="Search: refresh, css, sync, pg4…"
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        aria-label="Search the tasks"
      />
      <Segmented options={areas} value={area} onChange={setArea} />
      <ul className={styles.list}>
        {shown.map((entry) => (
          <li key={entry.task} className={styles.entry}>
            <div className={styles.task}>
              <span className={styles.area}>{entry.area}</span>
              {entry.task}
            </div>
            <div className={styles.places}>
              {entry.places.map((place) => (
                <CodeRef key={place.path + (place.symbol ?? '')} path={place.path} symbol={place.symbol} />
              ))}
            </div>
            {(entry.chapter || entry.doc) && (
              <div className={styles.more}>
                {entry.chapter && <Link to={entry.chapter.to}>{entry.chapter.label} →</Link>}
                {entry.doc && <RefDoc file={entry.doc} />}
              </div>
            )}
          </li>
        ))}
        {shown.length === 0 && <li className={styles.none}>Nothing matches. Try a shorter word.</li>}
      </ul>
    </div>
  );
}
