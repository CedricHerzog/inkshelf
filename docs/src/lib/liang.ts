// Liang's hyphenation, as internal/typesetting/hyphen does it (Hyphenator.Points and
// Hyphenator.hyphenate in hyphen.go), with the patterns of hyphenPatterns.ts. Besides the points,
// it returns every step, so that a widget can show the patterns sliding over the word.

import {patternSubsets} from './hyphenPatterns';

export type Language = keyof typeof patternSubsets;

/** The minimum lengths of hyphen.go's hyphenators table: letters kept before and after a hyphen. */
export const minimums: Record<Language, {leftMin: number; rightMin: number; file: string}> = {
  en: {leftMin: 2, rightMin: 3, file: 'hyph-en-us'},
  fr: {leftMin: 2, rightMin: 3, file: 'hyph-fr'},
  de: {leftMin: 2, rightMin: 2, file: 'hyph-de-1996'},
};

/** The words hyph-en-us.hyp.txt hyphenates by hand, which the patterns get wrong. */
const englishExceptions = [
  'as-so-ciate', 'as-so-ciates', 'dec-li-na-tion', 'oblig-a-tory', 'phil-an-thropic', 'present',
  'presents', 'project', 'projects', 'reci-procity', 're-cog-ni-zance', 'ref-or-ma-tion',
  'ret-ri-bu-tion', 'ta-ble',
];

export type Pattern = {
  text: string; // as in the file: "hy3ph"
  letters: string; // without its digits: "hyph"
  values: number[]; // the value of the gap before each letter, and after the last
};

function parsePattern(text: string): Pattern {
  let letters = '';
  const values = [0];
  for (const character of text) {
    if (character >= '0' && character <= '9') {
      values[values.length - 1] = Number(character);
      continue;
    }
    letters += character;
    values.push(0);
  }
  return {text, letters, values};
}

const parsed: Record<Language, Pattern[]> = {
  en: patternSubsets.en.patterns.split(/\s+/).filter(Boolean).map(parsePattern),
  fr: patternSubsets.fr.patterns.split(/\s+/).filter(Boolean).map(parsePattern),
  de: patternSubsets.de.patterns.split(/\s+/).filter(Boolean).map(parsePattern),
};

export function patternCount(language: Language): {carried: number; total: number} {
  return {carried: parsed[language].length, total: patternSubsets[language].total};
}

/** A pattern found in the dotted word, at index `at` of ".word.". */
export type Match = {at: number; pattern: Pattern};

/** What Liang's algorithm does to one word without apostrophes, in lowercase. */
export type Trace = {
  dotted: string[]; // ".", the letters, "."
  matches: Match[]; // in the order the trie finds them: by start, then shortest first
  levels: number[]; // levels[gap]: the highest value of the gap before dotted[gap]
  points: number[]; // offsets in the word where a hyphen may go
  exception: boolean; // hyphenated from the exceptions list, not by the patterns
  tooShort: boolean; // shorter than leftMin + rightMin: no hyphen at all
};

/** traceWord runs Liang's algorithm on letters, a lowercase word, as hyphenate does. */
export function traceWord(language: Language, letters: string): Trace {
  const {leftMin, rightMin} = minimums[language];
  const characters = [...letters];
  const count = characters.length;
  const dotted = ['.', ...characters, '.'];
  const levels = new Array<number>(dotted.length + 1).fill(0);
  const trace: Trace = {dotted, matches: [], levels, points: [], exception: false, tooShort: false};
  if (count < leftMin + rightMin) {
    trace.tooShort = true;
    return trace;
  }
  if (language === 'en') {
    const exception = englishExceptions.find((word) => word.replaceAll('-', '') === letters);
    if (exception) {
      trace.exception = true;
      let offset = 0;
      for (const character of exception) {
        if (character === '-') {
          if (offset >= leftMin && offset <= count - rightMin) trace.points.push(offset);
          continue;
        }
        offset++;
      }
      return trace;
    }
  }
  const joined = dotted.join('');
  for (let start = 0; start < dotted.length; start++) {
    const found = parsed[language]
      .filter((pattern) => joined.startsWith(pattern.letters, start))
      .sort((first, second) => first.letters.length - second.letters.length);
    for (const pattern of found) {
      trace.matches.push({at: start, pattern});
      pattern.values.forEach((value, gap) => {
        levels[start + gap] = Math.max(levels[start + gap], value);
      });
    }
  }
  // A hyphen after the word's first `point` letters is in the gap before dotted[point + 1].
  for (let point = leftMin; point <= count - rightMin; point++) {
    if (levels[point + 1] % 2 === 1) trace.points.push(point);
  }
  return trace;
}

const punctuation = /[\p{P}\p{S}]/u;
const letter = /[\p{L}\p{Mn}]/u;
const apostrophe = /['’]/;

/**
 * points returns the rune offsets in word where a hyphen may go, as Hyphenator.Points: the
 * punctuation around the word is skipped, an apostrophe splits it, and a word with anything else
 * than letters inside has none.
 */
export function points(language: Language, word: string): number[] {
  const runes = [...word.toLowerCase()];
  let first = 0;
  let last = runes.length;
  while (first < last && punctuation.test(runes[first])) first++;
  while (last > first && punctuation.test(runes[last - 1])) last--;
  const found: number[] = [];
  let partStart = first;
  for (let index = first; index <= last; index++) {
    if (index < last && letter.test(runes[index])) continue;
    if (index < last && !apostrophe.test(runes[index])) return [];
    const part = runes.slice(partStart, index).join('');
    for (const point of traceWord(language, part).points) found.push(partStart + point);
    partStart = index + 1;
  }
  return found;
}

/** hyphenated returns word with a hyphen at each of its points. */
export function hyphenated(language: Language, word: string): string {
  const at = new Set(points(language, word));
  return [...word].map((character, index) => (at.has(index) ? '-' : '') + character).join('');
}
