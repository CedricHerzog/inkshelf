// Greedy line breaking and justification, as internal/typesetting/layout does them for a block of
// one style: splitFragments and groupWords (fragments.go) cut the text into words, breakLines,
// hyphenate, finishLine and cutWord (line_breaking.go) fill the lines, and fitLine
// (line_placement.go) aligns them. Every turn of breakLines' loop is recorded as a step.

/** What follows a word: a space, a hyphen drawn only at a line's end, or nothing (after a dash). */
export type BreakKind = 'space' | 'soft' | 'direct' | 'none';

export type Word = {
  text: string;
  width: number;
  breakAfter: BreakKind;
  /** Set on a word that hyphenation may still split: one not split or tried already. */
  whole: boolean;
  /** The offset of its first character in the paragraph. */
  start: number;
};

export type Measure = (text: string) => number;

/**
 * splitWords cuts text into words as splitFragments and groupWords do for alphabetic text: a
 * space ends a word (and is not drawn), a dash ends one too and stays at its end.
 */
export function splitWords(text: string, measure: Measure): Word[] {
  const words: Word[] = [];
  let current = '';
  let start = 0;
  let offset = 0;
  const end = (kind: BreakKind) => {
    if (current) {
      words.push({text: current, width: measure(current), breakAfter: kind, whole: true, start});
    } else if (kind === 'space' && words.length > 0) {
      words[words.length - 1].breakAfter = 'space';
    }
    current = '';
  };
  for (const character of text) {
    if (/\s/.test(character)) {
      end('space');
    } else {
      if (!current) start = offset;
      current += character;
      if (character === '-' || character === '–' || character === '—') end('direct');
    }
    offset++;
  }
  end('none');
  return words;
}

/** The branches of breakLines' switch, in its order. */
export type Branch = 'fits' | 'hyphenate' | 'finish' | 'cut';

export type Step = {
  candidate: Word;
  branch: Branch;
  /** The width the line may take, and how much of it the line and the gap before the word use. */
  available: number;
  used: number;
  gap: number;
  /** hyphenate: the points tried, and the split found (null: none fits, the word stays whole). */
  points?: number[];
  split?: [Word, Word] | null;
  /** cut: the parts the word was cut into (one: it could not be cut). */
  cutParts?: Word[];
  /** finish: the line's last word moved to the next line, as its hyphen did not fit. */
  movedBack?: boolean;
  /** The lines finished before the step, the line being filled before and after it. */
  linesBefore: number;
  lineBefore: Word[];
  lines: Word[][];
  line: Word[];
};

export type BreakOptions = {
  width: number;
  indent: number;
  measure: Measure;
  /** null when the block is not hyphenated (hyphens: auto) or its language has no patterns. */
  hyphenPoints: ((word: string) => number[]) | null;
};

function gapAfter(word: Word, space: number): number {
  return word.breakAfter === 'space' ? space : 0;
}

/** splitWord splits a word before its character at `at`: the start ends with `breakAfter`. */
function splitWord(word: Word, at: number, breakAfter: BreakKind, measure: Measure): [Word, Word] {
  const characters = [...word.text];
  const head = characters.slice(0, at).join('');
  const tail = characters.slice(at).join('');
  return [
    {text: head, width: measure(head), breakAfter, whole: false, start: word.start},
    {text: tail, width: measure(tail), breakAfter: word.breakAfter, whole: false, start: word.start + at},
  ];
}

/** cutWord cuts a word too wide for a line: the longest start that fits, one character at least. */
function cutWord(word: Word, width: number, measure: Measure): Word[] {
  const runes = [...word.text].length;
  if (runes < 2) return [word];
  let fitting = 1;
  while (fitting + 1 < runes && splitWord(word, fitting + 1, 'direct', measure)[0].width <= width) {
    fitting++;
  }
  return splitWord(word, fitting, 'direct', measure);
}

/** breakLines fills lines greedily and returns every step and the lines. */
export function breakLines(words: Word[], options: BreakOptions): {steps: Step[]; lines: Word[][]} {
  const {measure, hyphenPoints} = options;
  const space = measure(' ');
  const hyphen = measure('-');
  const firstWidth = options.width - Math.min(options.indent, Math.floor(options.width / 2));
  const lines: Word[][] = [];
  let line: Word[] = [];
  let used = 0;
  const queue = [...words];
  const steps: Step[] = [];
  const available = () => (lines.length === 0 ? firstWidth : options.width);

  // finishLine ends the line. A line ending at a soft hyphen draws a hyphen, which breaking did
  // not count: if it does not fit, the last word moves to the next line.
  const finishLine = (): boolean => {
    const last = line[line.length - 1];
    if (line.length > 1 && last.breakAfter === 'soft' && used + hyphen > available()) {
      lines.push(line.slice(0, -1));
      line = [last];
      used = last.width;
      return true;
    }
    lines.push(line);
    line = [];
    used = 0;
    return false;
  };

  let guard = 0;
  while (queue.length > 0 && guard++ < 5000) {
    const candidate = queue[0];
    const gap = line.length > 0 ? gapAfter(line[line.length - 1], space) : 0;
    const room = available();
    const step: Step = {
      candidate,
      branch: 'fits',
      available: room,
      used,
      gap,
      linesBefore: lines.length,
      lineBefore: [...line],
      lines: [],
      line: [],
    };
    if (used + gap + candidate.width <= room) {
      line.push(candidate);
      used += gap + candidate.width;
      queue.shift();
    } else if (candidate.whole && hyphenPoints) {
      step.branch = 'hyphenate';
      queue.shift();
      const tried = {...candidate, whole: false};
      const found = hyphenPoints(candidate.text);
      step.points = found;
      step.split = null;
      for (const point of [...found].reverse()) {
        const parts = splitWord(tried, point, 'soft', measure);
        if (parts[0].width + hyphen <= room - used - gap) {
          step.split = parts;
          break;
        }
      }
      queue.unshift(...(step.split ?? [tried]));
    } else if (line.length > 0) {
      step.branch = 'finish';
      step.movedBack = finishLine();
    } else {
      step.branch = 'cut';
      queue.shift();
      const parts = cutWord(candidate, room, measure);
      step.cutParts = parts;
      if (parts.length === 1) {
        line = parts;
        used = candidate.width;
        finishLine();
      } else {
        queue.unshift(...parts);
      }
    }
    step.lines = lines.map((finished) => [...finished]);
    step.line = [...line];
    steps.push(step);
  }
  const all = line.length > 0 ? [...lines, line] : lines;
  return {steps, lines: all};
}

export type PlacedWord = {word: Word; x: number; hyphen: boolean};

/**
 * placeLine places a line's words as fitLine and placeWords do: justification spreads the room
 * left over the gaps that stretch (spaces), a pixel more for the first ones when it does not
 * divide evenly. The last line of a paragraph is not justified.
 */
export function placeLine(
  words: Word[],
  options: {left: number; width: number; justify: boolean; last: boolean; measure: Measure},
): PlacedWord[] {
  const space = options.measure(' ');
  const hyphenated = !options.last && words[words.length - 1].breakAfter === 'soft';
  let natural = 0;
  let gaps = 0;
  words.forEach((word, position) => {
    natural += word.width;
    if (position < words.length - 1) {
      natural += gapAfter(word, space);
      if (word.breakAfter === 'space') gaps++;
    }
  });
  if (hyphenated) natural += options.measure('-');
  const extra = options.justify && !options.last && gaps > 0 ? Math.max(options.width - natural, 0) : 0;
  let x = options.left;
  let gapIndex = 0;
  return words.map((word, position) => {
    const placed = {word, x, hyphen: hyphenated && position === words.length - 1};
    x += word.width;
    if (position < words.length - 1) {
      x += gapAfter(word, space);
      if (word.breakAfter === 'space' && gaps > 0) {
        x += Math.floor(extra / gaps) + (gapIndex < extra % gaps ? 1 : 0);
        gapIndex++;
      }
    }
    return placed;
  });
}
