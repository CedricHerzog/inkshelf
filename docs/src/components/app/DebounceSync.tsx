// The debounced push of reading progress: each page turn marks the book dirty and restarts a
// 3 s timer (Syncer.Schedule with openbook.SyncDelay, a time.AfterFunc reset); when it fires,
// PushNow sends the unsynced progress and clears the flag only if the page did not change
// while the request was out. Offline, the push fails and the book stays dirty until the
// network comes back (NetConnected: Schedule(0)). Follows internal/progress/syncer.go.

import React, {useState} from 'react';
import clsx from 'clsx';
import {Button, Controls, Toggle, useAnimationFrame} from '@site/src/components/kit';
import styles from './app.module.css';

const syncDelay = 3000;
const requestMs = 900;
const failMs = 400;

type Request = {page: number; left: number; fails: boolean};

type Sim = {
  page: number;
  dirty: boolean;
  serverPage: number;
  timer: number | null; // ms left before PushNow
  request: Request | null;
  pushAgain: boolean; // PushNow called while one runs: it waits for the pushing lock
  turns: number;
  pushes: number;
  log: string[];
  changed: 'page' | 'dirty' | null;
};

const initial: Sim = {
  page: 12,
  dirty: false,
  serverPage: 12,
  timer: null,
  request: null,
  pushAgain: false,
  turns: 0,
  pushes: 0,
  log: ['book b1 open at page 12, synced'],
  changed: null,
};

function log(sim: Sim, line: string): string[] {
  return [line, ...sim.log].slice(0, 7);
}

/** pushNow starts PushNow: nothing to do if the book is not dirty. */
function pushNow(sim: Sim, online: boolean): Sim {
  if (sim.request) return {...sim, pushAgain: true};
  if (!sim.dirty) return {...sim, log: log(sim, 'PushNow: no unsynced book, nothing sent')};
  return {
    ...sim,
    request: {page: sim.page, left: online ? requestMs : failMs, fails: !online},
    pushes: sim.pushes + 1,
    log: log(sim, `PushNow: SetProgress(b1, page ${sim.page})…`),
  };
}

export default function DebounceSync() {
  const [online, setOnline] = useState(true);
  const [sim, setSim] = useState<Sim>(initial);

  const schedule = (current: Sim, delay: number): Sim =>
    delay === 0 ? pushNow({...current, timer: null}, online) : {...current, timer: delay};

  useAnimationFrame(
    (elapsed) => {
      setSim((current) => {
        let next = current;
        if (next.timer !== null) {
          const timer = next.timer - elapsed;
          next = timer > 0 ? {...next, timer} : pushNow({...next, timer: null}, online);
        }
        if (next.request) {
          const left = next.request.left - elapsed;
          if (left > 0) return {...next, request: {...next.request, left}};
          const sent = next.request;
          if (sent.fails) {
            next = {
              ...next,
              request: null,
              log: log(next, 'sync b1: no network: stays dirty, retried later'),
            };
          } else if (next.page === sent.page) {
            next = {
              ...next,
              request: null,
              serverPage: sent.page,
              dirty: false,
              changed: 'dirty',
              log: log(next, `synced b1: page ${sent.page}: markSyncedLocked clears dirty`),
            };
          } else {
            next = {
              ...next,
              request: null,
              serverPage: sent.page,
              log: log(
                next,
                `server has page ${sent.page}, but the reader is at ${next.page}: stays dirty`,
              ),
            };
          }
          if (next.pushAgain) next = pushNow({...next, pushAgain: false}, online);
        }
        return next;
      });
    },
    sim.timer !== null || sim.request !== null,
  );

  const turn = () =>
    setSim((current) =>
      schedule(
        {
          ...current,
          page: current.page + 1,
          dirty: true,
          turns: current.turns + 1,
          changed: 'page',
          log: log(current, `turn to page ${current.page + 1}: dirty, Schedule(3 s)`),
        },
        syncDelay,
      ),
    );

  const timerFraction = sim.timer === null ? 0 : sim.timer / syncDelay;
  const requestFraction = sim.request
    ? 1 - sim.request.left / (sim.request.fails ? failMs : requestMs)
    : 0;

  return (
    <div>
      <Controls>
        <Button primary onClick={turn}>
          Turn page ›
        </Button>
        <Button
          onClick={() =>
            setSim((current) =>
              schedule({...current, log: log(current, 'book closed: Schedule(0)')}, 0),
            )
          }>
          Close the book
        </Button>
        <Toggle
          label="Network up"
          checked={online}
          onChange={(value) => {
            setOnline(value);
            if (value) {
              // NetConnected: Schedule(0). The push must see the network up: build it here.
              setSim((current) => {
                const withLog = {...current, timer: null, log: log(current, 'network up: Schedule(0)')};
                if (withLog.request) return {...withLog, pushAgain: true};
                if (!withLog.dirty) return {...withLog, log: log(withLog, 'PushNow: no unsynced book, nothing sent')};
                return {
                  ...withLog,
                  request: {page: withLog.page, left: requestMs, fails: false},
                  pushes: withLog.pushes + 1,
                  log: log(withLog, `PushNow: SetProgress(b1, page ${withLog.page})…`),
                };
              });
            }
          }}
        />
      </Controls>
      <div className={styles.row}>
        <div className={clsx(styles.card, styles.grow)}>
          <div className={styles.cardTitle}>📖 The reader: book.json</div>
          <pre className={styles.json}>
            {'{"id": "b1", "page": '}
            <span className={clsx(sim.changed === 'page' && styles.changed)}>{sim.page}</span>
            {', "completed": false,\n "dirty": '}
            <span className={clsx(sim.changed === 'dirty' && styles.changed)}>
              {String(sim.dirty)}
            </span>
            {'}'}
          </pre>
        </div>
        <div className={clsx(styles.card, styles.grow)}>
          <div className={styles.cardTitle}>🖥️ The server</div>
          <div>
            book b1 read to page <strong>{sim.serverPage}</strong>
            {sim.serverPage !== sim.page && <span className={styles.muted}> (behind)</span>}
          </div>
        </div>
      </div>
      <div style={{margin: '0.7rem 0 0.3rem', fontSize: '0.85rem'}} className={styles.mono}>
        timer: {sim.timer === null ? 'not running' : `${(sim.timer / 1000).toFixed(1)} s until PushNow`}
      </div>
      <div className={styles.bar}>
        <div className={styles.barFill} style={{width: `${timerFraction * 100}%`}} />
      </div>
      <div style={{margin: '0.6rem 0 0.3rem', fontSize: '0.85rem'}} className={styles.mono}>
        {sim.request
          ? sim.request.fails
            ? `sending page ${sim.request.page}… (no network)`
            : `sending page ${sim.request.page} to the server…`
          : 'no request out'}
      </div>
      <div className={styles.bar}>
        <div
          className={clsx(styles.barFill, !sim.request?.fails && styles.barFillOk)}
          style={{width: `${requestFraction * 100}%`}}
        />
      </div>
      <div className={styles.counters}>
        <span className={styles.counter}>
          <strong>{sim.turns}</strong>page turns
        </span>
        <span className={clsx(styles.counter, styles.counterAccent)}>
          <strong>{sim.pushes}</strong>requests sent
        </span>
      </div>
      <div className={styles.log} aria-live="polite">
        {sim.log.map((line, index) => (
          <div key={`${sim.log.length}-${index}-${line}`} style={{opacity: 1 - index * 0.12}}>
            {line}
          </div>
        ))}
      </div>
    </div>
  );
}
