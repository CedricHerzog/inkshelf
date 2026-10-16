// What the event loop makes of raw touches and button presses: drags, long presses, holds and
// key repeats. The firmware only sends down, move, up, press and release. Follows handleTouch
// and sendLongPress (internal/device/pocketbook/touch.go), repeatHeldKey (keys.go) and the
// order of poll (event_loop.go).

/** pollInterval: while a finger or a button is down, the event loop polls every 20 ms. */
export const POLL_INTERVAL_MS = 20;
/** idlePollInterval: otherwise, it polls the task queue every 250 ms; input wakes it at once. */
export const IDLE_POLL_INTERVAL_MS = 250;
/** longPressDelay and holdInterval of touch.go. */
export const LONG_PRESS_DELAY_MS = 500;
export const HOLD_INTERVAL_MS = 500;
/** dragDeadZone of touch.go: a squared radius, in screen pixels. */
export const DRAG_DEAD_ZONE = 200;
/** repeatDelay and repeatInterval of keys.go. */
export const REPEAT_DELAY_MS = 600;
export const REPEAT_INTERVAL_MS = 300;

export type Point = {x: number; y: number};

export type PointerState = 'Down' | 'Move' | 'Drag' | 'Long' | 'Hold' | 'Up';

export type PointerEvent = {state: PointerState; point: Point; at: number};

export function squaredDistance(first: Point, second: Point): number {
  const dx = first.x - second.x;
  const dy = first.y - second.y;
  return dx * dx + dy * dy;
}

/** TouchTracker is touchState and the two functions of touch.go that change it. */
export class TouchTracker {
  active = false;
  start: Point = {x: 0, y: 0};
  last: Point = {x: 0, y: 0};
  since = 0;
  dragging = false;
  longSent = false;
  lastHold = 0;

  /** handleTouch: the events the app gets for one raw down, move or up. */
  handle(kind: 'down' | 'move' | 'up', point: Point, now: number): PointerEvent[] {
    switch (kind) {
      case 'down':
        this.active = true;
        this.start = point;
        this.last = point;
        this.since = now;
        this.dragging = false;
        this.longSent = false;
        this.lastHold = 0;
        return [{state: 'Down', point, at: now}];
      case 'move': {
        this.last = point;
        const events: PointerEvent[] = [{state: 'Move', point, at: now}];
        if (!this.active) return events;
        if (!this.dragging && squaredDistance(point, this.start) > DRAG_DEAD_ZONE) {
          this.dragging = true;
        }
        if (this.dragging) events.push({state: 'Drag', point, at: now});
        return events;
      }
      case 'up':
        this.active = false;
        return [{state: 'Up', point, at: now}];
    }
  }

  /** sendLongPress: called at every poll, as no input arrives while a finger is held still. */
  poll(now: number): PointerEvent[] {
    if (!this.active || this.dragging || now - this.since < LONG_PRESS_DELAY_MS) return [];
    if (!this.longSent) {
      this.longSent = true;
      this.lastHold = now;
      return [{state: 'Long', point: this.last, at: now}];
    }
    if (now - this.lastHold >= HOLD_INTERVAL_MS) {
      this.lastHold = now;
      return [{state: 'Hold', point: this.last, at: now}];
    }
    return [];
  }
}

export type KeyState = 'Press' | 'Repeat' | 'Release';

export type KeyEvent = {state: KeyState; at: number};

/** KeyRepeater is the held-button part of eventLoop, with repeatHeldKey. */
export class KeyRepeater {
  held = false;
  heldSince = 0;
  lastRepeat = 0;

  press(now: number): KeyEvent {
    this.held = true;
    this.heldSince = now;
    this.lastRepeat = now;
    return {state: 'Press', at: now};
  }

  release(now: number): KeyEvent {
    this.held = false;
    return {state: 'Release', at: now};
  }

  /** repeatHeldKey: a repeat after 600 ms, then every 300 ms, checked at every poll. */
  poll(now: number): KeyEvent | null {
    if (
      this.held &&
      now - this.heldSince >= REPEAT_DELAY_MS &&
      now - this.lastRepeat >= REPEAT_INTERVAL_MS
    ) {
      this.lastRepeat = now;
      return {state: 'Repeat', at: now};
    }
    return null;
  }
}
