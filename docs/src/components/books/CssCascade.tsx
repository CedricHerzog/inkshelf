// The cascade on a small chapter: pick an element and a property, and the rules that match it
// stack up as cards in the order Cascade.Compute (internal/web/css/cascade.go) applies them.
// Selectors can be edited, book rules moved, declarations made !important, and a style
// attribute added; the winning declaration lights up.

import React, {useMemo, useState} from 'react';
import clsx from 'clsx';
import {Controls, Segmented, Toggle} from '@site/src/components/kit';
import {
  compute,
  parseDeclarations,
  parseSelector,
  specificityParts,
  type CssElement,
  type Rule,
} from '@site/src/lib/cssCascade';
import {lengthOf, textWidth} from '@site/src/lib/cssLength';
import styles from './Css.module.css';

// The chapter: <div class="chapter"><h2>Two</h2><p class="first">…</p><p id="here">…</p></div>
const chapter: CssElement = {name: 'div', classes: ['chapter'], parent: null, previous: null};
const heading: CssElement = {name: 'h2', classes: [], parent: chapter, previous: null};
const first: CssElement = {name: 'p', classes: ['first'], parent: chapter, previous: heading};
const here: CssElement = {name: 'p', id: 'here', classes: [], parent: chapter, previous: first};

const nodes: {key: string; element: CssElement; label: string; depth: number}[] = [
  {key: 'div', element: chapter, label: '<div class="chapter">', depth: 0},
  {key: 'h2', element: heading, label: '<h2>Two</h2>', depth: 1},
  {key: 'first', element: first, label: '<p class="first">It was a dark night.</p>', depth: 1},
  {key: 'here', element: here, label: '<p id="here">It rained.</p>', depth: 1},
];

const userAgentRules: Rule[] = [
  {key: 'ua1', selector: 'p, blockquote, figure', declarations: parseDeclarations('margin-top: 1em; margin-bottom: 1em'), origin: 'user agent'},
  {key: 'ua2', selector: 'h1, h2, h3, h4, h5, h6', declarations: parseDeclarations('font-weight: bold; text-indent: 0'), origin: 'user agent'},
];

const initialBookRules: Rule[] = [
  {key: 'b1', selector: 'p', declarations: parseDeclarations('text-indent: 1.5em; margin: 0'), origin: 'book'},
  {key: 'b2', selector: 'p.first', declarations: parseDeclarations('text-indent: 0'), origin: 'book'},
  {key: 'b3', selector: 'div.chapter p', declarations: parseDeclarations('text-indent: 2em'), origin: 'book'},
  {key: 'b4', selector: '#here', declarations: parseDeclarations('text-indent: 3em'), origin: 'book'},
  {key: 'b5', selector: 'p:last-child', declarations: parseDeclarations('text-indent: 4em'), origin: 'book'},
  {key: 'b6', selector: 'div.chapter h2', declarations: parseDeclarations('font-weight: normal'), origin: 'book'},
];

type Property = 'text-indent' | 'margin-top' | 'font-weight';

const inlineStyle: Record<Property, string> = {
  'text-indent': 'text-indent: 5em',
  'margin-top': 'margin-top: 2em',
  'font-weight': 'font-weight: bold',
};

function declarationsText(rule: Rule) {
  return rule.declarations
    .map((declared) => `${declared.property}: ${declared.value}${declared.important ? ' !important' : ''}`)
    .join('; ');
}

// What the winning value becomes in the document, for a paragraph (font size 100%).
function outcome(property: Property, value: string | undefined): string {
  if (property === 'font-weight') {
    if (value === undefined) return 'nothing declares it: the parent’s weight is inherited (normal).';
    return `${value} → ${value === 'bold' || Number(value) >= 600 ? 'the runs are Bold.' : 'the runs are not bold.'}`;
  }
  if (value === undefined) {
    return property === 'text-indent'
      ? 'nothing declares it: the parent’s indent is inherited (0).'
      : 'nothing declares it: 0, margins are not inherited.';
  }
  const length = lengthOf(value, 100, textWidth);
  const field = property === 'text-indent' ? 'Indent' : 'MarginTop';
  return length === null ? `${value} is no length.` : `${value} → ${field}: ${length} (hundredths of an em).`;
}

export default function CssCascade() {
  const [picked, setPicked] = useState('first');
  const [property, setProperty] = useState<Property>('text-indent');
  const [bookRules, setBookRules] = useState(initialBookRules);
  const [withStyle, setWithStyle] = useState(false);

  const element = nodes.find((node) => node.key === picked)!.element;
  const rules = useMemo(() => [...userAgentRules, ...bookRules], [bookRules]);
  const result = useMemo(
    () => compute(rules, element, withStyle ? inlineStyle[property] : undefined),
    [rules, element, withStyle, property],
  );
  const winner = result.winners[property];

  const updateRule = (key: string, change: (rule: Rule) => Rule) =>
    setBookRules((current) => current.map((rule) => (rule.key === key ? change(rule) : rule)));
  const move = (index: number, by: number) =>
    setBookRules((current) => {
      const next = [...current];
      const [rule] = next.splice(index, 1);
      next.splice(index + by, 0, rule);
      return next;
    });
  const toggleImportant = (rule: Rule, declarationIndex: number) => {
    if (rule.origin !== 'book' || rule.key === 'style') return;
    updateRule(rule.key, (current) => ({
      ...current,
      declarations: current.declarations.map((declared, index) =>
        index === declarationIndex ? {...declared, important: !declared.important} : declared,
      ),
    }));
  };

  // The cards, the last applied on top.
  const cards = result.layers.map((layer, index) => ({layer, index})).reverse();

  return (
    <div>
      <Controls>
        <Segmented
          options={[
            {value: 'text-indent', label: 'text-indent'},
            {value: 'margin-top', label: 'margin-top'},
            {value: 'font-weight', label: 'font-weight'},
          ]}
          value={property}
          onChange={setProperty}
        />
        <Toggle
          label={<>add <code>style="{inlineStyle[property]}"</code></>}
          checked={withStyle}
          onChange={setWithStyle}
        />
      </Controls>
      <div className={styles.cascadeGrid}>
        <div>
          <div className={styles.panelTitle}>The chapter: pick an element</div>
          <div className={styles.domTree}>
            {nodes.map((node) => (
              <button
                key={node.key}
                type="button"
                className={clsx(styles.domNode, node.key === picked && styles.domNodePicked)}
                style={{marginLeft: node.depth * 14}}
                onClick={() => setPicked(node.key)}>
                {node.label}
              </button>
            ))}
          </div>
          <div className={styles.panelTitle} style={{marginTop: '0.8rem'}}>
            The book's stylesheet, in order (edit a selector)
          </div>
          <div className={styles.sheet}>
            {bookRules.map((rule, index) => {
              const dropped = rule.selector.split(',').some((part) => 'dropped' in parseSelector(part));
              return (
                <div key={rule.key} className={styles.ruleRow}>
                  <div className={styles.ruleLine}>
                    <button type="button" className={styles.tiny} disabled={index === 0} onClick={() => move(index, -1)} title="Earlier">
                      ↑
                    </button>
                    <button type="button" className={styles.tiny} disabled={index === bookRules.length - 1} onClick={() => move(index, 1)} title="Later">
                      ↓
                    </button>
                    <input
                      value={rule.selector}
                      className={dropped ? styles.ruleDropped : undefined}
                      aria-label="Selector"
                      onChange={(event) => updateRule(rule.key, (current) => ({...current, selector: event.target.value}))}
                    />
                  </div>
                  <span className={styles.ruleDecl}>{`{ ${declarationsText(rule)} }`}</span>
                </div>
              );
            })}
          </div>
          {result.dropped.filter((entry) => entry.rule.origin === 'book').length > 0 && (
            <ul className={styles.droppedList}>
              {result.dropped
                .filter((entry) => entry.rule.origin === 'book')
                .map((entry) => (
                  <li key={entry.rule.key + entry.selector}>
                    <code>{entry.selector}</code> dropped: {entry.why}.
                  </li>
                ))}
            </ul>
          )}
        </div>
        <div>
          <div className={styles.panelTitle}>Rules that match, the last applied on top</div>
          <div className={styles.cards}>
            {cards.length === 0 && <div className={styles.passLabel}>No rule matches this element.</div>}
            {cards.map(({layer, index}) => {
              const rule = layer.rule!;
              const [ids, classes, types] = layer.selector ? specificityParts(layer.selector.specificity) : [0, 0, 0];
              const winsHere = winner?.layer === index;
              return (
                <div key={`${rule.key}-${layer.selector?.text ?? 'style'}`} className={clsx(styles.ruleCard, winsHere && styles.ruleCardWinner)}>
                  <div className={styles.ruleCardHead}>
                    <span>{layer.selector ? layer.selector.text : 'style="" attribute'}</span>
                    <span className={styles.spec}>
                      {layer.selector ? `(${ids}, ${classes}, ${types})` : 'above every rule'}
                    </span>
                  </div>
                  <div className={styles.origin}>
                    {layer.selector
                      ? `${rule.origin} rule #${rule.origin === 'book' ? layer.order - userAgentRules.length + 1 : layer.order + 1}`
                      : 'on the element itself'}
                  </div>
                  <div>
                    {rule.declarations.map((declared, declarationIndex) => {
                      const focus = declared.property === property;
                      const wins = focus && winsHere && winner.important === declared.important && winner.value === declared.value;
                      return (
                        <button
                          key={declared.property + declarationIndex}
                          type="button"
                          title={rule.origin === 'book' && layer.selector ? 'Click to toggle !important' : undefined}
                          className={clsx(styles.decl, focus && styles.declFocus, wins && styles.declWin, focus && !wins && styles.declLost)}
                          onClick={() => toggleImportant(rule, declarationIndex)}>
                          {declared.property}: {declared.value}
                          {declared.important && <span className={styles.important}> !important</span>}
                        </button>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
          <div className={styles.result}>
            <strong>{property}</strong>:{' '}
            {winner?.important && <span className={styles.important}>(!important, applied in the second pass) </span>}
            {outcome(property, winner?.value)}
          </div>
        </div>
      </div>
    </div>
  );
}
