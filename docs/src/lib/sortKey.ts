// How Inkshelf sorts titles, as internal/servers/sorting.go does: SortKey folds a title (no
// accents, ligatures spelled out), spells its apostrophes and dashes one way and drops a
// leading French or English article; CompareTitles compares the lowercased keys with
// naturalorder.Compare (internal/naturalorder), runs of digits as numbers, then the full titles.

const ligatures: readonly [string, string][] = [
  ['œ', 'oe'],
  ['Œ', 'Oe'],
  ['æ', 'ae'],
  ['Æ', 'Ae'],
  ['ß', 'ss'],
];

/** fold returns title without its accents and ligatures, as servers.Fold. */
export function fold(title: string): string {
  let spelled = title;
  for (const [ligature, letters] of ligatures) spelled = spelled.split(ligature).join(letters);
  return spelled.normalize('NFD').replace(/\p{Mn}/gu, '');
}

/** punctuation: the apostrophes, dashes and no-break spaces that sorting.go spells one way. */
function spellPunctuation(title: string): string {
  return title
    .replace(/[’‘ʼ]/g, "'")
    .replace(/[‐‑‒–—−]/g, '-')
    .replace(/[  ]/g, ' ');
}

const articles = ['le ', 'la ', 'les ', "l'", 'un ', 'une ', 'des ', 'the ', 'a ', 'an '];
const frenchWords = ['la ', 'le ', 'les ', "l'", 'un ', 'une ', 'des ', 'du '];

function startsWithFold(text: string, word: string): boolean {
  return text.length > word.length && text.slice(0, word.length).toLowerCase() === word;
}

function startsWithVowelOrH(text: string): boolean {
  const first = fold(Array.from(text)[0] ?? '').toLowerCase();
  return first !== '' && 'aeiouyh'.includes(first[0]);
}

/** What withoutArticle returns, and the article it dropped, if any. */
export function withoutArticle(title: string): {rest: string; article: string | null} {
  for (const article of articles) {
    if (!startsWithFold(title, article)) continue;
    const rest = title.slice(article.length).replace(/^ +/, '');
    if (rest === '') return {rest: title, article: null};
    if (article === "l'" && !startsWithVowelOrH(rest)) return {rest: title, article: null};
    if (article === 'a ' && frenchWords.some((word) => startsWithFold(rest, word))) return {rest: title, article: null};
    return {rest, article: title.slice(0, article.length)};
  }
  return {rest: title, article: null};
}

/** sortKey is servers.SortKey. */
export function sortKey(title: string): string {
  return fold(withoutArticle(spellPunctuation(title).trim()).rest);
}

function digitPrefix(text: string): string {
  return /^[0-9]*/.exec(text)![0];
}

function compareNumbers(left: number, right: number): number {
  return left < right ? -1 : left > right ? 1 : 0;
}

/**
 * naturalCompare is naturalorder.Compare: runs of digits compare as numbers, the rest character
 * by character (Go compares bytes; for the folded titles here, characters order the same way).
 */
export function naturalCompare(leftText: string, rightText: string): number {
  let left = leftText;
  let right = rightText;
  while (left !== '' && right !== '') {
    const leftDigits = digitPrefix(left);
    const rightDigits = digitPrefix(right);
    if (leftDigits === '' || rightDigits === '') {
      if (left[0] !== right[0]) return compareNumbers(left.charCodeAt(0), right.charCodeAt(0));
      left = left.slice(1);
      right = right.slice(1);
      continue;
    }
    const leftNumber = leftDigits.replace(/^0+/, '');
    const rightNumber = rightDigits.replace(/^0+/, '');
    if (leftNumber.length !== rightNumber.length) return compareNumbers(leftNumber.length, rightNumber.length);
    if (leftNumber !== rightNumber) return leftNumber < rightNumber ? -1 : 1;
    left = left.slice(leftDigits.length);
    right = right.slice(rightDigits.length);
  }
  return compareNumbers(left.length, right.length);
}

/** compareTitles is servers.CompareTitles. */
export function compareTitles(first: string, second: string): number {
  const byKey = naturalCompare(sortKey(first).toLowerCase(), sortKey(second).toLowerCase());
  if (byKey !== 0) return byKey;
  return first < second ? -1 : first > second ? 1 : 0;
}

/** plainCompare is what a naive sort does: the raw titles, character by character. */
export function plainCompare(first: string, second: string): number {
  return first < second ? -1 : first > second ? 1 : 0;
}
