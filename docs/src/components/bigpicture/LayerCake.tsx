// The layer cake: Inkshelf's domains stacked bottom to top. Explore shows what each layer holds
// and may import; "Try an import" checks one import against the layering rules of
// docs/reference/architecture.md (src/lib/layers.ts) and draws it as a green or a red arrow.

import React, {useState} from 'react';
import clsx from 'clsx';
import {Button, Controls, Segmented, usePrefersReducedMotion} from '@site/src/components/kit';
import {
  checkImport,
  domainById,
  domainImports,
  domainOf,
  domains,
  stack,
  type DomainId,
} from '@site/src/lib/layers';
import styles from './LayerCake.module.css';

const rowHeight = 30;
const rowGap = 5;
const top = 8;
const barLeft = 70;
const barRight = 318;
const pillarLeft = 332;
const pillarRight = 388;
const width = 400;
const height = top + stack.length * (rowHeight + rowGap) + 4;

/** The y of the middle of a layer's row (the cake is drawn bottom up). */
function rowMiddle(id: DomainId): number {
  const index = stack.findIndex((domain) => domain.id === id);
  const fromTop = stack.length - 1 - index;
  return top + fromTop * (rowHeight + rowGap) + rowHeight / 2;
}

// The pillar of the utilities (naturalorder, timing) stands beside every layer: any of them may
// use it.
const pillarTop = rowMiddle('cmd') - rowHeight / 2;
const pillarBottom = rowMiddle('device') + rowHeight / 2;

const presets: readonly {from: string; to: string; label: string}[] = [
  {from: 'shelf', to: 'ebook/epub', label: 'shelf → ebook/epub'},
  {from: 'app/screens/booklist', to: 'app/screens/list', label: 'booklist → list'},
  {from: 'app/openbook', to: 'servers/komga', label: 'app → servers/komga'},
  {from: 'app/screens/booklist', to: 'app/screens/reader', label: 'a screen → another'},
  {from: 'graphics/fonts', to: 'app/widgets', label: 'graphics → app'},
  {from: 'web/css', to: 'servers', label: 'web → servers'},
  {from: 'cmd/inkshelf', to: 'servers/registry', label: 'cmd → registry'},
  {from: 'typesetting/document', to: 'typesetting/layout', label: 'document → layout'},
];

/** An arrow between two layers, curving out on the left (or to the pillar on the right). */
function arrowPath(from: DomainId, to: DomainId): string {
  if (from === 'utilities' || to === 'utilities') {
    const other = from === 'utilities' ? to : from;
    const y = Math.min(Math.max(rowMiddle(other), pillarTop + 8), pillarBottom - 8);
    return from === 'utilities'
      ? `M ${pillarLeft} ${y + 4} L ${barRight + 2} ${y + 4}`
      : `M ${barRight} ${y} L ${pillarLeft - 2} ${y}`;
  }
  const y1 = rowMiddle(from);
  const y2 = rowMiddle(to);
  if (from === to) {
    return `M ${barLeft} ${y1 - 6} C ${barLeft - 46} ${y1 - 26}, ${barLeft - 46} ${y1 + 26}, ${barLeft} ${y1 + 6}`;
  }
  const bulge = barLeft - 22 - Math.min(40, Math.abs(y2 - y1) / 6);
  return `M ${barLeft} ${y1} C ${bulge} ${y1}, ${bulge} ${y2}, ${barLeft} ${y2}`;
}

function PackageSelect({value, onChange, label}: {value: string; onChange: (pkg: string) => void; label: string}) {
  return (
    <label className={styles.select}>
      <span>{label}</span>
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {[...domains].reverse().map((domain) => (
          <optgroup key={domain.id} label={domain.id}>
            {domain.packages.map((pkg) => (
              <option key={pkg} value={pkg}>
                {pkg}
              </option>
            ))}
          </optgroup>
        ))}
      </select>
    </label>
  );
}

export default function LayerCake() {
  const [mode, setMode] = useState<'explore' | 'try'>('explore');
  const [selected, setSelected] = useState<DomainId>('shelf');
  const [from, setFrom] = useState('app/openbook');
  const [to, setTo] = useState('servers/komga');
  const [attempt, setAttempt] = useState(0);
  const reducedMotion = usePrefersReducedMotion();

  const verdict = checkImport(from, to);
  const fromDomain = domainOf(from).id;
  const toDomain = domainOf(to).id;
  const selectedDomain = domainById(selected);
  const usedToday = domainImports(selected);
  const selectedRank = selectedDomain.rank;

  const pick = (pkgFrom: string, pkgTo: string) => {
    setFrom(pkgFrom);
    setTo(pkgTo);
    setAttempt((count) => count + 1);
  };

  const rowClass = (id: DomainId) => {
    const domain = domainById(id);
    if (mode === 'try') {
      return clsx(
        styles.row,
        id === fromDomain && styles.rowFrom,
        id === toDomain && styles.rowTo,
      );
    }
    return clsx(
      styles.row,
      id === selected && styles.rowSelected,
      id !== selected && (domain.rank < selectedRank || id === 'utilities') && styles.rowAllowed,
      domain.rank > selectedRank && styles.rowAbove,
    );
  };

  return (
    <div className={styles.widget}>
      <Controls>
        <Segmented
          options={[
            {value: 'explore', label: 'Explore the layers'},
            {value: 'try', label: 'Try an import'},
          ]}
          value={mode}
          onChange={setMode}
        />
      </Controls>

      <div className={styles.stage}>
        <svg
          viewBox={`0 0 ${width} ${height}`}
          className={styles.cake}
          role="img"
          aria-label="Inkshelf's layers, from device at the bottom to cmd at the top">
          <defs>
            <marker id="cake-arrow-ok" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto">
              <path d="M0,0 L10,5 L0,10 z" fill="var(--ink-ok)" />
            </marker>
            <marker id="cake-arrow-bad" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto">
              <path d="M0,0 L10,5 L0,10 z" fill="var(--ink-accent)" />
            </marker>
          </defs>

          {stack.map((domain) => {
            const y = rowMiddle(domain.id) - rowHeight / 2;
            const uses = mode === 'explore' && usedToday.includes(domain.id);
            return (
              <g
                key={domain.id}
                className={rowClass(domain.id)}
                onClick={() => {
                  if (mode === 'explore') setSelected(domain.id);
                }}>
                <rect x={barLeft} y={y} width={barRight - barLeft} height={rowHeight} rx={6} />
                <text x={barLeft + 12} y={y + rowHeight / 2} dominantBaseline="central" className={styles.rowName}>
                  {domain.id}
                </text>
                <text
                  x={barRight - 10}
                  y={y + rowHeight / 2}
                  dominantBaseline="central"
                  textAnchor="end"
                  className={styles.rowCount}>
                  {uses ? '● used today · ' : ''}
                  {domain.packages.length} pkg
                </text>
              </g>
            );
          })}

          <g
            className={rowClass('utilities')}
            onClick={() => {
              if (mode === 'explore') setSelected('utilities');
            }}>
            <rect x={pillarLeft} y={pillarTop} width={pillarRight - pillarLeft} height={pillarBottom - pillarTop} rx={6} />
            <text
              x={(pillarLeft + pillarRight) / 2}
              y={(pillarTop + pillarBottom) / 2}
              textAnchor="middle"
              dominantBaseline="central"
              transform={`rotate(-90 ${(pillarLeft + pillarRight) / 2} ${(pillarTop + pillarBottom) / 2})`}
              className={styles.rowName}>
              utilities
            </text>
          </g>

          <text x={6} y={rowMiddle('cmd')} dominantBaseline="central" className={styles.axis}>
            top
          </text>
          <text x={6} y={rowMiddle('device')} dominantBaseline="central" className={styles.axis}>
            bottom
          </text>

          {mode === 'try' && (
            <path
              key={`${from}>${to}>${attempt}`}
              d={arrowPath(fromDomain, toDomain)}
              fill="none"
              strokeWidth={3}
              strokeLinecap="round"
              markerEnd={verdict.allowed ? 'url(#cake-arrow-ok)' : 'url(#cake-arrow-bad)'}
              className={clsx(
                styles.arrow,
                verdict.allowed ? styles.arrowOk : styles.arrowBad,
                !reducedMotion && styles.arrowAnimated,
              )}
            />
          )}
        </svg>

        <div className={styles.panel}>
          {mode === 'explore' ? (
            <>
              <div className={styles.panelTitle}>{selectedDomain.id}</div>
              <p className={styles.knows}>{selectedDomain.knows}</p>
              <div className={styles.chips}>
                {selectedDomain.packages.map((pkg) => (
                  <code key={pkg} className={styles.chip}>
                    {pkg}
                  </code>
                ))}
              </div>
              <p className={styles.small}>
                <strong>May import:</strong>{' '}
                {selectedDomain.id === 'utilities'
                  ? 'nothing (it needs nothing).'
                  : [
                      ...stack.filter((domain) => domain.rank < selectedRank).map((domain) => domain.id),
                      'utilities',
                    ].join(', ') + ', and its own packages.'}
              </p>
              <p className={styles.small}>
                <strong>Imports today:</strong> {usedToday.length > 0 ? usedToday.join(', ') : 'nothing outside itself'}.
              </p>
              {selected === 'app' && (
                <p className={styles.small}>
                  Inside, the app has its own little cake: <code>widgets</code>, then <code>state</code>, then the
                  services (<code>session</code>, <code>catalog</code>, <code>downloadqueue</code>, <code>covers</code>,{' '}
                  <code>openbook</code>), then the screens. Screens never import each other;{' '}
                  <code>screens/list</code> is not a screen but the header, rows and footer the list screens share.
                </p>
              )}
              {selected === 'servers' && (
                <p className={styles.small}>
                  Only <code>cmd</code> imports <code>servers/registry</code>, which knows every kind. Everything else
                  talks to the <code>servers.Server</code> interface.
                </p>
              )}
            </>
          ) : (
            <>
              <div className={styles.pickers}>
                <PackageSelect label="from" value={from} onChange={(pkg) => pick(pkg, to)} />
                <PackageSelect label="imports" value={to} onChange={(pkg) => pick(from, pkg)} />
              </div>
              <div
                key={`${from}>${to}>${attempt}`}
                className={clsx(styles.verdict, verdict.allowed ? styles.verdictOk : styles.verdictBad)}
                aria-live="polite">
                <strong>{verdict.allowed ? '✓ Allowed' : '✗ Forbidden'}</strong>
                <span>{verdict.reason}</span>
                <span className={styles.real}>
                  {verdict.real ? 'This import exists in the code today.' : 'The code has no such import today.'}
                </span>
              </div>
              <div className={styles.presets}>
                {presets.map((preset) => (
                  <Button key={preset.label} onClick={() => pick(preset.from, preset.to)} active={preset.from === from && preset.to === to}>
                    {preset.label}
                  </Button>
                ))}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
