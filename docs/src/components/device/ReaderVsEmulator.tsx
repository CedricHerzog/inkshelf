// What changes when the app runs in the emulator instead of on the reader: only what is behind
// the firmware interface (firmware.go). readerFirmware (firmware_linux_arm.go) becomes Emulator
// (emulated_firmware.go), the panel an image served by cmd/emulator/server.go, the finger a
// browser click. The bytes the event loop decodes stay the same (encodeHardwareEvent, rawTouch).

import React, {useState} from 'react';
import {Segmented} from '@site/src/components/kit';
import {EVENT, hardwareEvent, hexBytes, rawTouch} from '@site/src/lib/firmwareMessages';
import styles from './device.module.css';

type Where = 'reader' | 'emulator';

type Swap = {title: string; reader: [string, string]; emulator: [string, string]};

const kept: [string, string][] = [
  ['internal/app', 'the screens, the state, the servers: the same code'],
  ['event loop', 'event_loop.go, touch.go, keys.go: the same code'],
  ['firmware interface', 'flush, visible, nextHardwareEvent, nextTaskMessage, nextReply, send, battery'],
];

const swaps: Swap[] = [
  {
    title: 'Behind the interface',
    reader: ['readerFirmware', 'firmware_linux_arm.go: syscalls on the real firmware’s queues and shared memory'],
    emulator: ['Emulator', 'emulated_firmware.go: the same queues as Go slices, under a mutex'],
  },
  {
    title: 'The panel',
    reader: ['/dev/fb0 + epdc.ko', 'copyRotated, then the MXCFB ioctl; a GL16 page turn takes about 490 ms'],
    emulator: ['an image in memory', 'flush rounds to the waveform’s grays; GET /panel.png, refreshes streamed on /refreshes; -eink sleeps 600/490/265 ms'],
  },
  {
    title: 'A touch',
    reader: ['a finger on the glass', 'the firmware reads /dev/input and posts 32 bytes on /hwevent'],
    emulator: ['a click in the browser', 'POST /pointer → PointerDown → rawTouch → the same 32 bytes'],
  },
  {
    title: 'Buttons',
    reader: ['Back, Forward, Menu', 'key press and release messages on /hwevent'],
    emulator: ['[ ] m Enter Backspace', 'POST /key → PressKey, ReleaseKey'],
  },
  {
    title: 'Home, Wi-Fi, USB',
    reader: ['the firmware’s task events', 'on the SysV task queue, with the whiteboard for who is in front'],
    emulator: ['the side panel’s buttons', 'POST /control → SetForeground, SendSystem, SetBattery, RequestExit'],
  },
];

function Face({face, shown}: {face: [string, string]; shown: boolean}) {
  return (
    <div
      style={{
        gridArea: '1 / 1',
        opacity: shown ? 1 : 0,
        transform: shown ? 'none' : 'translateY(6px) scale(0.98)',
        transition: 'opacity 0.35s, transform 0.35s',
        pointerEvents: shown ? 'auto' : 'none',
      }}
      aria-hidden={!shown}>
      <div className={styles.stageTitle + ' ' + styles.mono}>{face[0]}</div>
      <div className={styles.stageDetail}>{face[1]}</div>
    </div>
  );
}

export default function ReaderVsEmulator() {
  const [where, setWhere] = useState<Where>('reader');
  const tap = {x: 700, y: 1048};
  const raw = rawTouch(tap.x, tap.y);
  const bytes = hexBytes(hardwareEvent(EVENT.pointerDown, raw.rawX, raw.rawY));
  return (
    <div className={styles.stack}>
      <Segmented
        options={[
          {value: 'reader', label: 'On the reader'},
          {value: 'emulator', label: 'In the emulator'},
        ]}
        value={where}
        onChange={setWhere}
      />
      <div className={styles.label}>Stays the same</div>
      <div className={styles.facts}>
        {kept.map(([name, what]) => (
          <div key={name} className={styles.stage + ' ' + styles.stageGood}>
            <div className={styles.stageTitle}>{name}</div>
            <div className={styles.stageDetail}>{what}</div>
          </div>
        ))}
      </div>
      <div className={styles.label}>Swapped</div>
      <div className={styles.stack} style={{gap: '0.5rem'}}>
        {swaps.map((swap) => (
          <div key={swap.title} className={styles.row} style={{flexWrap: 'nowrap', alignItems: 'stretch'}}>
            <div className={styles.label} style={{width: '6.5rem', flex: 'none', paddingTop: '0.6rem'}}>
              {swap.title}
            </div>
            <div
              className={styles.stage + ' ' + (where === 'reader' ? styles.stagePain : styles.stageLit)}
              style={{display: 'grid', flex: 1}}>
              <Face face={swap.reader} shown={where === 'reader'} />
              <Face face={swap.emulator} shown={where === 'emulator'} />
            </div>
          </div>
        ))}
      </div>
      <div className={styles.label}>A tap at (700, 1048), as the event loop receives it</div>
      <div className={styles.code}>
        {`${bytes.join(' | ')}\n`}
        {`type 30 (down) | finger 0 | raw x ${raw.rawX} | raw y ${raw.rawY} | pressure | tool | time (unused)`}
        {`\n${where === 'reader' ? '↑ written by the firmware' : '↑ written by encodeHardwareEvent, in the emulator'}: the same layout either way, and the loop reads only the first four numbers.`}
      </div>
    </div>
  );
}
