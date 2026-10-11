// toneTable on a faded scan: its histogram, the two clip points found from the share of
// pixels clipped at each end, the curve (stretch, then gamma) and the page before and after.
// The reader always uses 0.5% and gamma 1.8 (tone.go); here both can move.

import React, {useMemo, useState} from 'react';
import {Button, Controls, Slider} from '@site/src/components/kit';
import {
  applyTones,
  clippedShare,
  gamma as inkshelfGamma,
  histogramOf,
  toneTable,
} from '@site/src/lib/pageimage';
import {makeScan} from '@site/src/lib/sampleImages';
import GrayCanvas from './GrayCanvas';
import styles from './grays.module.css';

const plotSize = 150;
const margin = 22;

export default function ToneCurve() {
  const scan = useMemo(() => makeScan(240, 180), []);
  const histogram = useMemo(() => histogramOf(scan), [scan]);
  const [sharePercent, setSharePercent] = useState(clippedShare * 100);
  const [curveGamma, setCurveGamma] = useState(inkshelfGamma);

  const tone = useMemo(() => toneTable(histogram, sharePercent / 100, curveGamma), [histogram, sharePercent, curveGamma]);
  const after = useMemo(() => applyTones(scan, tone.tones), [scan, tone]);
  const afterHistogram = useMemo(() => histogramOf(after), [after]);

  const toX = (level: number) => margin + (level / 255) * plotSize;
  const toY = (level: number) => margin + plotSize - (level / 255) * plotSize;

  const histogramPath = (counts: number[]) => {
    // Square roots, so the few dark ink pixels still show next to the many paper ones.
    const peak = Math.sqrt(Math.max(...counts));
    return counts
      .map((count, level) => `M${toX(level) + 0.3},${margin + plotSize} v${-(Math.sqrt(count) / peak) * plotSize * 0.9}`)
      .join(' ');
  };

  const curve = Array.from(tone.tones)
    .map((output, level) => `${level === 0 ? 'M' : 'L'}${toX(level)},${toY(output)}`)
    .join(' ');

  const isInkshelf = Math.abs(sharePercent - clippedShare * 100) < 1e-9 && curveGamma === inkshelfGamma;

  return (
    <div>
      <div className={styles.row}>
        <GrayCanvas image={scan} displayWidth={240} label="A faded scan, before" caption="before: gray paper, faded ink" />
        <GrayCanvas image={after} displayWidth={240} label="The same scan through the tone table" caption="after toneTable (not dithered yet)" />
      </div>
      <div className={styles.row} style={{marginTop: '0.8rem'}}>
        <svg className={styles.svg} width={plotSize + margin * 2} viewBox={`0 0 ${plotSize + margin * 2} ${plotSize + margin * 2}`} role="img" aria-label="Histogram of the scan with its clip points">
          <rect x={margin} y={margin} width={toX(tone.darkest) - margin} height={plotSize} fill="#000" opacity={0.12} />
          <rect x={toX(tone.lightest)} y={margin} width={margin + plotSize - toX(tone.lightest)} height={plotSize} fill="var(--ink-blue)" opacity={0.12} />
          <path d={histogramPath(histogram)} stroke="var(--ink-muted)" strokeWidth={0.7} />
          <path d={histogramPath(afterHistogram)} stroke="var(--ink-accent)" strokeWidth={0.7} opacity={0.55} />
          <line x1={toX(tone.darkest)} x2={toX(tone.darkest)} y1={margin - 4} y2={margin + plotSize} stroke="var(--ink-text)" strokeWidth={1.5} />
          <line x1={toX(tone.lightest)} x2={toX(tone.lightest)} y1={margin - 4} y2={margin + plotSize} stroke="var(--ink-blue)" strokeWidth={1.5} />
          <text x={toX(tone.darkest)} y={margin - 7} textAnchor="middle" fontSize={9} fontFamily="var(--ink-mono)" fill="var(--ink-text)">{tone.darkest}</text>
          <text x={toX(tone.lightest)} y={margin - 7} textAnchor="middle" fontSize={9} fontFamily="var(--ink-mono)" fill="var(--ink-blue)">{tone.lightest}</text>
          <line x1={margin} x2={margin + plotSize} y1={margin + plotSize} y2={margin + plotSize} stroke="var(--ink-line)" />
          <text x={margin} y={plotSize + margin + 13} fontSize={9} fill="var(--ink-muted)">0</text>
          <text x={margin + plotSize} y={plotSize + margin + 13} textAnchor="end" fontSize={9} fill="var(--ink-muted)">255</text>
          <text x={margin + plotSize / 2} y={plotSize + margin + 13} textAnchor="middle" fontSize={9} fill="var(--ink-muted)">
            histogram: <tspan fill="var(--ink-muted)">before</tspan> · <tspan fill="var(--ink-accent)">after</tspan>
          </text>
        </svg>
        <svg className={styles.svg} width={plotSize + margin * 2} viewBox={`0 0 ${plotSize + margin * 2} ${plotSize + margin * 2}`} role="img" aria-label="The tone curve">
          <rect x={margin} y={margin} width={plotSize} height={plotSize} fill="none" stroke="var(--ink-line)" />
          <line x1={toX(0)} y1={toY(0)} x2={toX(255)} y2={toY(255)} stroke="var(--ink-line)" strokeDasharray="3 3" />
          <path d={curve} fill="none" stroke="var(--ink-accent)" strokeWidth={2} />
          <line x1={toX(tone.darkest)} x2={toX(tone.darkest)} y1={margin} y2={margin + plotSize} stroke="var(--ink-text)" strokeWidth={0.8} strokeDasharray="2 2" />
          <line x1={toX(tone.lightest)} x2={toX(tone.lightest)} y1={margin} y2={margin + plotSize} stroke="var(--ink-blue)" strokeWidth={0.8} strokeDasharray="2 2" />
          <text x={margin + plotSize / 2} y={plotSize + margin + 13} textAnchor="middle" fontSize={9} fill="var(--ink-muted)">input gray →</text>
          <text x={8} y={margin + plotSize / 2} textAnchor="middle" fontSize={9} fill="var(--ink-muted)" transform={`rotate(-90 8 ${margin + plotSize / 2})`}>
            output gray →
          </text>
          <text x={margin + 4} y={margin + 11} fontSize={9} fontFamily="var(--ink-mono)" fill="var(--ink-muted)">
            mid ({Math.round((tone.darkest + tone.lightest) / 2)}) → {tone.tones[Math.round((tone.darkest + tone.lightest) / 2)]}
          </text>
        </svg>
      </div>
      <p className={styles.formula}>
        out = 255 × ((in − {tone.darkest}) ÷ ({tone.lightest} − {tone.darkest}))<sup>{curveGamma.toFixed(1)}</sup>, kept between 0 and 255
      </p>
      <Controls>
        <Slider label="Clip each end" value={sharePercent} min={0} max={5} step={0.1} onChange={setSharePercent} format={(value) => `${value.toFixed(1)}%`} />
        <Slider label="Gamma" value={curveGamma} min={1} max={3} step={0.1} onChange={setCurveGamma} format={(value) => value.toFixed(1)} />
        <Button onClick={() => { setSharePercent(clippedShare * 100); setCurveGamma(inkshelfGamma); }} disabled={isInkshelf}>
          Inkshelf's values
        </Button>
      </Controls>
    </div>
  );
}
