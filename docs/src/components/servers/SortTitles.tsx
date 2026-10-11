// A shelf of titles sorted twice: as a plain string sort would, and as servers.CompareTitles
// does, with each title's sort key (no accents, no leading article, numbers as numbers).

import React, {useState} from 'react';
import {Button, Controls, Segmented} from '@site/src/components/kit';
import {compareTitles, plainCompare, sortKey, withoutArticle} from '@site/src/lib/sortKey';
import styles from './servers.module.css';

const initial = [
  'Tome 10',
  'Le Trône de fer',
  'Tome 2',
  'L’Arabe du futur',
  'The Hobbit',
  'Élégance du hérisson',
  'A la croisée des mondes',
  'Œufs brouillés',
  'Astérix',
  'Les Rois maudits',
];

export default function SortTitles() {
  const [titles, setTitles] = useState(initial);
  const [draft, setDraft] = useState('');
  const [mode, setMode] = useState<'inkshelf' | 'plain'>('inkshelf');

  const sorted = [...titles].sort(mode === 'inkshelf' ? compareTitles : plainCompare);

  const add = () => {
    const title = draft.trim();
    if (title === '') return;
    setTitles((current) => [...current, title]);
    setDraft('');
  };

  return (
    <div>
      <Segmented
        options={[
          {value: 'inkshelf', label: 'CompareTitles'},
          {value: 'plain', label: 'Plain string sort'},
        ]}
        value={mode}
        onChange={setMode}
      />
      <ol className={styles.list} style={{marginTop: '0.7rem'}}>
        {sorted.map((title, index) => {
          const {article} = withoutArticle(title.replace(/[’‘ʼ]/g, "'").trim());
          return (
            <li key={`${title}-${index}`}>
              {article ? (
                <>
                  <span className={styles.dropped}>{title.slice(0, article.length)}</span>
                  {title.slice(article.length)}
                </>
              ) : (
                title
              )}{' '}
              {mode === 'inkshelf' && <span className={styles.key}>→ {sortKey(title).toLowerCase()}</span>}
            </li>
          );
        })}
      </ol>
      <Controls>
        <input
          className={styles.input}
          value={draft}
          placeholder="Add a title, e.g. Tome 9"
          onChange={(event) => setDraft(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') add();
          }}
          aria-label="A title to add"
        />
        <Button onClick={add}>Add</Button>
        <Button onClick={() => setTitles(initial)}>Reset</Button>
      </Controls>
    </div>
  );
}
