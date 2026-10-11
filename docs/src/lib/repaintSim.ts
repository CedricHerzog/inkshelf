// A millisecond-by-millisecond model of how many redraws a burst of state changes costs. It
// follows state.Update (internal/app/state/state.go), pocketbook.Repaint and the event loop's
// `case <-repaints: app.Draw()` (internal/device/pocketbook/app.go), and app.Draw
// (internal/app/app.go):
//
//   - Update takes the mutex, runs change, and if it returns true sets Repaint.Queued; only the
//     Update that finds Queued false calls pocketbook.Repaint.
//   - Repaint puts a token in a channel that holds one; a second token would be dropped.
//   - When the event loop is free, it takes the token and calls Draw, which holds the mutex
//     the whole time and clears Queued as it starts. An Update arriving meanwhile waits for the
//     mutex, then queues one more Draw.

export type UpdateCall = {
  /** When the goroutine calls Update, in ms. */
  at: number;
  /** Which goroutine (an index into the widget's list). */
  goroutine: number;
};

export type UpdateOutcome = UpdateCall & {
  /** When it got the mutex: later than `at` if a Draw held it. */
  locked: number;
  /** Whether it was the one to call pocketbook.Repaint (Queued was false). */
  repainted: boolean;
  /** The Draw that shows this change (an index into draws). */
  drawIndex: number;
};

export type DrawSpan = {start: number; end: number};

export type RepaintRun = {
  updates: UpdateOutcome[];
  draws: DrawSpan[];
  /** Repaint() calls, each one token into the channel. */
  repaintCalls: number;
  /** When everything is over. */
  end: number;
};

export function simulateRepaints(
  calls: readonly UpdateCall[],
  options: {loopBusyUntil: number; drawMs: number},
): RepaintRun {
  const sorted = [...calls].sort((left, right) => left.at - right.at);
  const outcomes: UpdateOutcome[] = [];
  const draws: DrawSpan[] = [];
  let queued = false; // Repaint.Queued
  let token = false; // the repaints channel, which holds one
  let repaintCalls = 0;
  let drawing: DrawSpan | null = null;
  let next = 0;
  const horizon = 20000;
  for (let now = 0; now < horizon; now++) {
    if (drawing && now >= drawing.end) drawing = null; // Draw returns: Unlock
    // Updates waiting for the mutex run once nobody holds it, in arrival order.
    while (!drawing && next < sorted.length && sorted[next].at <= now) {
      const call = sorted[next++];
      const repainted = !queued;
      queued = true;
      if (repainted) {
        repaintCalls++;
        token = true;
      }
      outcomes.push({...call, locked: now, repainted, drawIndex: -1});
    }
    // The event loop, when free, takes the token and calls Draw.
    if (!drawing && token && now >= options.loopBusyUntil) {
      token = false;
      queued = false; // Draw clears Repaint.Queued as it starts, with the mutex held
      drawing = {start: now, end: now + options.drawMs};
      draws.push(drawing);
      // Every change made so far is in this Draw.
      for (const outcome of outcomes) {
        if (outcome.drawIndex < 0) outcome.drawIndex = draws.length - 1;
      }
    }
    if (next >= sorted.length && !token && !drawing) {
      return {updates: outcomes, draws, repaintCalls, end: now};
    }
  }
  return {updates: outcomes, draws, repaintCalls, end: horizon};
}
