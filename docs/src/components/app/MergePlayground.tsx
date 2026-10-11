// Who wins when a book opens: the reader's book.json or the server's progress. The rule is
// src/lib/merge.ts (progress.Merge); fromServer is what openbook.OpenLocked passes:
// app.Server.Online && !app.Server.LocalProgress.

import React, {useState} from 'react';
import clsx from 'clsx';
import {Controls, Slider, Toggle} from '@site/src/components/kit';
import {merge, type MergeReason} from '@site/src/lib/merge';
import styles from './app.module.css';

const reasons: Record<MergeReason, {winner: 'reader' | 'server' | 'tie'; say: string}> = {
  'not-from-server': {
    winner: 'reader',
    say: 'fromServer is false: the progress in the list was not just read from a server that keeps progress. The reader’s book.json stays as it is.',
  },
  unsynced: {
    winner: 'reader',
    say: 'The book is dirty (Unsynced): the reader has progress the server has not seen yet. Overwriting it would lose pages read offline.',
  },
  same: {
    winner: 'tie',
    say: 'Both say the same thing: nothing changes, nothing is saved.',
  },
  'server-wins': {
    winner: 'server',
    say: 'The server’s progress is fresh and the reader has nothing unsent: another device read this book since. The server wins, even if it is behind, and book.json is saved.',
  },
};

const rows: {fromServer: string; unsynced: string; same: string; reason: MergeReason; result: string}[] = [
  {fromServer: 'no', unsynced: 'any', same: 'any', reason: 'not-from-server', result: 'reader wins'},
  {fromServer: 'yes', unsynced: 'yes', same: 'any', reason: 'unsynced', result: 'reader wins'},
  {fromServer: 'yes', unsynced: 'no', same: 'yes', reason: 'same', result: 'nothing to do'},
  {fromServer: 'yes', unsynced: 'no', same: 'no', reason: 'server-wins', result: 'server wins, saved'},
];

export default function MergePlayground() {
  const [localPage, setLocalPage] = useState(14);
  const [localCompleted, setLocalCompleted] = useState(false);
  const [unsynced, setUnsynced] = useState(false);
  const [remotePage, setRemotePage] = useState(31);
  const [remoteCompleted, setRemoteCompleted] = useState(false);
  const [online, setOnline] = useState(true);
  const [keepsProgress, setKeepsProgress] = useState(true);
  const fromServer = online && keepsProgress;
  const result = merge(
    {page: localPage, completed: localCompleted, unsynced},
    {page: remotePage, completed: remoteCompleted},
    fromServer,
  );
  const reason = reasons[result.reason];
  const arrow =
    reason.winner === 'server' ? '⟵ server wins' : reason.winner === 'reader' ? 'reader wins ⟶' : '=';

  return (
    <div>
      <div className={styles.row}>
        <div className={clsx(styles.card, styles.grow)}>
          <div className={styles.cardTitle}>📖 Reader (book.json)</div>
          <Slider label="page" value={localPage} min={1} max={48} onChange={setLocalPage} />
          <Toggle label="completed" checked={localCompleted} onChange={setLocalCompleted} />
          <br />
          <Toggle label="dirty (Unsynced)" checked={unsynced} onChange={setUnsynced} />
        </div>
        <div className={clsx(styles.card, styles.grow)}>
          <div className={styles.cardTitle}>🖥️ Server (from the book list)</div>
          <Slider label="page" value={remotePage} min={1} max={48} onChange={setRemotePage} />
          <Toggle label="completed" checked={remoteCompleted} onChange={setRemoteCompleted} />
        </div>
      </div>
      <Controls>
        <Toggle label="Online" checked={online} onChange={setOnline} />
        <Toggle
          label="Server keeps progress (Komga; not Mealie or Calibre-Web)"
          checked={keepsProgress}
          onChange={setKeepsProgress}
        />
        <span className={styles.mono}>fromServer = {String(fromServer)}</span>
      </Controls>
      <div
        className={styles.verdict}
        style={{
          textAlign: 'center',
          color:
            reason.winner === 'server'
              ? 'var(--ink-blue)'
              : reason.winner === 'reader'
                ? 'var(--ink-ok)'
                : 'var(--ink-muted)',
        }}>
        {arrow}
      </div>
      <div className={styles.card}>
        <div>
          Merge returns page <strong>{result.merged.page}</strong>
          {result.merged.completed ? ', completed' : ''}; changed ={' '}
          <strong>{String(result.changed)}</strong>
          {result.changed ? ' (book.json is saved)' : ''}.
        </div>
        <div className={styles.muted} style={{marginTop: '0.3rem'}}>
          {reason.say}
        </div>
      </div>
      <div style={{overflowX: 'auto', marginTop: '0.8rem'}}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>fromServer</th>
              <th>dirty</th>
              <th>same page & completed</th>
              <th>result</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.reason} className={clsx(row.reason === result.reason && styles.rowNow)}>
                <td>{row.fromServer}</td>
                <td>{row.unsynced}</td>
                <td>{row.same}</td>
                <td>{row.result}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
