// A small copy of Inkshelf's CSS selectors and cascade, enough for the site's widgets.
// Selectors follow internal/web/css/selector.go (parseSelector, specificity, the pseudo-classes
// it follows); matching and the cascade follow internal/web/css/cascade.go (matchesFrom,
// Cascade.matching, Cascade.Compute). Attribute selectors and pseudo-elements are left out.

/** CssElement is what a selector may look at: the element, its parent, its previous sibling. */
export type CssElement = {
  name: string;
  id?: string;
  classes: readonly string[];
  parent: CssElement | null;
  previous: CssElement | null;
};

type Combinator = 'descendant' | 'child' | 'nextSibling' | 'subsequentSibling';

type PseudoClass =
  | {kind: 'first-child' | 'first-of-type' | 'root' | 'link'}
  | {kind: 'nth-child'; step: number; offset: number}
  | {kind: 'not'; negated: Compound[]};

type Compound = {
  name: string; // '' for any
  id: string;
  classes: string[];
  pseudoClasses: PseudoClass[];
  combinator: Combinator; // to the compound on its left
};

export type Selector = {
  text: string;
  compounds: Compound[]; // right to left: compounds[0] is the element's
  specificity: number;
};

/** Specificities in one number, as selector.go counts them: ids, classes, types. */
export const idSpecificity = 1 << 20;
export const classSpecificity = 1 << 10;

/** specificityParts splits a specificity into (ids, classes, types). */
export function specificityParts(specificity: number): [number, number, number] {
  return [
    Math.floor(specificity / idSpecificity),
    Math.floor(specificity / classSpecificity) % 1024,
    specificity % 1024,
  ];
}

/** Pseudo-classes that need what comes after the element: a stream doesn't know it yet. */
export const needsTheFuture = [
  'last-child', 'only-child', 'empty', 'last-of-type', 'only-of-type', 'nth-last-child',
  'nth-last-of-type',
];
const needsAUser = ['hover', 'focus', 'active', 'visited', 'focus-within', 'focus-visible', 'target'];

export type ParsedSelector = {selector: Selector} | {dropped: string};

/** parseSelector parses one complex selector, or says why Inkshelf drops it. */
export function parseSelector(text: string): ParsedSelector {
  const source = text.trim();
  if (source === '') return {dropped: 'an empty selector'};
  // Split into compounds and combinators, outside of parentheses.
  const parts: {compound: string; combinator: Combinator}[] = [];
  let current = '';
  let depth = 0;
  let pending: Combinator | null = null;
  const push = () => {
    if (current === '') return;
    parts.push({compound: current, combinator: pending ?? 'descendant'});
    current = '';
    pending = null;
  };
  for (const character of source) {
    if (character === '(') depth++;
    if (character === ')') depth--;
    if (depth === 0 && /\s/.test(character)) {
      if (current !== '') {
        push();
        pending = 'descendant';
      }
      continue;
    }
    if (depth === 0 && '>+~'.includes(character)) {
      if (current !== '') push();
      if (parts.length === 0) return {dropped: 'a combinator with nothing on its left'};
      pending = character === '>' ? 'child' : character === '+' ? 'nextSibling' : 'subsequentSibling';
      continue;
    }
    current += character;
  }
  if (current === '') return {dropped: 'a combinator with nothing on its right'};
  push();
  const compounds: Compound[] = [];
  let specificity = 0;
  for (let index = 0; index < parts.length; index++) {
    const parsed = parseCompound(parts[index].compound);
    if ('dropped' in parsed) return parsed;
    // The combinator read before a compound relates it to the compound on its left.
    parsed.compound.combinator = parts[index].combinator;
    compounds.unshift(parsed.compound);
    specificity += compoundSpecificity(parsed.compound);
  }
  return {selector: {text: source, compounds, specificity}};
}

function parseCompound(text: string): {compound: Compound} | {dropped: string} {
  const compound: Compound = {name: '', id: '', classes: [], pseudoClasses: [], combinator: 'descendant'};
  if (text === '') return {dropped: 'an empty compound'};
  const pattern = /^(\*|[a-zA-Z][\w-]*)?/;
  const head = pattern.exec(text)?.[0] ?? '';
  compound.name = head === '*' ? '' : head.toLowerCase();
  let rest = text.slice(head.length);
  while (rest !== '') {
    let match: RegExpExecArray | null;
    if ((match = /^#([\w-]+)/.exec(rest))) {
      compound.id = match[1];
    } else if ((match = /^\.([\w-]+)/.exec(rest))) {
      compound.classes.push(match[1]);
    } else if ((match = /^::?([\w-]+)(\(([^)]*)\))?/.exec(rest))) {
      const name = match[1].toLowerCase();
      if (['first-letter', 'first-line', 'before', 'after'].includes(name) || rest.startsWith('::')) {
        return {dropped: `::${name} styles a pseudo-element, not the element`};
      }
      if (needsTheFuture.includes(name)) {
        return {dropped: `:${name} needs to know what comes after the element`};
      }
      if (needsAUser.includes(name)) return {dropped: `:${name} needs a user's action`};
      if (match[2] !== undefined) {
        if (name === 'nth-child') {
          const nth = parseNth(match[3]);
          if (!nth) return {dropped: `:nth-child(${match[3]}) cannot be read`};
          compound.pseudoClasses.push({kind: 'nth-child', ...nth});
        } else if (name === 'not') {
          const negated: Compound[] = [];
          for (const argument of match[3].split(',')) {
            const inner = parseCompound(argument.trim());
            if ('dropped' in inner) return {dropped: `:not(${match[3]}): ${inner.dropped}`};
            negated.push(inner.compound);
          }
          compound.pseudoClasses.push({kind: 'not', negated});
        } else {
          return {dropped: `:${name}() is not one Inkshelf follows`};
        }
      } else if (['first-child', 'first-of-type', 'root', 'link'].includes(name)) {
        compound.pseudoClasses.push({kind: name as 'first-child'});
      } else {
        return {dropped: `:${name} is not one Inkshelf follows`};
      }
    } else {
      return {dropped: `“${rest}” cannot be parsed`};
    }
    rest = rest.slice(match[0].length);
  }
  return {compound};
}

function parseNth(argument: string): {step: number; offset: number} | null {
  const text = argument.replace(/\s+/g, '').toLowerCase();
  if (text === 'odd') return {step: 2, offset: 1};
  if (text === 'even') return {step: 2, offset: 0};
  const match = /^([+-]?\d*)n([+-]\d+)?$/.exec(text);
  if (match) {
    const step = match[1] === '' || match[1] === '+' ? 1 : match[1] === '-' ? -1 : Number(match[1]);
    return {step, offset: match[2] ? Number(match[2]) : 0};
  }
  return /^[+-]?\d+$/.test(text) ? {step: 0, offset: Number(text)} : null;
}

function compoundSpecificity(part: Compound): number {
  let specificity = part.classes.length * classSpecificity;
  if (part.name !== '') specificity += 1;
  if (part.id !== '') specificity += idSpecificity;
  for (const pseudo of part.pseudoClasses) {
    if (pseudo.kind !== 'not') {
      specificity += classSpecificity;
      continue;
    }
    specificity += Math.max(0, ...pseudo.negated.map(compoundSpecificity)); // its most specific argument
  }
  return specificity;
}

/** matches reports whether the element matches the selector (matchesFrom in cascade.go). */
export function matches(selector: Selector, element: CssElement): boolean {
  return matchesFrom(selector, 0, element);
}

function matchesFrom(selector: Selector, index: number, element: CssElement): boolean {
  const current = selector.compounds[index];
  if (!compoundMatches(current, element)) return false;
  if (index === selector.compounds.length - 1) return true;
  switch (current.combinator) {
    case 'child':
      return element.parent !== null && matchesFrom(selector, index + 1, element.parent);
    case 'descendant':
      for (let ancestor = element.parent; ancestor; ancestor = ancestor.parent) {
        if (matchesFrom(selector, index + 1, ancestor)) return true;
      }
      return false;
    case 'nextSibling':
      return element.previous !== null && matchesFrom(selector, index + 1, element.previous);
    case 'subsequentSibling':
      for (let previous = element.previous; previous; previous = previous.previous) {
        if (matchesFrom(selector, index + 1, previous)) return true;
      }
      return false;
  }
}

function compoundMatches(part: Compound, element: CssElement): boolean {
  if (part.name !== '' && element.name !== part.name) return false;
  if (part.id !== '' && element.id !== part.id) return false;
  if (part.classes.some((name) => !element.classes.includes(name))) return false;
  return part.pseudoClasses.every((pseudo) => pseudoMatches(pseudo, element));
}

function pseudoMatches(pseudo: PseudoClass, element: CssElement): boolean {
  switch (pseudo.kind) {
    case 'first-child':
      return element.previous === null;
    case 'first-of-type':
      for (let previous = element.previous; previous; previous = previous.previous) {
        if (previous.name === element.name) return false;
      }
      return true;
    case 'nth-child': {
      let position = 1;
      for (let previous = element.previous; previous; previous = previous.previous) position++;
      if (pseudo.step === 0) return position === pseudo.offset;
      const steps = position - pseudo.offset;
      return steps % pseudo.step === 0 && steps / pseudo.step >= 0;
    }
    case 'root':
      return element.parent === null;
    case 'link':
      return false; // the widgets have no links
    case 'not':
      return !pseudo.negated.some((negated) => compoundMatches(negated, element));
  }
}

/* ---------- Declarations and the cascade ---------- */

export type Declaration = {property: string; value: string; important: boolean};

const sides = ['top', 'right', 'bottom', 'left'];

/** parseDeclarations reads "a: b; c: d !important", margin and padding expanded. */
export function parseDeclarations(text: string): Declaration[] {
  const declarations: Declaration[] = [];
  for (const part of text.split(';')) {
    const colon = part.indexOf(':');
    if (colon < 0) continue;
    const property = part.slice(0, colon).trim().toLowerCase();
    let value = part.slice(colon + 1).trim().replace(/\s+/g, ' ');
    const important = /!\s*important\s*$/i.test(value);
    if (important) value = value.replace(/!\s*important\s*$/i, '').trim();
    if (!property || !value) continue;
    if (property === 'margin' || property === 'padding') {
      const values = value.split(' ');
      const [top, right = top, bottom = top, left = right] = values;
      [top, right, bottom, left].forEach((sideValue, index) =>
        declarations.push({property: `${property}-${sides[index]}`, value: sideValue, important}),
      );
      continue;
    }
    declarations.push({property, value, important});
  }
  return declarations;
}

export type Rule = {
  /** A key that stays the same while the rule is edited. */
  key: string;
  selector: string;
  declarations: Declaration[];
  origin: 'user agent' | 'book';
};

/** Layer is one set of declarations in the order Compute applies them. */
export type Layer = {
  rule: Rule | null; // null for the style attribute
  selector: Selector | null;
  order: number; // the rule's place in the cascade, -1 for the style attribute
};

export type CascadeResult = {
  /** The layers that apply, from the first applied to the last (Compute's `layers`). */
  layers: Layer[];
  /** The selectors Inkshelf dropped, with why. */
  dropped: {rule: Rule; selector: string; why: string}[];
  /** For each property, the winning value and the layer and pass it came from. */
  winners: Record<string, {value: string; layer: number; important: boolean}>;
};

/**
 * compute follows Cascade.Compute: the matching rules sorted by specificity then order, the
 * style attribute above them; normal declarations applied in that order, then the !important
 * ones in the same order. The last write of a property wins.
 */
export function compute(rules: readonly Rule[], element: CssElement, styleAttribute?: string): CascadeResult {
  const dropped: CascadeResult['dropped'] = [];
  const matched: Layer[] = [];
  rules.forEach((rule, order) => {
    for (const part of rule.selector.split(',')) {
      const parsed = parseSelector(part);
      if ('dropped' in parsed) {
        dropped.push({rule, selector: part.trim(), why: parsed.dropped});
        continue;
      }
      if (matches(parsed.selector, element)) matched.push({rule, selector: parsed.selector, order});
    }
  });
  matched.sort((first, second) =>
    first.selector!.specificity !== second.selector!.specificity
      ? first.selector!.specificity - second.selector!.specificity
      : first.order - second.order,
  );
  const layers = [...matched];
  const inline = styleAttribute !== undefined ? parseDeclarations(styleAttribute) : [];
  const inlineRule: Rule = {key: 'style', selector: 'style=""', declarations: inline, origin: 'book'};
  if (styleAttribute !== undefined) layers.push({rule: inlineRule, selector: null, order: -1});
  const winners: CascadeResult['winners'] = {};
  for (const important of [false, true]) {
    layers.forEach((layer, index) => {
      const declarations = layer.selector === null ? inline : layer.rule!.declarations;
      for (const declared of declarations) {
        if (declared.important === important) {
          winners[declared.property] = {value: declared.value, layer: index, important};
        }
      }
    });
  }
  return {layers, dropped, winners};
}
