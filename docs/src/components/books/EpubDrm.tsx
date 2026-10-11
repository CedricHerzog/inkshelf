// checkEncryption (internal/ebook/epub/container/encryption.go) on three encryption.xml files:
// none, fonts obfuscated only, and a chapter encrypted. Only the last is refused with ErrDRM.

import React, {useState} from 'react';
import clsx from 'clsx';
import {Segmented} from '@site/src/components/kit';
import styles from './Epub.module.css';

// fontObfuscations: webfont.IDPFAlgorithm and webfont.AdobeAlgorithm.
const fontObfuscations = ['http://www.idpf.org/2008/embedding', 'http://ns.adobe.com/pdf/enc#RC'];

type Case = 'none' | 'fonts' | 'drm';

const examples: Record<Case, {file: string | null; data: {algorithm: string; uri: string}[]}> = {
  none: {file: null, data: []},
  fonts: {
    file: 'META-INF/encryption.xml',
    data: [{algorithm: 'http://www.idpf.org/2008/embedding', uri: 'OEBPS/fonts/serif.otf'}],
  },
  drm: {
    file: 'META-INF/encryption.xml',
    data: [
      {algorithm: 'http://www.idpf.org/2008/embedding', uri: 'OEBPS/fonts/serif.otf'},
      {algorithm: 'http://www.w3.org/2001/04/xmlenc#aes128-cbc', uri: 'OEBPS/text/c1.xhtml'},
    ],
  },
};

export default function EpubDrm() {
  const [which, setWhich] = useState<Case>('fonts');
  const example = examples[which];
  let refused = false;
  const checks = example.data.map((data) => {
    const allowed = fontObfuscations.includes(data.algorithm);
    const reached = !refused;
    if (!allowed) refused = true;
    return {...data, allowed, reached};
  });
  return (
    <div className={styles.drm}>
      <Segmented
        options={[
          {value: 'none', label: 'No encryption.xml'},
          {value: 'fonts', label: 'Fonts obfuscated'},
          {value: 'drm', label: 'A chapter encrypted'},
        ]}
        value={which}
        onChange={setWhich}
      />
      {example.file === null ? (
        <ul className={styles.checks}>
          <li>
            No <code>META-INF/encryption.xml</code> in the zip: nothing is encrypted.
          </li>
        </ul>
      ) : (
        <ul className={styles.checks}>
          {checks.map((check) => (
            <li key={check.uri} style={{opacity: check.reached ? 1 : 0.4}}>
              {check.allowed ? '✓' : '✗'} <code>{check.uri}</code> is encrypted with{' '}
              <code>{check.algorithm}</code>:{' '}
              {check.allowed
                ? 'only a font obfuscation, fine. Its path is noted, to undo it when the font loads.'
                : 'not a font obfuscation. Stop.'}
            </li>
          ))}
        </ul>
      )}
      <div className={clsx(styles.verdict, refused ? styles.verdictBad : styles.verdictOk)}>
        {refused
          ? 'Open returns ErrDRM: “the book is protected by DRM”.'
          : 'Open goes on to META-INF/container.xml.'}
      </div>
    </div>
  );
}
