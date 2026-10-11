// A letter "o" rasterized into an alpha mask, then blended into the page as fonts.blend does.
// The offset slider shows why Inkshelf hints fully: an outline that lands between two pixels
// gives a different, blurrier mask; snapped to whole pixels, one mask serves every "o".

import React, {useMemo, useState} from 'react';
import {Controls, PixelGrid, Slider, Toggle} from '@site/src/components/kit';
import {blendPixel, maskColumns, rasterize} from '@site/src/lib/glyphBlend';
import styles from './fonts.module.css';

export default function GlyphBlend() {
  const [offset, setOffset] = useState(0.5);
  const [hinted, setHinted] = useState(false);
  const [ink, setInk] = useState(0);
  const [paper, setPaper] = useState(255);
  const [hovered, setHovered] = useState<number | null>(null);

  const usedOffset = hinted ? Math.round(offset) : offset;
  const mask = useMemo(() => rasterize(usedOffset), [usedOffset]);
  const page = mask.map((alpha) => blendPixel(paper, ink, alpha));
  const partial = mask.filter((alpha) => alpha > 0 && alpha < 255).length;

  const at = hovered ?? mask.findIndex((alpha) => alpha > 0 && alpha < 255);
  const alpha = at >= 0 ? mask[at] : 0;

  return (
    <div>
      <div className={styles.row}>
        <figure className={styles.panel}>
          <PixelGrid
            columns={maskColumns}
            cells={mask.map((value) => 255 - value)}
            cellSize={22}
            highlight={at >= 0 ? new Set([at]) : undefined}
            onCellHover={setHovered}
            ariaLabel="The glyph's alpha mask: darker means more of the pixel is covered"
          />
          <figcaption className={styles.caption}>the mask: how much of each pixel the outline covers</figcaption>
        </figure>
        <figure className={styles.panel}>
          <PixelGrid
            columns={maskColumns}
            cells={page}
            cellSize={22}
            highlight={at >= 0 ? new Set([at]) : undefined}
            onCellHover={setHovered}
            ariaLabel="The page after blending the ink through the mask"
          />
          <figcaption className={styles.caption}>the page after blend</figcaption>
        </figure>
      </div>
      <div className={styles.formula}>
        {at >= 0 ? (
          <>
            alpha {alpha}: ({paper} × (255 − {alpha}) + {ink} × {alpha} + 127) ÷ 255 = <strong>{blendPixel(paper, ink, alpha)}</strong>
          </>
        ) : (
          <span className={styles.muted}>hover a pixel</span>
        )}
        <div className={styles.muted}>
          {partial} pixels partly covered{hinted ? ', the dot on a whole pixel: the same mask every time' : ''}
        </div>
      </div>
      <Controls>
        <Slider label="Where the dot lands" value={offset} min={0} max={1} step={0.25} onChange={setOffset} format={(value) => `+${value.toFixed(2)} px`} />
        <Toggle label="Full hinting (whole pixels)" checked={hinted} onChange={setHinted} />
        <Slider label="Ink gray" value={ink} min={0} max={255} step={17} onChange={setInk} />
        <Slider label="Page gray" value={paper} min={0} max={255} step={17} onChange={setPaper} />
      </Controls>
    </div>
  );
}
