// The life of a tap, step by step: a mock book list on the left, whose buttons are real
// targets; tapping one plays its path through the event loop, the app and, for a download, a
// goroutine, with the state's mutex closing and opening as it does in internal/app/input.go
// (Pointer, touchTargetLocked, run, command), internal/app/state/state.go (Update) and
// internal/app/app.go (Draw).

import React, {useEffect, useState} from 'react';
import clsx from 'clsx';
import {
  Controls,
  Narration,
  StepperBar,
  Toggle,
  usePrefersReducedMotion,
  useStepper,
} from '@site/src/components/kit';
import styles from './app.module.css';

type Lane = 'loop' | 'app' | 'goroutine';
type LockHolder = 'free' | 'main' | 'goroutine';
type Panel = 'list' | 'list2' | 'series' | 'queued' | 'progress';
type Scenario = 'next' | 'back' | 'download';
type TargetId = 'back' | 'next' | 'tome2';

type Step = {
  lane: Lane;
  code: string;
  say: string;
  lock: LockHolder;
  /** The target the touch hit, highlighted on the mock screen. */
  hit?: boolean;
  /** Whether the targets exist (Draw clears them, DrawLocked registers them again). */
  targets?: boolean;
  /** What the panel shows from this step on. */
  panel?: Panel;
  flash?: boolean;
};

const touchSteps = (target: string): Step[] => [
  {
    lane: 'loop',
    code: 'Pointer(PointerDown)',
    say: 'A finger touches the panel. The kernel wakes the event loop, which reads the touch and calls app.Pointer. A PointerDown only records where the touch started (touchStart).',
    lock: 'free',
  },
  {
    lane: 'loop',
    code: 'Pointer(PointerUp)',
    say: 'The finger lifts. Now the app decides what the touch was.',
    lock: 'free',
  },
  {
    lane: 'app',
    code: 'app.Lock()',
    say: 'It takes the state’s mutex: nobody may change the screen or its targets while it looks at them.',
    lock: 'main',
  },
  {
    lane: 'app',
    code: 'touchTargetLocked(start, end)',
    say: `Does this screen decide touches itself (touchable)? The book list does not. Was it a swipe? No. So Targets.Hit returns the first target the touch started and ended in: ${target}.`,
    lock: 'main',
    hit: true,
  },
  {
    lane: 'app',
    code: 'app.Unlock()',
    say: 'The mutex is released before anything happens: what the target does takes the mutex itself, and taking it twice would freeze the app.',
    lock: 'free',
    hit: true,
  },
];

const repaintSteps = (fullRefresh: boolean, panel: Panel, what: string): Step[] => [
  {
    lane: 'loop',
    code: 'case <-repaints: app.Draw()',
    say: 'Pointer returns. On its next turn, the event loop finds the token in its repaints channel and calls Draw.',
    lock: 'free',
  },
  {
    lane: 'loop',
    code: 'Draw: Lock(); Targets.Clear()',
    say: 'Draw takes the mutex, clears Repaint.Queued and forgets every target: they described the screen that is going away.',
    lock: 'main',
    targets: false,
  },
  {
    lane: 'loop',
    code: 'ClearScreen(); screen.DrawLocked()',
    say: `The screen shown draws itself whole into the canvas (${what}), and registers a target for each button as it draws it. The panel still shows the old picture.`,
    lock: 'main',
    targets: true,
  },
  fullRefresh
    ? {
        lane: 'loop',
        code: 'FlashRefresh(); Unlock()',
        say: 'Update asked for a full refresh: the panel flashes and shows the new screen, clean. Draw returns and the mutex is free again.',
        lock: 'free',
        panel,
        flash: true,
      }
    : {
        lane: 'loop',
        code: 'Refresh(Screen(), GC16); Unlock()',
        say: 'A partial refresh: the panel shows the new picture without a flash. Draw returns and the mutex is free again.',
        lock: 'free',
        panel,
      },
];

const overlaySteps: Step[] = [
  {
    lane: 'app',
    code: 'Update(false, CloseOverlaysLocked)',
    say: 'First, any tap on a list closes its hold menu. That is an Update too: lock, change, unlock. No menu was open, so the change returns false and nothing is repainted.',
    lock: 'main',
  },
];

const scenarios: Record<Scenario, Step[]> = {
  next: [
    ...touchSteps('the footer’s “›”, a target whose Command is Next'),
    {
      lane: 'app',
      code: 'run(tapped) → command(Next)',
      say: 'The target carries a Command, so run hands it to command, as if the reader’s Next button had been pressed. The mutex is free.',
      lock: 'free',
    },
    ...overlaySteps,
    {
      lane: 'app',
      code: 'screen.Command(Next)',
      say: 'The book list’s Command turns its page. It does not touch the state itself: it calls state.Update.',
      lock: 'free',
    },
    {
      lane: 'app',
      code: 'Update(false, turnLocked) → true',
      say: 'Update takes the mutex and runs the change: turnLocked moves the list to its next page and returns true, “something visible changed”.',
      lock: 'main',
    },
    {
      lane: 'app',
      code: 'Queued = true; Unlock(); Repaint()',
      say: 'Repaint.Queued was false, so this Update asks for a repaint: pocketbook.Repaint drops a token in a channel. Nothing is drawn yet.',
      lock: 'free',
    },
    ...repaintSteps(false, 'list2', 'page 2 of the list'),
  ],
  back: [
    ...touchSteps('the header’s “‹ Back”, a target whose Command is Back'),
    {
      lane: 'app',
      code: 'run(tapped) → command(Back)',
      say: 'The target carries a Command, Back: run hands it to command, as the reader’s Back button would.',
      lock: 'free',
    },
    ...overlaySteps,
    {
      lane: 'app',
      code: 'screen.Command(Back) → backToSeries',
      say: 'The book list goes back to the series. It never imports the series list’s package: it only says which screen to show.',
      lock: 'free',
    },
    {
      lane: 'app',
      code: 'Update(true, … ShowLocked(SeriesScreen))',
      say: 'Inside Update, with the mutex held, state.ShowLocked(SeriesScreen) switches the screen and drops its targets. fullRefresh is true: a whole new screen deserves a clean flash.',
      lock: 'main',
      targets: false,
    },
    {
      lane: 'app',
      code: 'Queued = true; Unlock(); Repaint()',
      say: 'The change returned true and no repaint was queued: one token goes to the event loop.',
      lock: 'free',
      targets: false,
    },
    ...repaintSteps(true, 'series', 'the series list, now the screen shown'),
  ],
  download: [
    ...touchSteps('the row of Tome 2, a target with a Tap closure'),
    {
      lane: 'app',
      code: 'run(tapped)',
      say: 'This target has no Command but a Tap closure. run will call it, with the mutex free.',
      lock: 'free',
    },
    ...overlaySteps,
    {
      lane: 'app',
      code: 'tapped.Tap() → screen.open(1)',
      say: 'The closure the row registered while it was drawn: open book 1 of the list, or download it.',
      lock: 'free',
    },
    {
      lane: 'app',
      code: 'Update(false, … EnqueueLocked)',
      say: 'Inside Update, the book is not on the reader: downloadqueue.EnqueueLocked puts it in the queue and, the queue being idle, starts the downloader with state.Go. Only the main thread starts goroutines.',
      lock: 'main',
    },
    {
      lane: 'goroutine',
      code: 'go downloader(…) → Store.Download',
      say: 'A new goroutine starts downloading. It never draws and never calls the app: it is on its own, with no lock.',
      lock: 'main',
    },
    {
      lane: 'app',
      code: 'Queued = true; Unlock(); Repaint()',
      say: 'The change returned true: one repaint is asked for. The tap is over; the main thread is free for the next touch while the download goes on.',
      lock: 'free',
    },
    ...repaintSteps(false, 'queued', 'Tome 2 now says “Downloading”'),
    {
      lane: 'goroutine',
      code: 'progress → Update(false, noteProgressLocked)',
      say: 'Pages arrive. Each progress report is an Update: the goroutine takes the mutex for a moment and records how far it is. noteProgressLocked returns true for the first page, then at most every 2 s, so the panel is not refreshed for every page.',
      lock: 'goroutine',
    },
    {
      lane: 'goroutine',
      code: 'Unlock(); Repaint()',
      say: 'The goroutine asks for a repaint, and goes back to downloading. It never draws: drawing is the event loop’s job.',
      lock: 'free',
    },
    ...repaintSteps(false, 'progress', 'Tome 2 now says “Downloading 3 of 24”'),
  ],
};

const scenarioOf: Record<TargetId, Scenario> = {back: 'back', next: 'next', tome2: 'download'};
const targetOf: Record<Scenario, TargetId> = {back: 'back', next: 'next', download: 'tome2'};

const lanes: {id: Lane; name: string; note: string}[] = [
  {id: 'loop', name: 'Event loop', note: 'main thread'},
  {id: 'app', name: 'App & screen', note: 'main thread'},
  {id: 'goroutine', name: 'Downloader', note: 'a goroutine'},
];

function lockLabel(holder: LockHolder): string {
  switch (holder) {
    case 'main':
      return '🔒 held by the main thread';
    case 'goroutine':
      return '🔒 held by the downloader';
    default:
      return '🔓 free';
  }
}

function MockReader({
  panel,
  hit,
  targetsShown,
  flash,
  onTap,
}: {
  panel: Panel;
  hit: TargetId | null;
  targetsShown: boolean;
  flash: boolean;
  onTap: (target: TargetId) => void;
}) {
  const target = (id: TargetId, children: React.ReactNode, disabled = false) => (
    <button
      type="button"
      className={clsx(styles.target, hit === id && styles.targetHit)}
      disabled={disabled}
      onClick={() => onTap(id)}>
      {children}
    </button>
  );
  if (panel === 'series') {
    return (
      <div className={clsx(styles.reader, targetsShown && styles.targetsShown, flash && styles.readerFlash)}>
        <div className={styles.readerHeader}>
          <span>Komga</span>
          <span>12:04</span>
        </div>
        {['Astérix', 'Blacksad', 'Lucky Luke'].map((title) => (
          <div key={title} className={styles.readerRow}>
            <span className={styles.readerCover} />
            <span>
              <strong>{title}</strong>
              <br />
              <small>Series</small>
            </span>
          </div>
        ))}
        <div className={styles.readerFooter}>
          <span>‹</span>
          <span>1/4</span>
          <span>›</span>
        </div>
        <div style={{padding: '0.3rem', fontSize: '0.7rem'}}>(series list: replay to go back)</div>
      </div>
    );
  }
  const page2 = panel === 'list2';
  const tome2 =
    panel === 'queued'
      ? 'Downloading (tap to cancel)'
      : panel === 'progress'
        ? 'Downloading 3 of 24'
        : 'Tap to download';
  return (
    <div className={clsx(styles.reader, targetsShown && styles.targetsShown, flash && styles.readerFlash)}>
      <div className={styles.readerHeader}>
        {target('back', '‹ Back')}
        <span>Astérix</span>
      </div>
      {page2 ? (
        ['Tome 7', 'Tome 8'].map((title) => (
          <div key={title} className={styles.readerRow}>
            <span className={styles.readerCover} />
            <span>
              <strong>{title}</strong>
              <br />
              <small>Tap to download</small>
            </span>
          </div>
        ))
      ) : (
        <>
          <div className={styles.readerRow}>
            <span className={styles.readerCover} />
            <span>
              <strong>Tome 1</strong>
              <br />
              <small>Downloaded · page 12</small>
            </span>
          </div>
          <div className={styles.readerRow}>
            <span className={styles.readerCover} />
            {target(
              'tome2',
              <span>
                <strong>Tome 2</strong>
                <br />
                <small>{tome2}</small>
              </span>,
            )}
          </div>
        </>
      )}
      <div className={styles.readerFooter}>
        <span>‹</span>
        <span>{page2 ? '2/3' : '1/3'}</span>
        {target('next', '›')}
      </div>
    </div>
  );
}

function Run({
  scenario,
  autoplay,
  targetsShown,
  onTap,
}: {
  scenario: Scenario;
  autoplay: boolean;
  targetsShown: boolean;
  onTap: (target: TargetId) => void;
}) {
  const steps = scenarios[scenario];
  const stepper = useStepper(steps.length, 1500);
  useEffect(() => {
    if (autoplay) stepper.play();
    // Only when this run starts.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const step = steps[stepper.index];
  let panel: Panel = 'list';
  let targets = true;
  for (const past of steps.slice(0, stepper.index + 1)) {
    if (past.panel) panel = past.panel;
    if (past.targets !== undefined) targets = past.targets;
  }
  const hit = step.hit ? targetOf[scenario] : null;
  return (
    <>
      <div className={styles.row}>
        <MockReader
          panel={panel}
          hit={hit}
          targetsShown={targetsShown && targets}
          flash={Boolean(step.flash)}
          onTap={onTap}
        />
        <div className={styles.grow}>
          <div style={{marginBottom: '0.5rem'}}>
            <span
              className={clsx(styles.lock, step.lock === 'free' ? styles.lockFree : styles.lockHeld)}>
              state mutex: {lockLabel(step.lock)}
            </span>
          </div>
          <div className={styles.lanes}>
            {lanes.map((lane) => (
              <React.Fragment key={lane.id}>
                <div className={styles.laneName}>
                  {lane.name}
                  <small>{lane.note}</small>
                </div>
                <div className={styles.laneBody}>
                  {steps.map((candidate, index) =>
                    candidate.lane !== lane.id || index > stepper.index ? null : (
                      <span
                        key={index}
                        className={clsx(
                          styles.chip,
                          index === stepper.index ? styles.chipNow : styles.chipPast,
                        )}>
                        {index + 1}. {candidate.code}
                      </span>
                    ),
                  )}
                </div>
              </React.Fragment>
            ))}
          </div>
        </div>
      </div>
      <StepperBar stepper={stepper} />
      <Narration>{step.say}</Narration>
    </>
  );
}

export default function TapPath() {
  const reduced = usePrefersReducedMotion();
  const [scenario, setScenario] = useState<Scenario>('next');
  const [run, setRun] = useState(0);
  const [targetsShown, setTargetsShown] = useState(true);
  return (
    <div>
      <Controls>
        <Toggle label="Show the targets" checked={targetsShown} onChange={setTargetsShown} />
        <span className={styles.muted}>Tap “›”, “‹ Back” or Tome 2 on the reader.</span>
      </Controls>
      <Run
        key={`${scenario}-${run}`}
        scenario={scenario}
        autoplay={run > 0 && !reduced}
        targetsShown={targetsShown}
        onTap={(target) => {
          setScenario(scenarioOf[target]);
          setRun((count) => count + 1);
        }}
      />
    </div>
  );
}
