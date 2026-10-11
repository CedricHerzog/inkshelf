// The spine, the manifest and what the reader makes of them, on the package document of
// Inkshelf's test book. Each itemref points to a manifest card; the linear ones become the
// chapters that "Next page" walks through; a fallback chain is followed hop by hop.

import React, {useMemo, useState} from 'react';
import clsx from 'clsx';
import {
  Controls,
  Narration,
  Segmented,
  StepperBar,
  Toggle,
  useStepper,
} from '@site/src/components/kit';
import {readSpine, type ItemRef, type ManifestItem} from '@site/src/lib/epubSpine';
import {fallbackChain, fallbackLoop, sampleManifest, sampleSpine} from '@site/src/lib/epubSample';
import styles from './Epub.module.css';

type Fallback = 'none' | 'chain' | 'loop';

const cardHeight = 34;
const gap = 7;
const top = 34;
const spineX = 4;
const spineWidth = 128;
const manifestX = 176;
const manifestWidth = 196;
const readerX = 398;
const readerWidth = 130;
const width = readerX + readerWidth + 4;

function cardY(index: number) {
  return top + index * (cardHeight + gap);
}

function shortType(mediaType: string) {
  return mediaType.replace('application/', '').replace('+xml', '');
}

export default function EpubSpine() {
  const [notesLinear, setNotesLinear] = useState(false);
  const [fallback, setFallback] = useState<Fallback>('none');

  const manifest: ManifestItem[] = useMemo(
    () => [
      ...sampleManifest,
      ...(fallback === 'chain' ? fallbackChain : fallback === 'loop' ? fallbackLoop : []),
    ],
    [fallback],
  );
  const spine: ItemRef[] = useMemo(
    () =>
      sampleSpine.map((itemRef) => {
        if (itemRef.idref === 'notes') return notesLinear ? {idref: 'notes'} : itemRef;
        if (itemRef.idref === 'c2' && fallback !== 'none') {
          return {idref: fallback === 'chain' ? 'pdf' : 'loop1'};
        }
        return itemRef;
      }),
    [notesLinear, fallback],
  );
  const read = useMemo(() => readSpine(manifest, spine), [manifest, spine]);
  const stepper = useStepper(read.chapters.length, 1100);

  const manifestIndex = (id: string) => manifest.findIndex((item) => item.id === id);
  const reading = read.chapters[stepper.index];
  const hops = read.spine.find((item) => item.idref === spine[spine.length - 1].idref)?.hops ?? [];
  const hopIds = new Set(fallback === 'none' ? [] : hops.map((hop) => hop.id));
  const nonLinear = read.spine.filter((item) => !item.linear);
  const chapterTop = (index: number) => top + index * (cardHeight + 22);
  const height = Math.max(cardY(manifest.length), chapterTop(read.chapters.length) + 70) + 10;

  const shownName = (content: string) => (content === '' ? '“cannot be shown”' : content.split('/').pop());

  return (
    <div>
      <Controls>
        <Toggle
          label={<>notes is <code>linear="no"</code></>}
          checked={!notesLinear}
          onChange={(checked) => setNotesLinear(!checked)}
        />
        <span style={{fontSize: '0.85rem'}}>The last itemref points to:</span>
        <Segmented
          options={[
            {value: 'none', label: 'c2.xhtml'},
            {value: 'chain', label: 'a PDF with fallbacks'},
            {value: 'loop', label: 'a fallback loop'},
          ]}
          value={fallback}
          onChange={setFallback}
        />
      </Controls>
      <div className={styles.spineStage}>
        <svg className={styles.spineSvg} width={width} height={height} viewBox={`0 0 ${width} ${height}`} role="img"
          aria-label="Spine itemrefs pointing to manifest items, and the chapters they become">
          <defs>
            <marker id="epub-spine-head" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="7" markerHeight="7" orient="auto">
              <path d="M0,0 L8,4 L0,8 z" fill="var(--ink-blue)" />
            </marker>
            <marker id="epub-spine-hop" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="7" markerHeight="7" orient="auto">
              <path d="M0,0 L8,4 L0,8 z" fill="var(--ink-warn)" />
            </marker>
          </defs>
          <text className={styles.columnTitle} x={spineX} y={18}>Spine (order)</text>
          <text className={styles.columnTitle} x={manifestX} y={18}>Manifest (every file)</text>
          <text className={styles.columnTitle} x={readerX} y={18}>The reader</text>

          {/* wires from itemrefs to manifest items */}
          {spine.map((itemRef, index) => {
            const to = manifestIndex(itemRef.idref);
            const y1 = cardY(index) + cardHeight / 2;
            const y2 = cardY(to) + cardHeight / 2;
            const x1 = spineX + spineWidth;
            const x2 = manifestX - 2;
            return (
              <path
                key={`wire-${index}`}
                className={clsx(styles.wire, itemRef.linear === 'no' && styles.wireWeak)}
                d={`M ${x1} ${y1} C ${x1 + 35} ${y1}, ${x2 - 35} ${y2}, ${x2} ${y2}`}
                markerEnd="url(#epub-spine-head)"
              />
            );
          })}

          {/* the fallback chain's hops, on the manifest's right side */}
          {fallback !== 'none' &&
            hops.map((hop, index) => {
              const next = hops[index + 1]?.id ?? manifest.find((item) => item.id === hop.id)?.fallback;
              if (hop.supported || next === undefined) return null;
              const y1 = cardY(manifestIndex(hop.id)) + cardHeight / 2;
              const y2 = cardY(manifestIndex(next)) + cardHeight / 2;
              const x = manifestX + manifestWidth;
              const bulge = x + 26 + Math.abs(y2 - y1) * 0.15;
              return (
                <path
                  key={`hop-${hop.id}`}
                  className={clsx(styles.wire, styles.wireHop)}
                  d={`M ${x} ${y1} C ${bulge} ${y1}, ${bulge} ${y2}, ${x + 2} ${y2}`}
                  markerEnd="url(#epub-spine-hop)"
                />
              );
            })}

          {/* spine cards */}
          {spine.map((itemRef, index) => (
            <g key={`ref-${index}`} className={styles.card}>
              <rect className={styles.cardBox} x={spineX} y={cardY(index)} width={spineWidth} height={cardHeight} rx={6} />
              <text className={styles.label} x={spineX + 8} y={cardY(index) + 14}>
                idref="{itemRef.idref}"
              </text>
              <text className={styles.sublabel} x={spineX + 8} y={cardY(index) + 27}>
                {itemRef.linear === 'no' ? 'linear="no"' : 'linear (the default)'}
              </text>
            </g>
          ))}

          {/* manifest cards */}
          {manifest.map((item, index) => {
            const inSpine = spine.some((itemRef) => itemRef.idref === item.id);
            const hop = hops.find((candidate) => candidate.id === item.id);
            return (
              <g
                key={item.id}
                className={clsx(
                  styles.card,
                  !inSpine && !hopIds.has(item.id) && styles.cardDim,
                  reading && reading.hops[reading.hops.length - 1]?.id === item.id && styles.cardHot,
                  hopIds.has(item.id) && hop?.supported && styles.cardShown,
                  hopIds.has(item.id) && hop && !hop.supported && styles.cardRefused,
                )}>
                <rect className={styles.cardBox} x={manifestX} y={cardY(index)} width={manifestWidth} height={cardHeight} rx={6} />
                <text className={styles.label} x={manifestX + 8} y={cardY(index) + 14}>
                  {item.id} · {item.href}
                </text>
                <text className={styles.sublabel} x={manifestX + 8} y={cardY(index) + 27}>
                  {shortType(item.mediaType)}
                  {item.properties ? ` · ${item.properties}` : ''}
                  {item.fallback ? ` · fallback="${item.fallback}"` : ''}
                </text>
                {hopIds.has(item.id) && hop && (
                  <text x={manifestX + manifestWidth - 14} y={cardY(index) + 21} fontSize={13}
                    fill={hop.supported ? 'var(--ink-ok)' : 'var(--ink-accent)'}>
                    {hop.supported ? '✓' : '✗'}
                  </text>
                )}
              </g>
            );
          })}

          {/* chapters, and "Next page" between them */}
          {read.chapters.map((chapter, index) => {
            const y = chapterTop(index);
            const here = index === stepper.index;
            return (
              <g key={`chapter-${index}`} className={clsx(styles.card, here && styles.cardHot)}>
                <rect className={styles.cardBox} x={readerX} y={y} width={readerWidth} height={cardHeight} rx={6} />
                <text className={styles.label} x={readerX + 26} y={y + 14}>
                  Chapter {index}
                </text>
                <text className={styles.sublabel} x={readerX + 26} y={y + 27}>
                  {shownName(chapter.content)}
                </text>
                {index > 0 && (
                  <text className={styles.sublabel} x={readerX + 26} y={y - 6}>
                    ↓ Next page
                  </text>
                )}
              </g>
            );
          })}
          <circle
            className={styles.marker}
            cx={0}
            cy={0}
            r={7}
            style={{transform: `translate(${readerX + 13}px, ${chapterTop(stepper.index) + cardHeight / 2}px)`}}
          />
          {nonLinear.map((item, index) => {
            const y = chapterTop(read.chapters.length) + 6 + index * (cardHeight + 6);
            return (
              <g key={`popup-${item.idref}`}>
                <rect className={styles.popup} x={readerX} y={y} width={readerWidth} height={cardHeight + 10} rx={6} strokeDasharray="4 3" />
                <text className={styles.label} x={readerX + 8} y={y + 15}>
                  {item.content.split('/').pop()}
                </text>
                <text className={styles.sublabel} x={readerX + 8} y={y + 29}>
                  only a link opens it,
                </text>
                <text className={styles.sublabel} x={readerX + 8} y={y + 40}>
                  in a popup
                </text>
              </g>
            );
          })}
        </svg>
      </div>
      <StepperBar stepper={stepper} label="chapter" />
      <Narration>
        {reading && (
          <>
            Chapter {stepper.index} (counted from 0, {read.chapters.length} in all): itemref{' '}
            <code>{reading.idref}</code>
            {reading.hops.length > 1 && (
              <>
                , followed through {reading.hops.map((hop) => hop.id).join(' → ')}
              </>
            )}
            {reading.content === ''
              ? '. No item of the chain can be shown, so the chapter is one line: “This part of the book cannot be shown.”'
              : `, shows ${reading.content}.`}
            {stepper.atEnd && ' The end of the book: there is no next chapter.'}
          </>
        )}
      </Narration>
    </div>
  );
}
