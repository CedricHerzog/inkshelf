// A line of text cut into parts, one per font, as forEachCoveredPart does: the characters the
// text's font lacks go to the first fallback that has them, or stay as .notdef boxes.

import React, {useState} from 'react';
import {Button, Segmented} from '@site/src/components/kit';
import {bookFont, coveredParts, dejaVuSans, goFont, readerSerif, type ToyFont} from '@site/src/lib/fontFallback';
import styles from './fonts.module.css';

const samples = [
  'Le café coûte 3 €',
  'λόγος → the word',
  'Shalom: שלום',
  'Kanji: 漢字',
  '∀x ∈ ℝ ☺',
];

const colors = new Map<string, string>([
  [bookFont.name, 'var(--ink-muted)'],
  [readerSerif.name, 'var(--ink-blue)'],
  [dejaVuSans.name, 'var(--ink-ok)'],
  [goFont.name, 'var(--ink-accent)'],
]);

const fallbacks = [readerSerif, dejaVuSans, goFont];

export default function FontFallback() {
  const [text, setText] = useState(samples[1]);
  const [fontName, setFontName] = useState<'book' | 'reader'>('book');
  const font: ToyFont = fontName === 'book' ? bookFont : readerSerif;
  const {parts, oneCheck} = coveredParts(font, fallbacks, text);

  return (
    <div>
      <Segmented
        options={[
          {value: 'book', label: 'Text in a book’s font'},
          {value: 'reader', label: 'Text in the reader’s serif'},
        ]}
        value={fontName}
        onChange={setFontName}
      />
      <div className={styles.samples}>
        {samples.map((sample) => (
          <Button key={sample} onClick={() => setText(sample)} active={sample === text}>
            {sample}
          </Button>
        ))}
      </div>
      <input
        className={styles.input}
        value={text}
        onChange={(event) => setText(event.target.value)}
        aria-label="Text to draw"
      />
      <div className={styles.line} aria-label="The parts, one per font">
        {parts.map((part, index) => (
          <span
            key={index}
            className={`${styles.part} ${part.missing ? styles.missing : ''}`}
            style={{borderColor: part.missing ? 'var(--ink-warn)' : colors.get(part.font.name)}}
            title={part.missing ? `${part.font.name} lacks these: drawn as .notdef boxes` : part.font.name}>
            {part.missing ? '□'.repeat(Array.from(part.text).length) : part.text}
          </span>
        ))}
      </div>
      <div className={styles.legend}>
        {[font, ...fallbacks.filter((fallback) => fallback !== font)].map((each, index) => (
          <span key={each.name}>
            <span className={styles.swatch} style={{background: colors.get(each.name)}} />
            {index === 0 ? `${each.name} (the text’s font)` : `${index}. ${each.name}`}
          </span>
        ))}
        <span>
          <span className={styles.swatch} style={{background: 'var(--ink-warn)'}} />
          .notdef box
        </span>
      </div>
      <p className={styles.muted} style={{fontSize: '0.85rem', marginTop: '0.6rem', marginBottom: 0}}>
        {oneCheck
          ? 'The font covers the whole text: one Covers check, one part, no fallback opened.'
          : `${parts.length} part${parts.length === 1 ? '' : 's'}, each drawn with its own font and lowered to share the first font's baseline.`}
      </p>
    </div>
  );
}
