// A book file explodes into the files of its zip. Each file can be opened, and arrows show
// which file points to which: container.xml to the package document, the package document to
// the chapters. A stepper follows the order in which container.Open reads them.

import React, {useEffect, useMemo, useState, type CSSProperties, type ReactNode} from 'react';
import clsx from 'clsx';
import {
  Button,
  Controls,
  Narration,
  StepperBar,
  usePrefersReducedMotion,
  useStepper,
} from '@site/src/components/kit';
import {sampleFiles, type SampleFile} from '@site/src/lib/epubSample';
import styles from './Epub.module.css';

const rowHeight = 24;
const indentWidth = 16;
const laneX = 172; // where the arrows leave and reach the rows

type Row = {label: string; depth: number; file?: SampleFile};

// The rows of the tree: folders, then their files, in the order of the zip.
function treeRows(): Row[] {
  const rows: Row[] = [];
  const opened = new Set<string>();
  for (const file of sampleFiles) {
    const parts = file.path.split('/');
    for (let depth = 0; depth < parts.length - 1; depth++) {
      const folder = parts.slice(0, depth + 1).join('/');
      if (!opened.has(folder)) {
        opened.add(folder);
        rows.push({label: `${parts[depth]}/`, depth});
      }
    }
    rows.push({label: parts[parts.length - 1], depth: parts.length - 1, file});
  }
  return rows;
}

// The strings each file's viewer marks: what Inkshelf reads in it.
const marks: Record<string, readonly string[]> = {
  'META-INF/container.xml': ['full-path="OEBPS/content.opf"'],
  'OEBPS/content.opf': ['properties="nav"', 'linear="no"', '<spine toc="ncx">', 'properties="cover-image"'],
  'OEBPS/nav.xhtml': ['epub:type="toc"'],
  'OEBPS/text/c1.xhtml': ['href="../css/style.css"'],
  'OEBPS/css/style.css': ['@media amzn-kf8'],
};

// The steps of container.Open, each with the file it reads.
const openSteps: readonly {path: string; text: ReactNode}[] = [
  {
    path: 'mimetype',
    text: (
      <>
        The zip opens. Inkshelf does not even read <code>mimetype</code>: it is there for other
        programs, which recognize an EPUB by its first bytes.
      </>
    ),
  },
  {
    path: 'META-INF/container.xml',
    text: (
      <>
        <code>checkEncryption</code> looks for <code>META-INF/encryption.xml</code>: there is
        none, so nothing is locked. Then <code>packageDocumentPath</code> reads{' '}
        <code>container.xml</code>, the one file whose name is fixed, and finds the package
        document's path.
      </>
    ),
  },
  {
    path: 'OEBPS/content.opf',
    text: (
      <>
        The package document is read: <code>readMetadata</code> (the title, “Le Livre”, its
        spaces trimmed), <code>readManifest</code> (every file, by id) and{' '}
        <code>readSpine</code> (the reading order).
      </>
    ),
  },
  {
    path: 'OEBPS/text/c1.xhtml',
    text: (
      <>
        The spine's linear items become the chapters: the cover page, <code>c1</code> and{' '}
        <code>c2</code>. That is 3 chapters, not 4: <code>notes</code> is{' '}
        <code>linear="no"</code>.
      </>
    ),
  },
  {
    path: 'OEBPS/nav.xhtml',
    text: (
      <>
        Later, <code>Navigation</code> reads the item with <code>properties="nav"</code> for
        the table of contents. The NCX is only a backup for books without one.
      </>
    ),
  },
];

function highlighted(content: string, wanted: readonly string[]): ReactNode[] {
  const pieces: ReactNode[] = [];
  let rest = content;
  let key = 0;
  while (rest.length > 0) {
    let best = -1;
    let found = '';
    for (const mark of wanted) {
      const at = rest.indexOf(mark);
      if (at >= 0 && (best < 0 || at < best)) {
        best = at;
        found = mark;
      }
    }
    if (best < 0) {
      pieces.push(rest);
      break;
    }
    pieces.push(rest.slice(0, best));
    pieces.push(
      <mark key={key++} className={styles.mark}>
        {found}
      </mark>,
    );
    rest = rest.slice(best + found.length);
  }
  return pieces;
}

function ZipBytes() {
  // The start of the zip: the local header of mimetype (30 bytes), its name, its content.
  const groups: {text: string; note: string; hot?: boolean}[] = [
    {text: 'PK 03 04', note: 'bytes 0–3: a zip entry'},
    {text: '… 26 bytes …', note: '4–29: sizes, dates'},
    {text: 'mimetype', note: '30–37: its name', hot: true},
    {text: 'application/epub+zip', note: '38–57: stored, not compressed', hot: true},
    {text: 'PK 03 04', note: '58: next entry'},
    {text: 'META-INF/container.xml', note: 'its name'},
    {text: 'xœ³±È…', note: 'compressed'},
  ];
  return (
    <div className={styles.bytes}>
      {groups.map((group, index) => (
        <span key={index} className={clsx(styles.byteGroup, group.hot && styles.byteHot)}>
          {group.text}
          <small>{group.note}</small>
        </span>
      ))}
    </div>
  );
}

export default function EpubUnzip() {
  const rows = useMemo(treeRows, []);
  const reduced = usePrefersReducedMotion();
  const [unzipped, setUnzipped] = useState(false);
  const [selected, setSelected] = useState('META-INF/container.xml');
  const stepper = useStepper(openSteps.length, 2600);

  useEffect(() => {
    if (unzipped) setSelected(openSteps[stepper.index].path);
  }, [stepper.index, unzipped]);

  const rowOf = (path: string) => rows.findIndex((row) => row.file?.path === path);
  const file = sampleFiles.find((candidate) => candidate.path === selected)!;
  const height = rows.length * rowHeight;

  const arrows = (() => {
    const from = rowOf(selected);
    const wires: {to: number; weak: boolean}[] = [
      ...(file.links ?? []).map((path) => ({to: rowOf(path), weak: false})),
      ...(file.weakLinks ?? []).map((path) => ({to: rowOf(path), weak: true})),
    ];
    return wires.map(({to, weak}) => {
      const y1 = from * rowHeight + rowHeight / 2;
      const y2 = to * rowHeight + rowHeight / 2;
      const bulge = laneX + 18 + Math.min(70, Math.abs(y2 - y1) * 0.25);
      return {
        key: `${from}-${to}`,
        d: `M ${laneX} ${y1} C ${bulge} ${y1}, ${bulge} ${y2}, ${laneX + 6} ${y2}`,
        weak,
        to,
      };
    });
  })();
  const targets = new Set(arrows.map((arrow) => arrow.to));

  return (
    <div>
      <div className={styles.unzip}>
        <div className={styles.tree} style={{height: unzipped ? height : 'auto'}}>
          {!unzipped && (
            <div className={styles.zippedNote}>
              <div className={styles.bookFile}>
                <div className={styles.bookIcon}>EPUB</div>
                <div>
                  <strong>book.epub</strong>
                  <div style={{fontSize: '0.75rem', color: 'var(--ink-muted)'}}>
                    one file, a zip in disguise
                  </div>
                </div>
              </div>
              <p style={{margin: '0.75rem 0 0'}}>Its first bytes, read as text:</p>
              <ZipBytes />
            </div>
          )}
          {unzipped &&
            rows.map((row, index) => {
              const path = row.file?.path;
              const style = {
                top: index * rowHeight,
                paddingLeft: 4 + row.depth * indentWidth,
                '--from-y': `${-index * rowHeight}px`,
                animationDelay: reduced ? undefined : `${index * 55}ms`,
              } as CSSProperties;
              return (
                <button
                  key={row.label + index}
                  type="button"
                  className={clsx(
                    styles.row,
                    !reduced && styles.rowFly,
                    !row.file && styles.rowFolder,
                    path === selected && styles.rowSelected,
                    targets.has(index) && styles.rowTarget,
                  )}
                  style={style}
                  disabled={!row.file}
                  onClick={() => path && setSelected(path)}>
                  <span aria-hidden>{row.file ? (row.file.binary ? '🖼' : '📄') : '📁'}</span>
                  {row.label}
                </button>
              );
            })}
          {unzipped && (
            <svg
              className={clsx(styles.arrows, !reduced && styles.arrowsFade)}
              width={262}
              height={height}
              style={{animationDelay: reduced ? undefined : `${rows.length * 55 + 300}ms`}}
              aria-hidden>
              <defs>
                <marker id="epub-unzip-head" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="7" markerHeight="7" orient="auto">
                  <path d="M0,0 L8,4 L0,8 z" fill="var(--ink-blue)" />
                </marker>
              </defs>
              {arrows.map((arrow) => (
                <path
                  key={arrow.key}
                  d={arrow.d}
                  fill="none"
                  stroke={arrow.weak ? 'var(--ink-muted)' : 'var(--ink-blue)'}
                  strokeWidth={1.6}
                  strokeDasharray={arrow.weak ? '4 3' : undefined}
                  markerEnd="url(#epub-unzip-head)"
                />
              ))}
            </svg>
          )}
        </div>

        {unzipped ? (
          <div className={styles.viewer}>
            <div className={styles.viewerHead}>
              <div className={styles.viewerPath}>{file.path}</div>
              <div className={styles.viewerRole}>{file.role}</div>
            </div>
            {file.binary ? (
              <div className={styles.binary}>PNG</div>
            ) : (
              <pre className={styles.code}>{highlighted(file.content, marks[file.path] ?? [])}</pre>
            )}
          </div>
        ) : (
          <div className={styles.zippedNote} style={{alignSelf: 'center'}}>
            <p>
              To your computer this is one file. But rename it <code>book.zip</code> and any zip
              tool opens it: an EPUB is a folder of web files, zipped.
            </p>
            <Button primary onClick={() => setUnzipped(true)}>
              Unzip it ▸
            </Button>
          </div>
        )}
      </div>

      {unzipped && (
        <>
          <Controls>
            <Button
              onClick={() => {
                setUnzipped(false);
                stepper.reset();
              }}>
              ↺ Zip it again
            </Button>
          </Controls>
          <StepperBar stepper={stepper} label="Open, step" />
          <Narration>{openSteps[stepper.index].text}</Narration>
        </>
      )}
    </div>
  );
}
