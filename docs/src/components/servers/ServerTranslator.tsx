// What each kind of server sends, and what the app sees of it through servers.Server: the same
// Series and Book, a Format, and the methods that answer errors.ErrUnsupported. Taken from the
// adapters (internal/servers/{komga,mealie,calibreweb}/adapter.go) and their doc.go.

import React, {useState} from 'react';
import {Segmented} from '@site/src/components/kit';
import styles from './servers.module.css';

type Kind = 'komga' | 'mealie' | 'calibreweb';

const methods = [
  'Series',
  'Libraries',
  'Books',
  'InProgress',
  'OpenBook',
  'OpenFile',
  'FirstPage',
  'SeriesThumbnail',
  'BookThumbnail',
  'SetProgress',
  'ClearProgress',
] as const;

const kinds: Record<
  Kind,
  {
    auth: string;
    request: string;
    answer: string;
    series: string;
    book: string;
    format: string;
    unsupported: readonly (typeof methods)[number][];
    note: string;
  }
> = {
  komga: {
    auth: 'X-API-Key: …',
    request: 'GET /api/v1/series/{id}/books',
    answer: '{"content": [{"id": "0A1…", "name": "Tome 1",\n  "media": {"pagesCount": 46},\n  "readProgress": {"page": 12}}], …}',
    series: 'a Komga series',
    book: 'a volume or a chapter',
    format: 'PageImages: fetched page by page, /api/v1/books/{id}/pages/{n}',
    unsupported: ['OpenFile'],
    note: 'The only kind that keeps reading progress: the app reads it with the books and PATCHes it back.',
  },
  mealie: {
    auth: 'Authorization: Bearer …',
    request: 'GET /api/recipes?perPage=-1',
    answer: '{"items": [{"id": "…", "slug": "tarte-tatin",\n  "name": "Tarte Tatin",\n  "recipeCategory": [{"name": "Desserts"}]}]}',
    series: 'a category ("uncategorized" for the rest)',
    book: 'a recipe',
    format: 'PageImages: but the pages are laid out and drawn on the reader, by recipepages',
    unsupported: ['Libraries', 'InProgress', 'OpenFile', 'SetProgress', 'ClearProgress'],
    note: 'A recipe has no pages: Books lays out every recipe it lists to count them, and keeps the result.',
  },
  calibreweb: {
    auth: 'HTTP Basic: username and password',
    request: 'GET /opds/series/{id}',
    answer: '<feed><entry><title>Eragon</title>\n  <content>SERIES: Eragon [2.5]</content>\n  <link rel="…acquisition" href="/opds/download/7/epub/"/>…',
    series: 'a Calibre series (books in none are gathered into one)',
    book: 'a book, its number read from "[2.5]"',
    format: 'Reflowable for an EPUB, ComicArchive for a CBZ: fetched whole by OpenFile',
    unsupported: ['Libraries', 'InProgress', 'OpenBook', 'SetProgress', 'ClearProgress'],
    note: 'OPDS only: the JSON API under /ajax needs a login session. Files come through a client with no timeout.',
  },
};

export default function ServerTranslator() {
  const [kind, setKind] = useState<Kind>('komga');
  const shown = kinds[kind];
  return (
    <div>
      <Segmented
        options={[
          {value: 'komga', label: 'Komga'},
          {value: 'mealie', label: 'Mealie'},
          {value: 'calibreweb', label: 'Calibre-Web'},
        ]}
        value={kind}
        onChange={setKind}
      />
      <div className={styles.columns}>
        <div className={styles.box}>
          <div className={styles.boxTitle}>What the server says</div>
          <pre className={styles.code}>
            {shown.request}
            {'\n'}
            {shown.auth}
            {'\n\n'}
            {shown.answer}
          </pre>
        </div>
        <div className={styles.box}>
          <div className={styles.boxTitle}>What the app sees</div>
          <div>
            <span className={styles.mono}>Series</span>: {shown.series}
          </div>
          <div>
            <span className={styles.mono}>Book</span>: {shown.book}
          </div>
          <div>
            <span className={styles.mono}>Format</span>: {shown.format}
          </div>
          <p style={{margin: '0.4rem 0 0', color: 'var(--ink-muted)'}}>{shown.note}</p>
        </div>
        <div className={styles.box}>
          <div className={styles.boxTitle}>servers.Server, method by method</div>
          <table className={styles.methods}>
            <tbody>
              {methods.map((method) => {
                const unsupported = shown.unsupported.includes(method);
                return (
                  <tr key={method}>
                    <td className={styles.mono}>{method}</td>
                    <td className={unsupported ? styles.no : styles.yes}>{unsupported ? 'ErrUnsupported' : '✓'}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
