// What `make verify` runs, one stage per step (the Makefile's verify target): each stage lights
// up in a pipeline, with what it catches. A toggle breaks a shot to show how shots-compare
// fails.

import React, {useState} from 'react';
import clsx from 'clsx';
import {Controls, Narration, StepperBar, Toggle, useStepper} from '@site/src/components/kit';
import styles from './VerifyPipeline.module.css';

type Stage = {
  name: string;
  command: string;
  catches: string;
};

const stages: readonly Stage[] = [
  {
    name: 'build',
    command: 'CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags="-s -w" -o build/inkshelf.app ./cmd/inkshelf',
    catches: 'The program for the reader still cross-compiles: ARM, static, no cgo.',
  },
  {
    name: 'lint (host)',
    command: 'golangci-lint run ./...',
    catches: 'Suspicious code on your computer: a wrong Printf verb, an unchecked error, a copied lock, dead code.',
  },
  {
    name: 'lint (reader)',
    command: 'GOOS=linux GOARCH=arm golangci-lint run ./...',
    catches: 'The same, for the files that only build for the reader (the ones ending in _linux_arm.go).',
  },
  {
    name: 'test',
    command: 'go test ./...',
    catches: 'Every unit test, including the golden tests that keep the files on readers readable.',
  },
  {
    name: 'shots-all',
    command: 'build/emulator -fake -fresh -frozen-status -script emulator/scripts/<script>.txt -out build/shots/<script> …',
    catches: 'Every emulator script runs against its fake servers and saves its screenshots. The status (clock, free space) is frozen so two runs give the same pixels.',
  },
  {
    name: 'shots-compare',
    command: 'diff -r build/baseline/shots build/shots',
    catches: 'Lists every screenshot that differs from the baseline. Each difference must be one you meant.',
  },
];

export default function VerifyPipeline() {
  const stepper = useStepper(stages.length + 1, 1100);
  const [broken, setBroken] = useState(false);
  const done = stepper.index; // stages before this index have passed
  const failedAt = broken ? stages.length - 1 : -1;
  const finished = stepper.index === stages.length;

  let narration: React.ReactNode;
  if (stepper.index === 0) {
    narration = (
      <>
        <strong>{stages[0].name}.</strong> {stages[0].catches}
      </>
    );
  } else if (finished) {
    narration = broken ? (
      <>
        <strong>✗ make verify fails.</strong> <code>diff</code> printed{' '}
        <code>Binary files build/baseline/shots/mealie/04-page-1.png and build/shots/mealie/04-page-1.png differ</code>.
        Open both. If the change is the one you meant, accept it with <code>make baseline</code>; if not, fix the code.
      </>
    ) : (
      <>
        <strong>✓ make verify passes.</strong> Built, linted twice, tested, and every screenshot identical to the
        baseline: the change is ready to commit.
      </>
    );
  } else {
    const stage = stages[stepper.index];
    narration = (
      <>
        <strong>{stage.name}.</strong> {stage.catches}
      </>
    );
  }

  return (
    <div className={styles.widget}>
      <Controls>
        <Toggle label="My change moved a pixel of the Mealie recipe" checked={broken} onChange={setBroken} />
      </Controls>
      <ol className={styles.pipeline}>
        {stages.map((stage, index) => {
          const state =
            index === failedAt && finished
              ? 'failed'
              : index < done || (finished && index !== failedAt)
                ? 'passed'
                : index === done
                  ? 'running'
                  : 'waiting';
          return (
            <li
              key={stage.name}
              className={clsx(styles.stage, styles[state])}
              onClick={() => stepper.seek(index)}>
              <span className={styles.mark}>
                {state === 'passed' ? '✓' : state === 'failed' ? '✗' : state === 'running' ? '…' : index + 1}
              </span>
              <span className={styles.name}>{stage.name}</span>
            </li>
          );
        })}
      </ol>
      <pre className={styles.terminal}>
        <code>
          $ make verify{'\n'}
          {stages
            .slice(0, Math.min(done + 1, stages.length))
            .map((stage) => stage.command)
            .join('\n')}
        </code>
      </pre>
      <StepperBar stepper={stepper} label="stage" />
      <Narration>{narration}</Narration>
    </div>
  );
}
