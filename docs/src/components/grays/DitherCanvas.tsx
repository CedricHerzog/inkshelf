// A whole picture with few grays: plain rounding bands its smooth tones, Floyd–Steinberg
// dithering (the same loop as ditherAndPack) trades the bands for a fine grain. The reader
// always has 16 grays; fewer here make the difference obvious.

import React, {useMemo, useState} from 'react';
import {Controls, Segmented, Slider} from '@site/src/components/kit';
import {quantize} from '@site/src/lib/pageimage';
import {makeShaded} from '@site/src/lib/sampleImages';
import GrayCanvas from './GrayCanvas';
import styles from './grays.module.css';

type Method = 'original' | 'round' | 'dither';

export default function DitherCanvas() {
  const picture = useMemo(() => makeShaded(180, 120), []);
  const [method, setMethod] = useState<Method>('dither');
  const [levels, setLevels] = useState(4);
  const shown = useMemo(
    () => (method === 'original' ? picture : quantize(picture, levels, method === 'dither')),
    [picture, method, levels],
  );
  return (
    <div>
      <div className={styles.row}>
        <GrayCanvas image={shown} displayWidth={360} label={`The picture, ${method}`} />
      </div>
      <p className={styles.note}>
        {method === 'original'
          ? `The original: ${new Set(picture.pixels).size} different grays.`
          : method === 'round'
            ? `Each pixel rounded to the nearest of ${levels} grays: smooth tones become flat bands.`
            : `${levels} grays, dithered: the bands are gone, and from afar the average gray is right.`}
      </p>
      <Controls>
        <Segmented
          options={[
            {value: 'original', label: 'Original'},
            {value: 'round', label: 'Round'},
            {value: 'dither', label: 'Dither'},
          ]}
          value={method}
          onChange={setMethod}
        />
        <Slider label="Grays" value={levels} min={2} max={16} onChange={setLevels} format={(value) => (value === 16 ? '16 (the panel)' : String(value))} />
      </Controls>
    </div>
  );
}
