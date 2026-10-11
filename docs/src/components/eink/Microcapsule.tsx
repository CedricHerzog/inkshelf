// A cross-section of five e-ink pixels: each a capsule of white and black particles between a
// transparent electrode on top and a pixel electrode below. Clicking a pixel drives it; the
// particles swim to their new side, and the strip above shows what the eye sees. Switching
// the power off shows that the image stays: e-ink is bistable.

import React, {useState} from 'react';
import {
  Controls,
  Segmented,
  Toggle,
  useAnimationFrame,
  usePrefersReducedMotion,
} from '@site/src/components/kit';
import {seededRandom} from '@site/src/lib/sampleImages';
import styles from './eink.module.css';

const pixelCount = 5;
const particlesPerColor = 9;
/** How long a full swim takes here: slowed down about 2× so the eye can follow it. */
const swimMs = 1000;

const cellWidth = 66;
const left = 15;
const capsuleTop = 64;
const capsuleBottom = 176;
const topLayer = [76, 100];
const bottomLayer = [140, 164];

type Particle = {x: number; layerOffset: number; delay: number; white: boolean};

/** The particles of each pixel, placed once: their spot in the layer and their pace. */
const particles: Particle[][] = (() => {
  const random = seededRandom(42);
  return Array.from({length: pixelCount}, () =>
    Array.from({length: particlesPerColor * 2}, (_, index) => ({
      x: 10 + random() * (cellWidth - 26),
      layerOffset: random(),
      delay: random(),
      white: index % 2 === 0,
    })),
  );
})();

/** 0 = white on top (the pixel looks white), 1 = black on top. */
type Pixel = {position: number; goal: number};

type Mode = 'flip' | 'short';

function smooth(value: number): number {
  const clamped = Math.min(1, Math.max(0, value));
  return clamped * clamped * (3 - 2 * clamped);
}

export default function Microcapsule() {
  const reduced = usePrefersReducedMotion();
  const [pixels, setPixels] = useState<Pixel[]>(() =>
    [0, 1, 0, 1, 1].map((position) => ({position, goal: position})),
  );
  const [power, setPower] = useState(true);
  const [mode, setMode] = useState<Mode>('flip');

  const moving = pixels.some((pixel) => pixel.position !== pixel.goal);
  useAnimationFrame((elapsedMs) => {
    const stepSize = elapsedMs / swimMs;
    setPixels((current) =>
      current.map((pixel) => {
        if (pixel.position === pixel.goal) return pixel;
        const direction = Math.sign(pixel.goal - pixel.position);
        const position = pixel.position + direction * stepSize;
        const arrived = direction > 0 ? position >= pixel.goal : position <= pixel.goal;
        return {position: arrived ? pixel.goal : position, goal: pixel.goal};
      }),
    );
  }, moving && power);

  const drive = (index: number) => {
    if (!power) return;
    setPixels((current) =>
      current.map((pixel, at) => {
        if (at !== index) return pixel;
        let goal: number;
        if (mode === 'flip') goal = pixel.goal < 0.5 ? 1 : 0;
        else goal = pixel.goal <= 0.5 ? pixel.goal + 0.5 : pixel.goal - 0.5;
        return {position: reduced ? goal : pixel.position, goal};
      }),
    );
  };

  return (
    <div>
      <svg
        className={styles.svg}
        viewBox="0 0 360 236"
        role="img"
        aria-label="Five e-ink pixels in cross-section, with their particles">
        <text x={180} y={12} textAnchor="middle" fontSize={10} fill="var(--ink-muted)">
          what the eye sees, from above ↓
        </text>
        {pixels.map((pixel, index) => {
          const level = Math.round(255 * (1 - pixel.position));
          return (
            <rect
              key={`seen${index}`}
              x={left + index * cellWidth + 6}
              y={20}
              width={cellWidth - 12}
              height={22}
              fill={`rgb(${level},${level},${level})`}
              stroke="var(--ink-line)"
              onClick={() => drive(index)}
              style={{cursor: power ? 'pointer' : 'not-allowed'}}
            />
          );
        })}
        {/* The transparent common electrode on top. */}
        <rect x={left} y={52} width={pixelCount * cellWidth} height={6} fill="var(--ink-blue-soft)" stroke="var(--ink-blue)" strokeWidth={0.8} />
        <text x={left + pixelCount * cellWidth} y={49} textAnchor="end" fontSize={8.5} fill="var(--ink-blue)">
          transparent electrode
        </text>
        {pixels.map((pixel, index) => {
          const x0 = left + index * cellWidth;
          const towardBlack = pixel.goal > pixel.position;
          const towardWhite = pixel.goal < pixel.position;
          const sign = !power ? '0 V' : towardBlack ? '+' : towardWhite ? '−' : '·';
          return (
            <g key={index} onClick={() => drive(index)} style={{cursor: power ? 'pointer' : 'not-allowed'}}>
              <rect
                x={x0 + 4}
                y={capsuleTop}
                width={cellWidth - 8}
                height={capsuleBottom - capsuleTop}
                rx={24}
                fill="var(--ink-paper-2)"
                stroke="var(--ink-muted)"
                strokeWidth={1.2}
              />
              {particles[index].map((particle, particleIndex) => {
                const progress = smooth(pixel.position * 1.3 - particle.delay * 0.3);
                const top = topLayer[0] + particle.layerOffset * (topLayer[1] - topLayer[0]);
                const bottom = bottomLayer[0] + particle.layerOffset * (bottomLayer[1] - bottomLayer[0]);
                // White particles start on top and sink as the pixel turns black; black ones rise.
                const y = particle.white ? top + (bottom - top) * progress : bottom + (top - bottom) * progress;
                return (
                  <circle
                    key={particleIndex}
                    cx={x0 + 4 + particle.x}
                    cy={y}
                    r={5}
                    fill={particle.white ? '#f4f2ec' : '#1d1b18'}
                    stroke={particle.white ? '#8d877b' : '#000'}
                    strokeWidth={0.7}
                  />
                );
              })}
              {/* The pixel's own electrode, and the voltage it is driven with. */}
              <rect
                x={x0 + 8}
                y={184}
                width={cellWidth - 16}
                height={14}
                rx={2}
                fill={power && (towardBlack || towardWhite) ? 'var(--ink-accent-soft)' : 'var(--ink-paper-2)'}
                stroke="var(--ink-muted)"
                strokeWidth={0.8}
              />
              <text x={x0 + cellWidth / 2} y={194.5} textAnchor="middle" fontSize={9} fontFamily="var(--ink-mono)" fill="var(--ink-text)">
                {sign}
              </text>
            </g>
          );
        })}
        <text x={180} y={216} textAnchor="middle" fontSize={9.5} fill="var(--ink-muted)">
          pixel electrodes: + pulls the white (−) particles down, − pushes them up
        </text>
        <text x={180} y={230} textAnchor="middle" fontSize={9.5} fill="var(--ink-muted)">
          {power ? (moving ? 'driving…' : 'at rest: no voltage needed to keep the image') : 'power off: nothing moves, the image stays'}
        </text>
      </svg>
      <Controls>
        <Segmented
          options={[
            {value: 'flip', label: 'Full pulse (flip)'},
            {value: 'short', label: 'Half pulse (gray)'},
          ]}
          value={mode}
          onChange={setMode}
        />
        <Toggle label="Power" checked={power} onChange={setPower} />
      </Controls>
    </div>
  );
}
