// How a .pg4 file is laid out (pg4.go): two pixels a byte, the left one in the high 4 bits,
// after an 8-byte header: "PG4\x01", then the width and the height as little-endian uint16.
// A 5×3 image to edit, the bits of the selected pair, and every byte of its file.

import React, {useMemo, useState} from 'react';
import {Controls, PixelGrid, Slider} from '@site/src/components/kit';
import {grayStep, newPG4, pg4Header} from '@site/src/lib/pageimage';
import styles from './grays.module.css';

const width = 5;
const height = 3;

function hex(value: number) {
  return value.toString(16).toUpperCase().padStart(2, '0');
}

function bits(value: number, count: number) {
  return value.toString(2).padStart(count, '0');
}

const headerNames = ['P', 'G', '4', '\\x01', 'width, low byte', 'width, high byte', 'height, low byte', 'height, high byte'];

export default function NibblePacking() {
  const [indexes, setIndexes] = useState<number[]>([15, 10, 4, 0, 8, 12, 6, 15, 3, 9, 0, 5, 11, 15, 2]);
  const [selected, setSelected] = useState(0);

  const {bytes, rowBytes} = useMemo(() => {
    const page = newPG4(width, height);
    for (let row = 0; row < height; row++) {
      for (let column = 0; column < width; column++) {
        const index = indexes[row * width + column];
        const at = pg4Header + row * page.rowBytes + (column >> 1);
        if (column % 2 === 0) page.bytes[at] = index << 4;
        else page.bytes[at] |= index;
      }
    }
    return page;
  }, [indexes]);

  const row = Math.floor(selected / width);
  const column = selected % width;
  const leftColumn = column - (column % 2);
  const left = row * width + leftColumn;
  const right = leftColumn + 1 < width ? left + 1 : null;
  const byteAt = pg4Header + row * rowBytes + (leftColumn >> 1);
  const pair = new Set(right === null ? [left] : [left, right]);

  const setIndex = (value: number) =>
    setIndexes((current) => current.map((index, at) => (at === selected ? value : index)));

  const selectByte = (at: number) => {
    if (at < pg4Header) return;
    const offset = at - pg4Header;
    setSelected(Math.floor(offset / rowBytes) * width + (offset % rowBytes) * 2);
  };

  const fullWidth = 1404;
  const fullHeight = 1872;
  const eightBit = fullWidth * fullHeight;
  const pg4 = pg4Header + Math.floor((fullWidth + 1) / 2) * fullHeight;

  return (
    <div>
      <div className={styles.row}>
        <figure className={styles.canvasFigure}>
          <PixelGrid
            columns={width}
            cells={indexes.map((index) => index * grayStep)}
            labels={indexes.map(String)}
            cellSize={40}
            highlight={new Set([selected])}
            outline={pair}
            onCellClick={setSelected}
            ariaLabel="A 5 by 3 image; click a pixel to select it"
          />
          <figcaption className={styles.canvasCaption}>a 5×3 page; the numbers are gray indexes 0–15</figcaption>
        </figure>
        <table className={styles.table}>
          <tbody>
            <tr>
              <td>left pixel</td>
              <td className={styles.mono}>
                {indexes[left]} × 17 = {indexes[left] * grayStep}
              </td>
              <td className={`${styles.bits} ${styles.high}`}>{bits(indexes[left], 4)}</td>
            </tr>
            <tr>
              <td>right pixel</td>
              <td className={styles.mono}>{right === null ? 'none (odd width)' : `${indexes[right]} × 17 = ${indexes[right] * grayStep}`}</td>
              <td className={`${styles.bits} ${styles.low}`}>{right === null ? '0000' : bits(indexes[right], 4)}</td>
            </tr>
            <tr>
              <td>one byte</td>
              <td className={styles.mono}>0x{hex(bytes[byteAt])}</td>
              <td className={styles.bits}>
                <span className={styles.high}>{bits(bytes[byteAt] >> 4, 4)}</span>
                <span className={styles.low}>{bits(bytes[byteAt] & 15, 4)}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <Controls>
        <Slider label={`Pixel (${column}, ${row})`} value={indexes[selected]} min={0} max={15} onChange={setIndex} format={(value) => `gray ${value}`} />
      </Controls>
      <div className={styles.byteStream} aria-label="The bytes of the .pg4 file">
        {Array.from(bytes).map((value, at) => (
          <span
            key={at}
            title={at < pg4Header ? `byte ${at}: ${headerNames[at]}` : `byte ${at}: row ${Math.floor((at - pg4Header) / rowBytes)}`}
            onClick={() => selectByte(at)}
            onMouseEnter={() => selectByte(at)}
            className={`${styles.byte} ${at < 4 ? styles.byteMagic : at < pg4Header ? styles.byteSize : ''} ${at === byteAt ? styles.byteActive : ''}`}
            style={{cursor: at >= pg4Header ? 'pointer' : 'default'}}>
            {hex(value)}
          </span>
        ))}
      </div>
      <p className={styles.note}>
        <span className={styles.byte + ' ' + styles.byteMagic}>50 47 34 01</span> is “PG4” and version 1;{' '}
        <span className={styles.byte + ' ' + styles.byteSize}>05 00 03 00</span> are 5 and 3, low byte first; then{' '}
        {height} rows of (5 + 1) ÷ 2 = {rowBytes} bytes. The last byte of a row only uses its high half.
      </p>
      <table className={styles.table}>
        <tbody>
          <tr>
            <td>A full page, 1404 × 1872, one byte a pixel</td>
            <td className={styles.mono} style={{textAlign: 'right'}}>{eightBit.toLocaleString('en')} bytes</td>
          </tr>
          <tr>
            <td>The same page as .pg4: 8 + 702 × 1872</td>
            <td className={styles.mono} style={{textAlign: 'right'}}>{pg4.toLocaleString('en')} bytes</td>
          </tr>
          <tr>
            <td>A 200-page comic as .pg4 (pages that fill the screen)</td>
            <td className={styles.mono} style={{textAlign: 'right'}}>{((pg4 * 200) / 1e6).toFixed(0)} MB</td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}
