// Two ways to build a program for the reader: through PocketBook's SDK (cgo, a C cross-compiler,
// libinkview) or as Inkshelf does, with one plain go build (Makefile, target build).

import React, {useState} from 'react';
import clsx from 'clsx';
import {Segmented, StepperBar, Narration, useStepper} from '@site/src/components/kit';
import styles from './device.module.css';

type Way = 'sdk' | 'go';

type Stage = {title: string; detail: string; kind?: 'pain' | 'good'; narration: string};

const stages: Record<Way, Stage[]> = {
  sdk: [
    {
      title: 'Go code that calls C',
      detail: 'import "C", then C.OpenScreen(), C.FullUpdate()…',
      narration:
        'To call the SDK, Go code must call C functions. That is cgo: Go’s bridge to C.',
    },
    {
      title: 'cgo hands the C parts to a C compiler',
      detail: 'CGO_ENABLED=1, CC=<the SDK’s ARM gcc>',
      kind: 'pain',
      narration:
        'Go cannot compile C. cgo calls a C compiler, and it must be a cross-compiler that makes ARM code: the one shipped in the SDK, with the reader’s C headers and libraries.',
    },
    {
      title: 'Link with libinkview',
      detail: 'the SDK’s C library: screen, buttons, firmware',
      kind: 'pain',
      narration:
        'The program is linked with libinkview, the C library that draws, reads the buttons and talks to the firmware. It stays a separate file that must be on the reader.',
    },
    {
      title: 'A dynamically linked ARM program',
      detail: 'needs libinkview.so and the C library on the reader',
      kind: 'pain',
      narration:
        'The result works only next to the right versions of those libraries. Every build needs the whole toolchain installed.',
    },
  ],
  go: [
    {
      title: 'Go code that talks to the system itself',
      detail: 'package syscall and unsafe: open, ioctl, mmap, msgrcv…',
      narration:
        'Inkshelf does what libinkview does in Go: it opens /dev/fb0, maps it and calls the kernel directly (internal/device/pocketbook).',
    },
    {
      title: 'One command, on any computer',
      detail: 'CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build',
      kind: 'good',
      narration:
        'Go’s own compiler makes ARM code from any computer. GOOS and GOARCH name the target, GOARM=7 the reader’s Cortex-A7, and CGO_ENABLED=0 promises there is no C at all.',
    },
    {
      title: 'Nothing to link but Go',
      detail: '-trimpath -ldflags="-s -w": stripped, no build paths',
      kind: 'good',
      narration:
        'Without C there is no system library to link against: Go puts its runtime and every package into the one file. -s -w strips the symbol tables to make it smaller.',
    },
    {
      title: 'build/inkshelf.app: one static ARM ELF',
      detail: 'copy it to applications/ on the reader, done',
      kind: 'good',
      narration:
        'The .app is an ordinary Linux executable for ARM (ELF), statically linked. It runs on the reader as it is: no SDK, no libraries, no installer.',
    },
  ],
};

const facts: Record<Way, [string, string][]> = {
  sdk: [
    ['To install', 'Go, the SDK, its cross gcc'],
    ['C in the build', 'yes (cgo)'],
    ['Output', 'dynamic ELF + libinkview'],
  ],
  go: [
    ['To install', 'Go'],
    ['C in the build', 'none (CGO_ENABLED=0)'],
    ['Output', 'one static ELF'],
  ],
};

export default function BuildPipeline() {
  const [way, setWay] = useState<Way>('go');
  const list = stages[way];
  const stepper = useStepper(list.length, 1400);
  const index = stepper.index;
  return (
    <div className={styles.stack}>
      <Segmented
        options={[
          {value: 'sdk', label: 'With the SDK (cgo)'},
          {value: 'go', label: 'Pure Go (Inkshelf)'},
        ]}
        value={way}
        onChange={(value) => {
          setWay(value);
          stepper.reset();
        }}
      />
      <div className={styles.pipeline}>
        {list.map((stage, stageIndex) => (
          <React.Fragment key={`${way}-${stage.title}`}>
            {stageIndex > 0 && <div className={styles.arrow}>↓</div>}
            <div
              className={clsx(
                styles.stage,
                stage.kind === 'pain' && styles.stagePain,
                stage.kind === 'good' && styles.stageGood,
                stageIndex === index && styles.stageLit,
              )}>
              <div className={styles.stageTitle}>{stage.title}</div>
              <div className={clsx(styles.stageDetail, styles.mono)}>{stage.detail}</div>
            </div>
          </React.Fragment>
        ))}
      </div>
      <StepperBar stepper={stepper} label="stage" />
      <Narration>{list[index].narration}</Narration>
      <div className={styles.facts}>
        {facts[way].map(([name, value]) => (
          <div key={name} className={styles.fact}>
            <strong>{name}</strong>
            {value}
          </div>
        ))}
      </div>
    </div>
  );
}
