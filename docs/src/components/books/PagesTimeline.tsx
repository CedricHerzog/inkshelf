// What happens to an EPUB once, when it is downloaded (internal/shelf/prepare_epub.go,
// prepare_fixed_layout.go), and every time it is read (internal/reading/textbook.go,
// renderer.go), for a reflowable book and for a fixed-layout one.

import React, {useState} from 'react';
import clsx from 'clsx';
import {Narration, Segmented, StepperBar, useStepper} from '@site/src/components/kit';
import styles from './PagesWidgets.module.css';

type Lane = 'download' | 'read';
type File = {name: string; gone?: boolean};
type Stage = {lane: Lane; title: string; code: string; files?: File[]; narration: string};

const reflowable: Stage[] = [
  {
    lane: 'download',
    title: 'Fetch the EPUB',
    code: 'fetchFile',
    files: [{name: 'book.epub'}],
    narration: 'The EPUB comes from the server, over Wi-Fi, and is saved as it is: the book stays a zip of XHTML and CSS.',
  },
  {
    lane: 'download',
    title: 'Read every chapter once',
    code: 'epub.BuildIndex',
    files: [{name: 'index.json'}],
    narration:
      'BuildIndex parses every chapter, without opening its fonts, just to count its runes. With the table of contents (each entry a chapter and an offset), that is index.json.',
  },
  {
    lane: 'download',
    title: 'Prepare the cover',
    code: 'saveCover',
    files: [{name: 'cover.pg4'}],
    narration: 'The cover is the server’s, else the one the EPUB names, prepared for the screen in 16 grays.',
  },
  {
    lane: 'download',
    title: 'Count virtual pages, write book.json last',
    code: 'book.Pages = ⌈Total / 1500⌉',
    files: [{name: 'book.json'}],
    narration:
      'The book’s page count is its virtual pages: the total runes over CharsPerPage, rounded up. book.json is written last: if it exists, the download is complete. No page has been laid out.',
  },
  {
    lane: 'read',
    title: 'Open the book',
    code: 'reading.Open, LoadIndex',
    narration:
      'Opening reads index.json. If it was made by another version of the parser (its Version is not IndexVersion), the offsets could differ: it is built again.',
  },
  {
    lane: 'read',
    title: 'Lay out one chapter',
    code: 'renderer.pages → layout.Paginate',
    narration:
      'GoTo needs the pages of the saved location’s chapter, and only that chapter is laid out, at the current font size. The pages are kept in renderer.chapters, under the renderer’s own lock.',
  },
  {
    lane: 'read',
    title: 'Find and draw the page',
    code: 'layout.PageAt, Page.Draw',
    narration: 'PageAt finds the page that holds the saved offset. Page.Draw draws it into a gray image, which goes to the screen.',
  },
  {
    lane: 'read',
    title: 'Draw the neighbours ahead',
    code: 'preloadText, Neighbours',
    narration:
      'In the background, the next page and the previous one are drawn ahead, the next first, so that turning the page is instant. Prune keeps the layouts of three chapters: this one and the two around it.',
  },
  {
    lane: 'read',
    title: 'A+ or A−',
    code: 'Relayout',
    narration:
      'A new font size drops every layout and every page drawn. The chapter is laid out again at the new size, and PageAt finds the page of the same offset.',
  },
];

const fixed: Stage[] = [
  {
    lane: 'download',
    title: 'Fetch the EPUB',
    code: 'fetchFile',
    files: [{name: 'book.epub'}],
    narration: 'The EPUB comes from the server like any other.',
  },
  {
    lane: 'download',
    title: 'A fixed-layout book?',
    code: 'opened.FixedLayout()',
    narration:
      'Its package says its pages are fixed (a comic, a picture book): each page is a picture-like XHTML page made for one screen size. There is nothing to reflow.',
  },
  {
    lane: 'download',
    title: 'Draw every page now',
    code: 'renderPages, RenderPage',
    files: [{name: '0001.pg4'}, {name: '0002.pg4'}, {name: '…'}],
    narration:
      'Every page is drawn whole at the screen’s size and prepared in 16 grays, by 2 to 8 workers at once (prepareWorkers, from the number of processors).',
  },
  {
    lane: 'download',
    title: 'Become a book of pages',
    code: 'Format = ComicArchive',
    files: [{name: 'book.json'}, {name: 'book.epub', gone: true}],
    narration: 'The book is now a book of pages, like a comic: book.json says so, and the EPUB is deleted.',
  },
  {
    lane: 'read',
    title: 'Show a page',
    code: 'LoadPage',
    narration: 'Reading only unpacks the page’s .pg4 file onto the screen. No layout, ever: the page numbers are real and never change.',
  },
];

const kinds = [
  {value: 'reflowable', label: 'Reflowable EPUB'},
  {value: 'fixed', label: 'Fixed-layout EPUB'},
] as const;

export default function PagesTimeline() {
  const [kind, setKind] = useState<'reflowable' | 'fixed'>('reflowable');
  const stages = kind === 'reflowable' ? reflowable : fixed;
  const stepper = useStepper(stages.length, 1600);
  const index = stepper.index;
  const files = new Map<string, File>();
  for (const stage of stages.slice(0, index + 1)) {
    for (const file of stage.files ?? []) files.set(file.name, file);
  }

  const lane = (which: Lane, title: string) => (
    <div className={styles.lane}>
      <div className={styles.laneTitle}>{title}</div>
      <div className={styles.laneSteps}>
        {stages.map((stage, position) =>
          stage.lane === which ? (
            <div
              key={stage.title}
              className={clsx(styles.stageBox, position < index && styles.stageDone, position === index && styles.stageNow)}>
              <div>{stage.title}</div>
              <code>{stage.code}</code>
            </div>
          ) : null,
        )}
      </div>
    </div>
  );

  return (
    <div className={styles.stage}>
      <Segmented options={kinds} value={kind} onChange={setKind} />
      <div className={styles.lanes}>
        {lane('download', 'Download time: once, on Wi-Fi')}
        {lane('read', 'Read time: every time the book opens')}
      </div>
      <div className={styles.files}>
        <span className={styles.laneTitle} style={{margin: 0}}>
          books/&lt;id&gt;/
        </span>
        {[...files.values()].map((file) => (
          <span key={file.name} className={clsx(styles.file, file.gone && styles.fileGone)}>
            {file.name}
          </span>
        ))}
      </div>
      <StepperBar stepper={stepper} label="stage" />
      <Narration>{stages[index].narration}</Narration>
    </div>
  );
}
