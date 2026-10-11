// A small model of the paginator of internal/typesetting/layout/paginate.go: startBlock (margins
// collapse, none at a page's top), placeText (a paragraph's first two lines kept together, a
// KeepWithNext heading kept with two lines of the next block), placeImage, and a new page when the
// next thing does not fit. Floats, tables and borders are left out. Each thing it does is an event.

export type ModelBlock = {
  kind: 'heading' | 'paragraph' | 'image';
  label: string;
  lines: number; // text: how many lines it broke into
  lineHeight: number; // text: the height of each line
  height: number; // image: its height
  marginTop: number;
  marginBottom: number;
  keepWithNext: boolean;
};

export type JumpReason = 'orphan' | 'keep' | 'image' | 'line';

export type PageEvent =
  | {type: 'margin'; block: number; page: number; y: number; previous: number; top: number; used: number}
  | {type: 'top'; block: number; page: number}
  | {type: 'jump'; block: number; page: number; reason: JumpReason; needed: number; room: number}
  | {type: 'line'; block: number; line: number; page: number; y: number; height: number}
  | {type: 'image'; block: number; page: number; y: number; height: number};

/**
 * paginate places blocks on pages pageHeight high. With keepRules false, it skips placeText's
 * check that keeps lines together: what a naive paginator would do.
 */
export function paginate(blocks: readonly ModelBlock[], pageHeight: number, keepRules = true): PageEvent[] {
  const events: PageEvent[] = [];
  let page = 0;
  let y = 0;
  let marginAfter = 0;
  let empty = true;
  const newPage = () => {
    page++;
    y = 0;
    marginAfter = 0;
    empty = true;
  };
  blocks.forEach((block, index) => {
    // startBlock: under the block's top margin, collapsed with the last one's bottom margin.
    if (!empty) {
      const used = Math.max(marginAfter, block.marginTop);
      events.push({type: 'margin', block: index, page, y, previous: marginAfter, top: block.marginTop, used});
      y += used;
    } else {
      events.push({type: 'top', block: index, page});
    }
    if (block.kind === 'image') {
      if (block.height > pageHeight - y && !empty) {
        events.push({type: 'jump', block: index, page: page + 1, reason: 'image', needed: block.height, room: pageHeight - y});
        newPage();
      }
      events.push({type: 'image', block: index, page, y, height: block.height});
      y += block.height;
      empty = false;
    } else {
      let needed = 0;
      for (let line = 0; line < block.lines; line++) {
        if (line < 2 || block.keepWithNext) needed += block.lineHeight;
      }
      const next = blocks[index + 1];
      if (block.keepWithNext && next && next.kind !== 'image') {
        needed += Math.max(next.marginTop, block.marginBottom) + 2 * next.lineHeight;
      }
      if (keepRules && needed > pageHeight - y && !empty && needed <= pageHeight) {
        events.push({
          type: 'jump',
          block: index,
          page: page + 1,
          reason: block.keepWithNext ? 'keep' : 'orphan',
          needed,
          room: pageHeight - y,
        });
        newPage();
      }
      for (let line = 0; line < block.lines; line++) {
        if (block.lineHeight > pageHeight - y && !empty) {
          events.push({type: 'jump', block: index, page: page + 1, reason: 'line', needed: block.lineHeight, room: pageHeight - y});
          newPage();
        }
        events.push({type: 'line', block: index, line, page, y, height: block.lineHeight});
        y += block.lineHeight;
        empty = false;
      }
    }
    marginAfter = block.marginBottom;
  });
  return events;
}
