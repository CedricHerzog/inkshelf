// One colored page through the four stages of pageimage.Prepare, computed live with the
// TypeScript ports of the Go code: decode to gray, fit to the screen, tone, dither to 16
// grays and pack. Click a stage to see the page as it leaves it.

import React, {useMemo, useState} from 'react';
import {
  applyTones,
  ditherAndPack,
  fit,
  luma,
  newGray,
  toneTable,
  unpack,
  type GrayImage,
} from '@site/src/lib/pageimage';
import {makeColorPage} from '@site/src/lib/sampleImages';
import GrayCanvas from './GrayCanvas';
import styles from './grays.module.css';

/** A pretend screen, the InkPad 3's 1404×1872 divided by 10. */
const screenWidth = 140;
const screenHeight = 187;

type Stage = 'server' | 'decode' | 'fit' | 'tone' | 'dither';

function distinctGrays(image: GrayImage): number {
  return new Set(image.pixels).size;
}

export default function PipelineOverview() {
  const [stage, setStage] = useState<Stage>('server');
  const results = useMemo(() => {
    const color = makeColorPage(300, 420);
    const gray = newGray(color.width, color.height);
    for (let index = 0; index < gray.pixels.length; index++) {
      gray.pixels[index] = luma(color.rgb[index * 3], color.rgb[index * 3 + 1], color.rgb[index * 3 + 2]);
    }
    const fitted = fit(gray, screenWidth, screenHeight);
    const tone = toneTable(fitted.histogram);
    const toned = applyTones(fitted.image, tone.tones);
    const pg4 = ditherAndPack(fitted.image, tone.tones);
    return {color, gray, fitted: fitted.image, tone, toned, pg4, dithered: unpack(pg4)};
  }, []);

  const stages: {value: Stage; label: string; image?: GrayImage; text: React.ReactNode}[] = [
    {
      value: 'server',
      label: '0 · from the server',
      text: (
        <>
          A {results.color.width}×{results.color.height} color page, as Komga sends it (a JPEG,
          PNG, WebP or JPEG XL file).
        </>
      ),
    },
    {
      value: 'decode',
      label: '1 · DecodeGray',
      image: results.gray,
      text: (
        <>
          Decoded straight to gray: 0.299·R + 0.587·G + 0.114·B. Same size, one byte a pixel,{' '}
          {distinctGrays(results.gray)} different grays.
        </>
      ),
    },
    {
      value: 'fit',
      label: '2 · fit',
      image: results.fitted,
      text: (
        <>
          Shrunk to fit a {screenWidth}×{screenHeight} screen without stretching:{' '}
          {results.fitted.width}×{results.fitted.height}, shown at 2× so you see its pixels. Each one averages a box of the source.
        </>
      ),
    },
    {
      value: 'tone',
      label: '3 · toneTable',
      image: results.toned,
      text: (
        <>
          Gray {results.tone.darkest} and darker become black, gray {results.tone.lightest} and
          lighter white, and the mid grays are darkened (gamma 1.8).
        </>
      ),
    },
    {
      value: 'dither',
      label: '4 · ditherAndPack',
      image: results.dithered,
      text: (
        <>
          {distinctGrays(results.dithered)} grays left, spread by dithering, packed two pixels a byte:
          a .pg4 file of {results.pg4.length.toLocaleString('en')} bytes, against{' '}
          {(results.fitted.width * results.fitted.height).toLocaleString('en')} for one byte a pixel.
        </>
      ),
    },
  ];
  const shown = stages.find((candidate) => candidate.value === stage) ?? stages[0];

  return (
    <div>
      <div className={styles.stages} role="group" aria-label="Stages">
        {stages.map((candidate) => (
          <button
            key={candidate.value}
            type="button"
            aria-pressed={candidate.value === stage}
            className={`${styles.stageButton} ${candidate.value === stage ? styles.stageButtonActive : ''}`}
            onClick={() => setStage(candidate.value)}>
            {candidate.label}
          </button>
        ))}
      </div>
      <div className={styles.row}>
        {shown.image ? (
          <GrayCanvas
            image={shown.image}
            displayWidth={shown.value === 'decode' ? 220 : shown.image.width * 2}
            label={shown.label}
          />
        ) : (
          <GrayCanvas color={results.color} displayWidth={220} label="The page from the server, in color" />
        )}
      </div>
      <p className={styles.note}>{shown.text}</p>
    </div>
  );
}
