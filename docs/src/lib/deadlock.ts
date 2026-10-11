// Two goroutines, each taking some locks in an order, doing their work, then releasing them
// all. explore() looks at every way a scheduler may interleave their steps and returns a run
// that deadlocks if one exists (the worst scheduler wins), else a run that completes. It is a
// model of the lock order of internal/app (docs/reference/app-threading.md, "Lock order"), not code from
// the app: the app avoids the deadlock by always taking the locks in one order.

export type LockId = 'S' | 'M' | 'R';

export const lockNames: Record<LockId, string> = {
  S: 'Syncer lock',
  M: 'state mutex',
  R: 'renderer lock',
};

/** The order a goroutine takes its locks in. */
export type Program = readonly LockId[];

export type Frame = {
  /** How many locks each goroutine holds (its position in its program). */
  taken: [number, number];
  finished: [boolean, boolean];
  holder: Record<LockId, 0 | 1 | null>;
  /** Who moved to reach this frame (null for the first). */
  actor: 0 | 1 | null;
  /** What happened, in a sentence. */
  action: string;
  /** For each goroutine, the lock it waits for, if it is blocked. */
  waitsFor: [LockId | null, LockId | null];
};

export type Exploration = {frames: Frame[]; deadlocked: boolean};

type Core = {taken: [number, number]; finished: [boolean, boolean]};

function holderOf(programs: readonly [Program, Program], core: Core): Record<LockId, 0 | 1 | null> {
  const holder: Record<LockId, 0 | 1 | null> = {S: null, M: null, R: null};
  for (const goroutine of [0, 1] as const) {
    if (core.finished[goroutine]) continue;
    for (const lock of programs[goroutine].slice(0, core.taken[goroutine])) {
      holder[lock] = goroutine;
    }
  }
  return holder;
}

function waitsFor(programs: readonly [Program, Program], core: Core): [LockId | null, LockId | null] {
  const holder = holderOf(programs, core);
  return [0, 1].map((goroutine) => {
    if (core.finished[goroutine]) return null;
    const wanted = programs[goroutine][core.taken[goroutine]];
    if (wanted === undefined) return null;
    const owner = holder[wanted];
    return owner !== null && owner !== goroutine ? wanted : null;
  }) as [LockId | null, LockId | null];
}

/** step moves goroutine one step, or returns null if it cannot (blocked or finished). */
function step(
  programs: readonly [Program, Program],
  core: Core,
  goroutine: 0 | 1,
): {core: Core; action: string} | null {
  if (core.finished[goroutine]) return null;
  const program = programs[goroutine];
  const name = `Goroutine ${goroutine + 1}`;
  const taken = core.taken[goroutine];
  if (taken < program.length) {
    const wanted = program[taken];
    const owner = holderOf(programs, core)[wanted];
    if (owner !== null && owner !== goroutine) return null;
    const next: Core = {taken: [...core.taken], finished: [...core.finished]};
    next.taken[goroutine]++;
    return {core: next, action: `${name} takes the ${lockNames[wanted]}.`};
  }
  const next: Core = {taken: [...core.taken], finished: [...core.finished]};
  next.finished[goroutine] = true;
  return {
    core: next,
    action:
      program.length === 0
        ? `${name} needs no lock: it does its work and returns.`
        : `${name} has every lock it needs: it does its work and releases them.`,
  };
}

function key(core: Core): string {
  return `${core.taken.join(',')}|${core.finished.join(',')}`;
}

function frameOf(
  programs: readonly [Program, Program],
  core: Core,
  actor: 0 | 1 | null,
  action: string,
): Frame {
  return {
    taken: core.taken,
    finished: core.finished,
    holder: holderOf(programs, core),
    actor,
    action,
    waitsFor: waitsFor(programs, core),
  };
}

export function explore(programs: readonly [Program, Program]): Exploration {
  const start: Core = {taken: [0, 0], finished: [false, false]};
  const startFrame = frameOf(programs, start, null, 'Both goroutines start. No lock is taken.');
  // Breadth first: the shortest run that deadlocks, if any.
  type Node = {core: Core; frames: Frame[]};
  const seen = new Set<string>([key(start)]);
  let level: Node[] = [{core: start, frames: [startFrame]}];
  while (level.length > 0) {
    const nextLevel: Node[] = [];
    for (const node of level) {
      const moves = ([0, 1] as const)
        .map((goroutine) => ({goroutine, result: step(programs, node.core, goroutine)}))
        .filter((move) => move.result !== null);
      const done = node.core.finished[0] && node.core.finished[1];
      if (moves.length === 0 && !done) {
        const frames = [...node.frames];
        const last = frames[frames.length - 1];
        frames.push({
          ...last,
          actor: null,
          action:
            'Deadlock: each goroutine waits for a lock the other holds, and neither will ever let go.',
        });
        return {frames, deadlocked: true};
      }
      for (const move of moves) {
        const result = move.result!;
        const id = key(result.core);
        if (seen.has(id)) continue;
        seen.add(id);
        nextLevel.push({
          core: result.core,
          frames: [...node.frames, frameOf(programs, result.core, move.goroutine, result.action)],
        });
      }
    }
    level = nextLevel;
  }
  // No interleaving deadlocks: show the two taking turns, as fairly as they can.
  const frames: Frame[] = [startFrame];
  let core = start;
  let turn: 0 | 1 = 0;
  for (let guard = 0; guard < 32 && !(core.finished[0] && core.finished[1]); guard++) {
    let moved = step(programs, core, turn);
    let actor: 0 | 1 = turn;
    if (!moved) {
      actor = turn === 0 ? 1 : 0;
      moved = step(programs, core, actor);
    }
    if (!moved) break;
    core = moved.core;
    frames.push(frameOf(programs, core, actor, moved.action));
    turn = actor === 0 ? 1 : 0;
  }
  frames.push({
    ...frames[frames.length - 1],
    actor: null,
    action: 'Both finished. No order of their steps could have deadlocked them.',
  });
  return {frames, deadlocked: false};
}
