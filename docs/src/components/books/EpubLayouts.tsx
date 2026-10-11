// The same page as a reflowable chapter and as a pre-paginated one, on screens of several
// shapes and at several text sizes. The reflowable text is laid out again and gains pages; the
// fixed page keeps its design and is only scaled to fit, as fixedlayout's Page.Render does
// (the page enlarged by min(width ratio, height ratio), its aspect ratio kept).

import React, {useEffect, useRef, useState} from 'react';
import {Controls, Segmented, Slider} from '@site/src/components/kit';
import {defaultTextSize, textSizes} from '@site/src/lib/cssLength';
import styles from './Epub.module.css';

type ScreenId = 'inkpad' | 'small' | 'wide';

const screens: Record<ScreenId, {label: string; width: number; height: number}> = {
  inkpad: {label: 'InkPad 3, 1404×1872', width: 1404, height: 1872},
  small: {label: 'a 6″ reader, 1072×1448', width: 1072, height: 1448},
  wide: {label: 'a landscape tablet, 1872×1404', width: 1872, height: 1404},
};

// The drawing is not to scale: a screen's long side is drawn 250 px long.
const drawnLongSide = 250;

const story = [
  'The night was dark, and the rain had not stopped since noon. Nobody in the village went out.',
  'Only the old ferryman sat by the river, waiting for a passenger who had promised to come.',
  'At midnight a lantern appeared on the far bank. It swung slowly, as if the hand that held it were tired.',
];

function ReflowPanel({width, height, textSize}: {width: number; height: number; textSize: number}) {
  const panel = useRef<HTMLDivElement>(null);
  const text = useRef<HTMLDivElement>(null);
  const [pages, setPages] = useState(1);
  useEffect(() => {
    if (!panel.current || !text.current) return;
    setPages(Math.max(1, Math.ceil(text.current.scrollHeight / panel.current.clientHeight)));
  }, [width, height, textSize]);
  return (
    <div ref={panel} className={styles.panel}>
      <div ref={text} className={styles.reflowText} style={{fontSize: textSize / 4.2}}>
        <h4>Chapter 1</h4>
        {story.map((paragraph) => (
          <p key={paragraph}>{paragraph}</p>
        ))}
      </div>
      <span className={styles.pageCount}>page 1 of {pages}</span>
    </div>
  );
}

// A page designed at 600×800 CSS pixels: a picture, a speech bubble, a caption.
function FixedPage() {
  return (
    <svg viewBox="0 0 600 800" width="100%" height="100%" preserveAspectRatio="xMidYMid meet" role="img"
      aria-label="A fixed-layout page, scaled to fit">
      <rect x={0} y={0} width={600} height={800} fill="#fff" />
      <rect x={40} y={40} width={520} height={460} fill="#cfcac0" />
      <circle cx={430} cy={160} r={50} fill="#f5f2ea" />
      <path d="M40 500 L220 280 L330 420 L420 330 L560 500 Z" fill="#7d786e" />
      <rect x={70} y={90} width={230} height={90} rx={40} fill="#fff" stroke="#1d1d1b" strokeWidth={4} />
      <path d="M150 178 L130 225 L190 178" fill="#fff" stroke="#1d1d1b" strokeWidth={4} />
      <text x={185} y={145} textAnchor="middle" fontSize={34} fontFamily="Georgia, serif" fill="#1d1d1b">
        Who's there?
      </text>
      <text x={300} y={590} textAnchor="middle" fontSize={40} fontFamily="Georgia, serif" fill="#1d1d1b">
        Chapter 1
      </text>
      <text x={300} y={650} textAnchor="middle" fontSize={26} fontFamily="Georgia, serif" fill="#1d1d1b">
        The night was dark, and the rain
      </text>
      <text x={300} y={690} textAnchor="middle" fontSize={26} fontFamily="Georgia, serif" fill="#1d1d1b">
        had not stopped since noon.
      </text>
    </svg>
  );
}

export default function EpubLayouts() {
  const [screenId, setScreenId] = useState<ScreenId>('inkpad');
  const [sizeIndex, setSizeIndex] = useState<number>(textSizes.indexOf(defaultTextSize));
  const screen = screens[screenId];
  const scale = drawnLongSide / Math.max(screen.width, screen.height);
  const width = Math.round(screen.width * scale);
  const height = Math.round(screen.height * scale);
  const textSize = textSizes[sizeIndex];
  const fit = Math.min(screen.width / 600, screen.height / 800);

  return (
    <div>
      <Controls>
        <Segmented
          options={(Object.keys(screens) as ScreenId[]).map((id) => ({value: id, label: screens[id].label}))}
          value={screenId}
          onChange={setScreenId}
        />
      </Controls>
      <Controls>
        <Slider
          label="Text size"
          value={sizeIndex}
          min={0}
          max={textSizes.length - 1}
          onChange={setSizeIndex}
          format={(index) => `${textSizes[index]} px`}
        />
      </Controls>
      <div className={styles.layouts}>
        <div className={styles.screenColumn}>
          <div className={styles.screenTitle}>Reflowable</div>
          <div className={styles.device} style={{width: width + 20, height: height + 32}}>
            <ReflowPanel width={width} height={height} textSize={textSize} />
          </div>
          <div className={styles.screenNote}>
            Laid out again for this screen and this size: the lines break elsewhere, the pages
            are counted again.
          </div>
        </div>
        <div className={styles.screenColumn}>
          <div className={styles.screenTitle}>Fixed layout (pre-paginated)</div>
          <div className={styles.device} style={{width: width + 20, height: height + 32}}>
            <div className={styles.panel}>
              <FixedPage />
            </div>
          </div>
          <div className={styles.screenNote}>
            Designed at 600×800, scaled ×{fit.toFixed(2)} to fit. The text size does nothing:
            the page's text is part of its picture.
          </div>
        </div>
      </div>
    </div>
  );
}
