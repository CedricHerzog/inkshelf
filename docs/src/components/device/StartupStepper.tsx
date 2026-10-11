// What happens between a tap on Inkshelf's icon and its first screen: main, setupLogging,
// app.New, pocketbook.Run, openFirmware (framebuffer_linux_arm.go, firmware_linux_arm.go)
// and run (app.go).

import React from 'react';
import clsx from 'clsx';
import {StepperBar, Narration, useStepper} from '@site/src/components/kit';
import styles from './device.module.css';

type Resource = {id: string; name: string; what: string};

const resources: Resource[] = [
  {id: 'log', name: 'inkshelf.log', what: 'stdout, stderr and the log'},
  {id: 'app', name: 'the App', what: 'state and screens, not drawn yet'},
  {id: 'fb', name: '/dev/fb0', what: 'framebuffer, mapped'},
  {id: 'global', name: 'shm 0xa12302', what: 'firmware’s global state'},
  {id: 'tasks', name: 'msg queue 0xa1230f', what: 'task messages'},
  {id: 'hwevent', name: '/hwevent', what: 'input queue'},
  {id: 'canvas', name: 'task canvas', what: '1404×1872 shm'},
  {id: 'loop', name: 'event loop', what: 'ticks every 20 ms'},
];

type Step = {
  where: string;
  code: string;
  opens?: string;
  text: React.ReactNode;
};

const steps: Step[] = [
  {
    where: 'the launcher',
    code: '/mnt/ext1/applications/inkshelf.app',
    text: 'You tap the icon. The firmware’s launcher starts the program file, as a new task in its own process group.',
  },
  {
    where: 'cmd/inkshelf/logging.go',
    code: 'setupLogging()',
    opens: 'log',
    text: 'First, the log: /mnt/ext1/applications/inkshelf.log, emptied when over 1 MiB. stdout and stderr are pointed at it too (Dup2), so even a crash leaves its trace there, readable over USB.',
  },
  {
    where: 'cmd/inkshelf/main.go',
    code: 'app.New(appDir, registry.Opener(…), registry.KindOf, app.Options{})',
    opens: 'app',
    text: 'The app is built: its state and its screens, with its folder /mnt/ext1/applications/inkshelf. Nothing is drawn yet: there is no screen to draw on.',
  },
  {
    where: 'internal/device/pocketbook/app.go',
    code: 'pocketbook.Run(application)  →  openFirmware()',
    text: 'Run takes over. Its first job is openFirmware: open everything the reader shares with apps.',
  },
  {
    where: 'framebuffer_linux_arm.go',
    code: 'open("/dev/fb0")\nioctl(FBIOGET_VSCREENINFO 0x4600)  // 1872×1404, 8 bits\nioctl(FBIOGET_FSCREENINFO 0x4602)  // bytes per row, size',
    text: 'It opens the framebuffer device and asks the driver, with two ioctls, how big it is: 1872×1404 pixels, one byte each. It is landscape: the panel is mounted sideways.',
  },
  {
    where: 'framebuffer_linux_arm.go',
    code: 'syscall.Mmap(fd, 0, size, PROT_READ|PROT_WRITE, MAP_SHARED)',
    opens: 'fb',
    text: 'mmap turns the framebuffer into an ordinary Go []byte: writing a byte there writes a pixel’s memory. If the framebuffer cannot be opened or mapped, Run gives up: without a screen there is no app.',
  },
  {
    where: 'firmware_linux_arm.go',
    code: 'attachShm(globalShmKey 0xa12302, 0x44d8 bytes, readOnly)',
    opens: 'global',
    text: 'The firmware’s global state: shared memory found by a number every process agrees on. The app only reads it, to know who is in front.',
  },
  {
    where: 'firmware_linux_arm.go',
    code: 'msgget(taskQueueKey 0xa1230f)',
    opens: 'tasks',
    text: 'The task queue, where the firmware posts news for each app and answers requests.',
  },
  {
    where: 'firmware_linux_arm.go',
    code: 'mqOpen("hwevent", O_RDONLY|O_NONBLOCK)',
    opens: 'hwevent',
    text: 'The input queue: button presses and touches. Non-blocking: reading an empty queue returns at once instead of waiting. If any of these three is missing, the app logs it and runs anyway.',
  },
  {
    where: 'firmware_linux_arm.go',
    code: 'taskCanvas(pgid, 1404×1872)\n// reads /var/run/task/<pgid>/fbshmkey and fbshmsize',
    opens: 'canvas',
    text: 'The canvas: the firmware made one for this task, in shared memory, so its task switcher can show a thumbnail. Two small files in the task’s folder give its key and size. The header must say 1404×1872 at 8 bits per pixel.',
  },
  {
    where: 'internal/device/pocketbook/app.go',
    code: 'loop.dropStaleEvents()\nsignal.Notify(SIGTERM, SIGINT, SIGHUP, SIGQUIT)\napp.Init()',
    text: 'run empties /hwevent of input meant for the app that was in front before, listens for the signals the firmware stops apps with, then calls app.Init, which draws the first screen.',
  },
  {
    where: 'internal/device/pocketbook/app.go',
    code: 'time.NewTicker(pollInterval)  // 20 ms\nfor { select { … } }',
    opens: 'loop',
    text: 'Last, the event loop starts: every 20 ms it reads the queues. From now on, every call into the app comes from this one goroutine.',
  },
];

export default function StartupStepper() {
  const stepper = useStepper(steps.length, 1600);
  const step = steps[stepper.index];
  const opened = new Set(
    steps.slice(0, stepper.index + 1).flatMap((candidate) => (candidate.opens ? [candidate.opens] : [])),
  );
  return (
    <div className={styles.stack}>
      <div className={styles.facts}>
        {resources.map((resource) => (
          <div
            key={resource.id}
            className={clsx(
              styles.stage,
              opened.has(resource.id) && styles.stageGood,
              step.opens === resource.id && styles.stageLit,
            )}
            style={{opacity: opened.has(resource.id) ? 1 : 0.45}}>
            <div className={clsx(styles.stageTitle, styles.mono)}>{resource.name}</div>
            <div className={styles.stageDetail}>{resource.what}</div>
          </div>
        ))}
      </div>
      <div>
        <div className={styles.label}>{step.where}</div>
        <div className={styles.code}>{step.code}</div>
      </div>
      <StepperBar stepper={stepper} label="step" />
      <Narration>{step.text}</Narration>
    </div>
  );
}
