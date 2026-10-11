// Reading XHTML as a stream of tokens with a stack of open elements, as Inkshelf's chapter
// builder does (builder.parse, start, end and close in internal/ebook/epub/build.go). Each
// element keeps its parent and its previous sibling, which is all a selector may look at.

import type {CssElement} from './cssCascade';

export type Token = {
  kind: 'start' | 'end' | 'text';
  name: string; // the element's name; '' for text
  attributes: Record<string, string>;
  selfClosing: boolean;
  from: number; // offsets in the source
  to: number;
};

/** tokenize splits simple XHTML (no comments, no entities) into tokens, blank text dropped. */
export function tokenize(source: string): Token[] {
  const tokens: Token[] = [];
  const pattern = /<(\/?)([a-zA-Z][\w-]*)([^>]*?)(\/?)>|[^<]+/g;
  for (const match of source.matchAll(pattern)) {
    const from = match.index ?? 0;
    const to = from + match[0].length;
    if (match[2] === undefined) {
      if (match[0].trim() !== '') {
        tokens.push({kind: 'text', name: '', attributes: {}, selfClosing: false, from, to});
      }
      continue;
    }
    const attributes: Record<string, string> = {};
    for (const attribute of match[3].matchAll(/([\w:-]+)="([^"]*)"/g)) {
      attributes[attribute[1].toLowerCase()] = attribute[2];
    }
    tokens.push({
      kind: match[1] === '/' ? 'end' : 'start',
      name: match[2].toLowerCase(),
      attributes,
      selfClosing: match[4] === '/',
      from,
      to,
    });
  }
  return tokens;
}

/** StreamElement is an element of the stream: a CssElement with where it is in the source. */
export type StreamElement = CssElement & {
  key: number;
  from: number; // its start tag's offset
  startTo: number; // the end of its start tag
  to: number | null; // the end of its end tag, null while open
  lastChild: StreamElement | null;
  parent: StreamElement | null;
  previous: StreamElement | null;
};

export type Frame = {
  token: Token;
  /** The open elements, the innermost last (the stack after the token). */
  stack: StreamElement[];
  /** The element the token opened, if it opened one. */
  opened: StreamElement | null;
  /** The element the token closed, if it closed one. */
  closed: StreamElement | null;
};

/**
 * stream reads the tokens one by one and returns a frame per token. Each element is made once:
 * its parent and previous sibling are set when it opens and never change; it is closed at a
 * frame if its `to` is at or before the end of that frame's token.
 */
export function stream(tokens: readonly Token[]): {frames: Frame[]; elements: StreamElement[]} {
  const elements: StreamElement[] = [];
  const stack: StreamElement[] = [];
  const frames: Frame[] = [];
  for (const token of tokens) {
    let opened: StreamElement | null = null;
    let closed: StreamElement | null = null;
    if (token.kind === 'start') {
      const parent = stack[stack.length - 1] ?? null;
      opened = {
        key: elements.length,
        name: token.name,
        id: token.attributes.id,
        classes: (token.attributes.class ?? '').split(/\s+/).filter(Boolean),
        parent,
        previous: parent ? parent.lastChild : null, // the last child closed so far
        lastChild: null,
        from: token.from,
        startTo: token.to,
        to: null,
      };
      elements.push(opened);
      stack.push(opened);
      if (token.selfClosing) {
        closed = stack.pop()!;
        closed.to = token.to;
        if (closed.parent) closed.parent.lastChild = closed;
      }
    } else if (token.kind === 'end') {
      const index = stack.map((element) => element.name).lastIndexOf(token.name);
      if (index >= 0) {
        while (stack.length > index) {
          closed = stack.pop()!;
          closed.to = token.to;
          if (closed.parent) closed.parent.lastChild = closed;
        }
      }
    }
    frames.push({token, stack: [...stack], opened, closed});
  }
  return {frames, elements};
}

/** knownRelatives returns what a selector may look at when element opens. */
export function knownRelatives(element: StreamElement): {ancestors: StreamElement[]; previous: StreamElement[]} {
  const ancestors: StreamElement[] = [];
  for (let ancestor = element.parent; ancestor; ancestor = ancestor.parent) ancestors.push(ancestor);
  const previous: StreamElement[] = [];
  for (let sibling = element.previous; sibling; sibling = sibling.previous) previous.push(sibling);
  return {ancestors, previous};
}
