// Blocks dropping onto pages, as the paginator of internal/typesetting/layout/paginate.go places
// them (modelled by src/lib/paginationModel.ts): a y cursor going down, margins that collapse, a
// paragraph's first two lines kept together, a heading kept with what follows.

import React, {useEffect, useMemo, useState} from 'react';
import clsx from 'clsx';
import {Controls, Narration, StepperBar, Toggle, useStepper} from '@site/src/components/kit';
import {paginate, type ModelBlock, type PageEvent} from '@site/src/lib/paginationModel';
import styles from './PagesWidgets.module.css';

const paragraph = (label: string, lines: number): ModelBlock => ({
  kind: 'paragraph', label, lines, lineHeight: 16, height: 0, marginTop: 8, marginBottom: 8, keepWithNext: false,
});
const heading = (label: string): ModelBlock => ({
  kind: 'heading', label, lines: 1, lineHeight: 24, height: 0, marginTop: 28, marginBottom: 12, keepWithNext: true,
});

const blocks: ModelBlock[] = [
  heading('Chapter 3'),
  paragraph('paragraph 1', 4),
  paragraph('paragraph 2', 3),
  {kind: 'image', label: 'the picture', lines: 0, lineHeight: 0, height: 40, marginTop: 12, marginBottom: 12, keepWithNext: false},
  heading('A new section'),
  paragraph('paragraph 3', 5),
  paragraph('paragraph 4', 4),
  paragraph('paragraph 5', 2),
  paragraph('paragraph 6', 4),
];

const pageHeight = 260; // the text area of each page
const margin = 16; // the page's own margins around it
const pageWidth = 150;
const pageGap = 26;
const textWidth = pageWidth - 2 * margin;

function narrate(event: PageEvent | undefined, events: PageEvent[]): string {
  if (!event) return '';
  const block = blocks[event.block];
  const name = `“${block.label}”`;
  switch (event.type) {
    case 'top':
      return `${name} starts at the top of page ${event.page + 1}: no top margin there, the page's own margin is enough.`;
    case 'margin': {
      const previous = blocks[event.block - 1];
      return `Between “${previous.label}” (bottom margin ${event.previous}) and ${name} (top margin ${event.top}): the margins collapse to max(${event.previous}, ${event.top}) = ${event.used}, not ${event.previous + event.top}.`;
    }
    case 'line': {
      const placed = `Line ${event.line + 1} of ${name} goes at y = ${event.y}; y moves down ${event.height}.`;
      const next = events.find((each) => each.type === 'line' && each.block === event.block + 1);
      if (block.kind === 'heading' && next && next.page > event.page) {
        return `${placed} The next block goes on page ${next.page + 1}: this heading ends page ${event.page + 1} alone, away from its text.`;
      }
      return placed;
    }
    case 'image':
      return `${name} fits: placed at y = ${event.y}, ${event.height} high.`;
    case 'jump':
      switch (event.reason) {
        case 'keep':
          return `${name} is a heading (KeepWithNext). With the margin and the first two lines of the next block, it needs ${event.needed}; only ${event.room} are left. It moves to page ${event.page + 1}.`;
        case 'orphan':
          return `The first two lines of ${name} need ${event.needed}; only ${event.room} are left. Rather than end the page with its first line alone, the paragraph starts on page ${event.page + 1}.`;
        case 'image':
          return `${name} is ${event.needed} high and only ${event.room} are left: it goes to page ${event.page + 1}.`;
        default:
          return `The next line of ${name} does not fit in the ${event.room} left: page ${event.page + 1} starts.`;
      }
  }
}

/** The y cursor after an event, and the page it is on. */
function cursorAfter(event: PageEvent | undefined): {page: number; y: number} {
  if (!event) return {page: 0, y: 0};
  switch (event.type) {
    case 'top':
      return {page: event.page, y: 0};
    case 'margin':
      return {page: event.page, y: event.y + event.used};
    case 'jump':
      return {page: event.page, y: 0};
    default:
      return {page: event.page, y: event.y + event.height};
  }
}

export default function PagesFall() {
  const [keepRules, setKeepRules] = useState(true);
  const events = useMemo(() => paginate(blocks, pageHeight, keepRules), [keepRules]);
  const stepper = useStepper(events.length, 520);
  const {seek} = stepper;
  useEffect(() => {
    // Open on the chapter paginated; Play drops the blocks again from the first one.
    seek(events.length - 1);
  }, [events]);

  const index = stepper.index;
  const event = events[index];
  const shown = events.slice(0, index + 1);
  const pages = Math.max(...events.map((each) => each.page)) + 1;
  const cursor = cursorAfter(event);
  const width = pages * pageWidth + (pages - 1) * pageGap + 12;
  const height = pageHeight + 2 * margin + 46;
  const pageX = (page: number) => 6 + page * (pageWidth + pageGap);
  const top = 20;

  // How far into its block each line is, to draw the first line indented and the last one short.
  const lastLine = (block: number, line: number) => line === blocks[block].lines - 1;

  return (
    <div className={styles.stage}>
      <Controls>
        <Toggle label="Keep lines together (orphans, headings)" checked={keepRules} onChange={setKeepRules} />
      </Controls>
      <div style={{overflowX: 'auto'}}>
        <svg viewBox={`0 0 ${width} ${height}`} className={styles.svgFluid} style={{minWidth: 420}} role="img" aria-label="Blocks placed on pages">
          <defs>
            <pattern id="pages-hatch" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
              <line x1="0" y1="0" x2="0" y2="6" stroke="var(--ink-blue)" strokeWidth="1" opacity="0.5" />
            </pattern>
          </defs>
          {Array.from({length: pages}, (_, page) => (
            <g key={page}>
              <rect x={pageX(page)} y={top} width={pageWidth} height={pageHeight + 2 * margin} rx={6} className={clsx(styles.page, page === cursor.page && styles.pageCurrent)} />
              <text x={pageX(page) + pageWidth / 2} y={top + pageHeight + 2 * margin + 16} textAnchor="middle" className={styles.smallLabel}>
                page {page + 1}
              </text>
            </g>
          ))}
          {shown.map((each, position) => {
            const fresh = position === index;
            const x = pageX(each.page) + margin;
            const y = (value: number) => top + margin + value;
            const block = blocks[each.block];
            switch (each.type) {
              case 'margin': {
                // A margin left above a block that then moved to the next page is lost with the page.
                const after = shown[position + 1];
                if (after?.type === 'jump' && after.block === each.block) return null;
                return <rect key={position} x={x} y={y(each.y)} width={textWidth} height={each.used} className={styles.marginBand} />;
              }
              case 'line': {
                const indent = block.kind === 'paragraph' && each.line === 0 ? 10 : 0;
                const lineWidth = block.kind === 'heading' ? textWidth * 0.62 : lastLine(each.block, each.line) ? textWidth * 0.55 : textWidth - indent;
                return (
                  <rect
                    key={position}
                    x={x + indent}
                    y={y(each.y) + (block.kind === 'heading' ? 5 : 4)}
                    width={lineWidth}
                    height={block.kind === 'heading' ? 13 : 8}
                    rx={2}
                    className={clsx(block.kind === 'heading' ? styles.headingBar : styles.lineBar, fresh && styles.fresh)}
                  />
                );
              }
              case 'image':
                return (
                  <g key={position} className={clsx(fresh && styles.fresh)}>
                    <rect x={x + 14} y={y(each.y)} width={textWidth - 28} height={each.height} className={styles.imageBox} />
                    <rect x={x + 14} y={y(each.y)} width={textWidth - 28} height={each.height} fill="url(#pages-hatch)" />
                  </g>
                );
              default:
                return null;
            }
          })}
          {event?.type === 'jump' && (
            <g>
              <rect
                x={pageX(event.page - 1) + margin}
                y={top + margin + pageHeight - event.room}
                width={textWidth}
                height={Math.min(event.needed, event.room + margin + 10)}
                className={styles.ghost}
              />
              <text x={pageX(event.page - 1) + margin + 4} y={top + margin + pageHeight - event.room + 12} className={styles.cursorLabel}>
                needs {event.needed}, room {event.room}
              </text>
            </g>
          )}
          {event?.type === 'margin' && (
            <g>
              {/* The two margins, side by side from the same y: the larger one wins. */}
              <rect x={pageX(event.page) + pageWidth - margin + 2} y={top + margin + event.y} width={5} height={Math.max(event.previous, 1)} fill="var(--ink-blue)" />
              <rect x={pageX(event.page) + pageWidth - margin + 8} y={top + margin + event.y} width={5} height={Math.max(event.top, 1)} fill="var(--ink-warn)" />
              <text
                x={pageX(event.page) + pageWidth - margin - 2}
                y={top + margin + event.y + event.used / 2 + 3}
                textAnchor="end"
                className={styles.cursorLabel}
                style={{fontSize: 9}}>
                max({event.previous}, {event.top}) = {event.used}
              </text>
            </g>
          )}
          <g>
            <line
              x1={pageX(cursor.page) - 4}
              x2={pageX(cursor.page) + pageWidth + 4}
              y1={top + margin + cursor.y}
              y2={top + margin + cursor.y}
              className={styles.cursor}
            />
            <text x={pageX(cursor.page) + 2} y={top + margin + cursor.y - 3} className={styles.cursorLabel}>
              y = {cursor.y}
            </text>
          </g>
          <text x={0} y={12} className={styles.smallLabel}>
            each page holds {pageHeight} px of text
          </text>
        </svg>
      </div>
      <StepperBar stepper={stepper} label="step" />
      <Narration>{narrate(event, events)}</Narration>
    </div>
  );
}
