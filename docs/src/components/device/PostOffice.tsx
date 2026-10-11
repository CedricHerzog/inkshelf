// The firmware's three channels as a post office: the global-state shared memory (a whiteboard
// everyone reads), /hwevent (one public mailbox for input) and the task queue (pigeonholes
// sorted by mtype). Follows firmware_shared_memory.go, firmware_messages.go, event_loop.go
// (handleTaskMessage, followForeground, readInput, request, readReplies) and system_events.go.
// The process ids are made up.

import React, {useEffect, useRef, useState} from 'react';
import {Button, Controls, useAnimationFrame, usePrefersReducedMotion} from '@site/src/components/kit';
import {FIRMWARE_MTYPE, TASK_EVENT_MTYPE, hex, requestID} from '@site/src/lib/firmwareMessages';
import styles from './device.module.css';

const INKSHELF = 812;
const OTHER = 640;

type Spot = 'hardware' | 'firmware' | 'board' | 'hwevent' | 'holeInk' | 'holeOther' | 'holeFirmware' | 'holeReply' | 'inkshelf' | 'other';

const spots: Record<Spot, [number, number]> = {
  hardware: [50, 38],
  firmware: [215, 38],
  board: [62, 168],
  hwevent: [180, 168],
  holeInk: [300, 141],
  holeOther: [300, 163],
  holeFirmware: [300, 185],
  holeReply: [300, 207],
  inkshelf: [90, 330],
  other: [270, 330],
};

type Tone = 'input' | 'event' | 'request';

type Action =
  | {kind: 'fly'; from: Spot; to: Spot; label: string; tone: Tone}
  | {kind: 'do'; run?: () => void; log?: string};

type Flight = {from: Spot; to: Spot; label: string; tone: Tone; progress: number};

const toneColor: Record<Tone, string> = {
  input: 'var(--ink-accent)',
  event: 'var(--ink-blue)',
  request: 'var(--ink-ok)',
};

const FLIGHT_MS = 650;

export default function PostOffice() {
  const reduced = usePrefersReducedMotion();
  const [foreground, setForeground] = useState(INKSHELF);
  const [queue, setQueue] = useState<Action[]>([]);
  const [flight, setFlight] = useState<Flight | null>(null);
  const [log, setLog] = useState<string[]>([
    'Inkshelf (pgid 812) is in front. Press a button below to post a letter.',
  ]);
  const [inkshelfFlash, setInkshelfFlash] = useState(false);
  const serial = useRef(0);
  const messageId = useRef(40);
  const logBox = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const element = logBox.current;
    if (element) element.scrollTop = element.scrollHeight;
  }, [log]);

  const inkshelfVisible = foreground === INKSHELF;

  // Run the queue: a flight animates, a "do" happens at once.
  useEffect(() => {
    if (flight || queue.length === 0) return;
    const [next, ...rest] = queue;
    if (next.kind === 'do') {
      next.run?.();
      if (next.log) setLog((lines) => [...lines.slice(-30), next.log as string]);
      setQueue(rest);
      return;
    }
    if (reduced) {
      setQueue(rest);
      return;
    }
    setFlight({from: next.from, to: next.to, label: next.label, tone: next.tone, progress: 0});
    setQueue(rest);
  }, [queue, flight, reduced]);

  useAnimationFrame((elapsed) => {
    setFlight((current) => {
      if (!current) return null;
      const progress = current.progress + elapsed / FLIGHT_MS;
      return progress >= 1 ? null : {...current, progress};
    });
  }, flight !== null);

  useEffect(() => {
    if (!inkshelfFlash) return undefined;
    const timer = window.setTimeout(() => setInkshelfFlash(false), 500);
    return () => window.clearTimeout(timer);
  }, [inkshelfFlash]);

  const busy = queue.length > 0 || flight !== null;
  const post = (actions: Action[]) => setQueue(actions);

  const tap = () =>
    post([
      {kind: 'fly', from: 'hardware', to: 'firmware', label: 'touch', tone: 'input'},
      {kind: 'do', log: 'The firmware reads the touch panel itself (apps may not) and writes a 32-byte message: {30 = pointer down, finger 0, raw x, raw y, pressure, tool, time}.'},
      {kind: 'fly', from: 'firmware', to: 'hwevent', label: '30', tone: 'input'},
      ...(inkshelfVisible
        ? ([
            {kind: 'fly', from: 'hwevent', to: 'inkshelf', label: '32 B', tone: 'input'},
            {
              kind: 'do',
              run: () => setInkshelfFlash(true),
              log: 'Inkshelf is in front: at its next 20 ms tick, readInput takes the message, touchPoint turns it upright, app.Pointer(Down) runs, and Refresh reaches the panel (visible() is true).',
            },
          ] as Action[])
        : ([
            {kind: 'fly', from: 'hwevent', to: 'other', label: '32 B', tone: 'input'},
            {
              kind: 'do',
              log: 'Inkshelf is in the background: followForeground returns false, so it never calls readInput. The touch belongs to the app in front, which reads it.',
            },
          ] as Action[])),
    ]);

  const home = () => {
    const goingBack = inkshelfVisible;
    const next = goingBack ? OTHER : INKSHELF;
    const event = goingBack ? 152 : 151;
    post([
      {kind: 'fly', from: 'hardware', to: 'firmware', label: 'Home', tone: 'input'},
      {
        kind: 'do',
        log: goingBack
          ? 'Home never reaches the app: the firmware handles it, and brings the other app to the front.'
          : 'The task switcher brings Inkshelf back.',
      },
      {kind: 'fly', from: 'firmware', to: 'board', label: String(next), tone: 'event'},
      {
        kind: 'do',
        run: () => setForeground(next),
        log: `The firmware writes the new foreground pgid, ${next}, on the whiteboard (offset 0x28).`,
      },
      {kind: 'fly', from: 'firmware', to: 'holeInk', label: String(event), tone: 'event'},
      {kind: 'fly', from: 'holeInk', to: 'inkshelf', label: String(event), tone: 'event'},
      {
        kind: 'do',
        run: goingBack ? undefined : () => setInkshelfFlash(true),
        log: goingBack
          ? 'Inkshelf reads event 152 (background) and lets it pass: followForeground reads the whiteboard instead. visible() is false now: no input read, every refresh skipped, held keys and touches forgotten.'
          : 'Event 151 (foreground). followForeground sees visible() turn true and calls app.Draw(): the panel still shows the other app’s picture.',
      },
    ]);
  };

  const systemEvent = (number: number, label: string, name: string, effect: string) =>
    post([
      {kind: 'fly', from: 'hardware', to: 'firmware', label, tone: 'input'},
      {kind: 'fly', from: 'firmware', to: 'holeInk', label: String(number), tone: 'event'},
      {kind: 'fly', from: 'holeInk', to: 'inkshelf', label: String(number), tone: 'event'},
      {
        kind: 'do',
        log: `Task event ${number} in pigeonhole ${hex(TASK_EVENT_MTYPE | INKSHELF)} → App.System(${name}): ${effect} Task messages are read even in the background.`,
      },
    ]);

  const download = () => {
    serial.current += 1;
    const id = requestID(INKSHELF, serial.current);
    post([
      {
        kind: 'do',
        log: `A download goroutine calls PreventSleepFor(1 min). It cannot touch the queue itself: the event loop sends request 0x123 with id ${hex(id)} (pid in bits 12–27, serial ${serial.current}).`,
      },
      {kind: 'fly', from: 'inkshelf', to: 'holeFirmware', label: '0x123', tone: 'request'},
      {kind: 'fly', from: 'holeFirmware', to: 'firmware', label: '60 s', tone: 'request'},
      {kind: 'do', log: `The firmware takes it from pigeonhole ${hex(FIRMWARE_MTYPE)}: no sleep for 60 s. It answers in the pigeonhole named by the id.`},
      {kind: 'fly', from: 'firmware', to: 'holeReply', label: 'reply', tone: 'request'},
      {kind: 'fly', from: 'holeReply', to: 'inkshelf', label: 'reply', tone: 'request'},
      {kind: 'do', log: `readReplies collects the reply with mtype ${hex(id)}. Nobody else would: an unread reply stays in the queue forever.`},
    ]);
  };

  const syncEvent = () => {
    messageId.current += 1;
    const id = messageId.current;
    post([
      {kind: 'fly', from: 'firmware', to: 'holeInk', label: '155', tone: 'event'},
      {kind: 'do', log: `A sync event (type 0x513), id ${id}: the firmware now waits for an answer before it goes on.`},
      {kind: 'fly', from: 'holeInk', to: 'inkshelf', label: '155', tone: 'event'},
      {kind: 'fly', from: 'inkshelf', to: 'holeReply', label: 'ok', tone: 'request'},
      {kind: 'fly', from: 'holeReply', to: 'firmware', label: 'ok', tone: 'request'},
      {kind: 'do', log: `handleTaskMessage answered first, with taskReply(${id}): {mtype ${id}, id ${id}, 0, 1}. Then App.System(SaveState) saves the reading position.`},
    ]);
  };

  const box = (x: number, y: number, width: number, height: number, title: string, fill = 'var(--ink-paper-2)', stroke = 'var(--ink-line)') => (
    <>
      <rect x={x} y={y} width={width} height={height} rx={8} fill={fill} stroke={stroke} strokeWidth={1.5} />
      <text x={x + width / 2} y={y + 15} textAnchor="middle" fontSize={11} fontWeight={650} fill="var(--ink-text)">
        {title}
      </text>
    </>
  );

  let flightNode: React.ReactNode = null;
  if (flight) {
    const [fromX, fromY] = spots[flight.from];
    const [toX, toY] = spots[flight.to];
    const eased = flight.progress < 0.5 ? 2 * flight.progress ** 2 : 1 - (-2 * flight.progress + 2) ** 2 / 2;
    const x = fromX + (toX - fromX) * eased;
    const y = fromY + (toY - fromY) * eased;
    flightNode = (
      <g transform={`translate(${x} ${y})`}>
        <rect x={-21} y={-10} width={42} height={20} rx={3} fill="var(--ink-paper)" stroke={toneColor[flight.tone]} strokeWidth={2} />
        <path d="M -21 -10 L 0 2 L 21 -10" fill="none" stroke={toneColor[flight.tone]} strokeWidth={1} />
        <text y={7} textAnchor="middle" fontSize={8.5} fontFamily="var(--ink-mono)" fill="var(--ink-text)">
          {flight.label}
        </text>
      </g>
    );
  }

  const holes: [Spot, string][] = [
    ['holeInk', `${hex(TASK_EVENT_MTYPE | INKSHELF)} Inkshelf`],
    ['holeOther', `${hex(TASK_EVENT_MTYPE | OTHER)} other`],
    ['holeFirmware', `${hex(FIRMWARE_MTYPE)} firmware`],
    ['holeReply', 'id → replies'],
  ];

  return (
    <div className={styles.stack}>
      <svg viewBox="0 0 360 380" className={styles.svg} style={{maxWidth: 560}} role="img" aria-label="The firmware, its three channels and two apps">
        {box(8, 14, 84, 48, 'Touch & keys')}
        <text x={50} y={50} textAnchor="middle" fontSize={9} fill="var(--ink-muted)">/dev/input</text>
        {box(150, 14, 130, 48, 'Firmware', 'var(--ink-paper-2)', 'var(--ink-text)')}
        <text x={215} y={50} textAnchor="middle" fontSize={9} fill="var(--ink-muted)">“pocketbook”, root</text>

        {box(6, 100, 112, 132, 'Whiteboard')}
        <text x={62} y={128} textAnchor="middle" fontSize={8.5} fill="var(--ink-muted)">shm 0xa12302</text>
        <text x={14} y={152} fontSize={9.5} fontFamily="var(--ink-mono)" fill="var(--ink-text)">0x02 suppressed 0</text>
        <text x={14} y={170} fontSize={9.5} fontFamily="var(--ink-mono)" fill={foreground === INKSHELF ? 'var(--ink-ok)' : 'var(--ink-accent)'}>
          0x28 front {foreground}
        </text>
        <text x={14} y={188} fontSize={9.5} fontFamily="var(--ink-mono)" fill="var(--ink-text)">0x30 battery 80</text>
        <text x={14} y={206} fontSize={9.5} fontFamily="var(--ink-mono)" fill="var(--ink-text)">0x3c multitask 0x544d</text>

        {box(124, 100, 112, 132, 'Public mailbox')}
        <text x={180} y={128} textAnchor="middle" fontSize={8.5} fill="var(--ink-muted)">/hwevent (POSIX)</text>
        <rect x={150} y={146} width={60} height={42} rx={4} fill="var(--ink-paper)" stroke="var(--ink-accent)" />
        <rect x={160} y={160} width={40} height={4} fill="var(--ink-text)" />
        <text x={180} y={206} textAnchor="middle" fontSize={8.5} fill="var(--ink-muted)">32-byte input</text>
        <text x={180} y={219} textAnchor="middle" fontSize={8.5} fill="var(--ink-muted)">front app only</text>

        {box(242, 100, 114, 132, 'Pigeonholes')}
        <text x={299} y={116 + 12} textAnchor="middle" fontSize={8.5} fill="var(--ink-muted)">task queue 0xa1230f</text>
        {holes.map(([spot, label]) => (
          <g key={spot}>
            <rect x={248} y={spots[spot][1] - 8} width={102} height={18} rx={3} fill="var(--ink-paper)" stroke="var(--ink-line)" />
            <text x={253} y={spots[spot][1] + 4} fontSize={7.6} fontFamily="var(--ink-mono)" fill="var(--ink-text)">
              {label}
            </text>
          </g>
        ))}

        <g opacity={inkshelfVisible ? 1 : 0.55}>
          {box(14, 290, 152, 80, 'Inkshelf · pgid 812', inkshelfFlash ? 'var(--ink-ok-soft)' : 'var(--ink-paper-2)', inkshelfVisible ? 'var(--ink-ok)' : 'var(--ink-line)')}
          <text x={90} y={326} textAnchor="middle" fontSize={9.5} fontFamily="var(--ink-mono)" fill="var(--ink-text)">
            visible() = {String(inkshelfVisible)}
          </text>
          <text x={90} y={346} textAnchor="middle" fontSize={9} fill="var(--ink-muted)">
            {inkshelfVisible ? 'reads input, refreshes' : 'input ignored, refresh skipped'}
          </text>
        </g>
        <g opacity={inkshelfVisible ? 0.55 : 1}>
          {box(194, 290, 152, 80, 'Other app · pgid 640', 'var(--ink-paper-2)', inkshelfVisible ? 'var(--ink-line)' : 'var(--ink-ok)')}
          <text x={270} y={326} textAnchor="middle" fontSize={9} fill="var(--ink-muted)">
            {inkshelfVisible ? 'in the background' : 'in front: reads input'}
          </text>
        </g>
        {flightNode}
      </svg>
      <Controls>
        <Button primary onClick={tap} disabled={busy}>
          Tap screen
        </Button>
        <Button onClick={home} disabled={busy}>
          {inkshelfVisible ? 'Press Home' : 'Switch back'}
        </Button>
        <Button onClick={() => systemEvent(163, 'USB', 'USBStorageIn', 'it saves the reading position before the computer takes the storage.')} disabled={busy}>
          Plug USB
        </Button>
        <Button onClick={() => systemEvent(256, 'Wi-Fi', 'NetConnected', 'it sends the progress read offline.')} disabled={busy}>
          Wi-Fi up
        </Button>
        <Button onClick={download} disabled={busy}>
          Download (keep awake)
        </Button>
        <Button onClick={syncEvent} disabled={busy}>
          Save state (sync)
        </Button>
      </Controls>
      <div ref={logBox} className={styles.log} aria-live="polite">
        {log.map((line, index) => (
          <div key={index} className={index === log.length - 1 ? styles.logNew : styles.logLine}>
            {line}
          </div>
        ))}
      </div>
    </div>
  );
}
