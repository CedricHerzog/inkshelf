// The byte layouts of the PocketBook firmware's messages, as the event loop reads and writes
// them. Follows internal/device/pocketbook/firmware_messages.go (encodeHardwareEvent,
// taskEvent, taskReply, taskRequest, requestID) and touchPoint and rawTouch of touch.go.

export const SCREEN_WIDTH = 1404;
export const SCREEN_HEIGHT = 1872;

// Event numbers (EVT_* of the 6.5 SDK's inkview.h).
export const EVENT = {
  exit: 22,
  keyPress: 25,
  keyRelease: 26,
  keyRepeat: 28,
  pointerUp: 29,
  pointerDown: 30,
  pointerMove: 31,
  fsChanged: 73,
  foreground: 151,
  background: 152,
  saveState: 155,
  usbStoreIn: 163,
  usbStoreOut: 164,
  netConnected: 256,
  netDisconnected: 257,
} as const;

export const HARDWARE_EVENT_SIZE = 32;
export const FULL_PRESSURE = 255;
export const FINGER_TOOL = 1;

export const TASK_ASYNC_EVENT = 0x50d;
export const TASK_SYNC_EVENT = 0x513;
export const TASK_BAN_SUSPEND = 0x123;
export const FIRMWARE_MTYPE = 0x7fffffff;
export const TASK_EVENT_MTYPE = 0x7a000000;

/** int32s writes values as little-endian int32s into a buffer of `size` bytes. */
function int32s(size: number, values: readonly number[]): Uint8Array {
  const bytes = new Uint8Array(size);
  const view = new DataView(bytes.buffer);
  values.forEach((value, index) => view.setInt32(index * 4, value | 0, true));
  return bytes;
}

/** rawTouch is the inverse of touchPoint: the raw coordinates the touch panel reports. */
export function rawTouch(x: number, y: number): {rawX: number; rawY: number} {
  return {rawX: SCREEN_HEIGHT - 1 - y, rawY: x};
}

/** touchPoint: screen x = raw y, screen y = 1871 - raw x, clamped to the screen. */
export function touchPoint(rawX: number, rawY: number): {x: number; y: number} {
  const x = rawY;
  const y = SCREEN_HEIGHT - 1 - rawX;
  return {
    x: Math.min(Math.max(x, 0), SCREEN_WIDTH - 1),
    y: Math.min(Math.max(y, 0), SCREEN_HEIGHT - 1),
  };
}

/** hardwareEvent is a 32-byte /hwevent message: {type, touch id, key or raw x, raw y, pressure, tool, 0, 0}. */
export function hardwareEvent(kind: number, keyOrRawX: number, rawY = 0): Uint8Array {
  return int32s(HARDWARE_EVENT_SIZE, [kind, 0, keyOrRawX, rawY, FULL_PRESSURE, FINGER_TOOL]);
}

/** taskEvent: {mtype 0x7a000000|pgid, id, 0x50d, 0, 16, {event, 0, code, value}}. */
export function taskEvent(pgid: number, id: number, event: number, code = pgid, value = 0): Uint8Array {
  return int32s(36, [TASK_EVENT_MTYPE | pgid, id, TASK_ASYNC_EVENT, 0, 16, event, 0, code, value]);
}

/** requestID holds the pid in bits 12 to 27 and a serial number in bits 0 to 11. */
export function requestID(pid: number, serial: number): number {
  return ((pid << 12) & 0x0ffff000) | (serial & 0xfff);
}

/** taskRequest: {mtype 0x7fffffff, id, kind, argument, length 0}. */
export function taskRequest(id: number, kind: number, argument: number): Uint8Array {
  return int32s(24, [FIRMWARE_MTYPE, id, kind, argument]);
}

/** taskReply: the answer to a sync event, {mtype id, id, 0, 1, length 0}. */
export function taskReply(id: number): Uint8Array {
  return int32s(24, [id, id, 0, 1]);
}

/** hexBytes writes bytes as two-digit hex, grouped by int32. */
export function hexBytes(bytes: Uint8Array): string[] {
  const groups: string[] = [];
  for (let offset = 0; offset < bytes.length; offset += 4) {
    groups.push(
      Array.from(bytes.slice(offset, offset + 4), (byte) => byte.toString(16).padStart(2, '0')).join(' '),
    );
  }
  return groups;
}

/** hex writes a number as 0x… */
export function hex(value: number): string {
  return `0x${(value >>> 0).toString(16)}`;
}
