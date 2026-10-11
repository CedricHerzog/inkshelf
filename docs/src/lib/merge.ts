// Who wins when a book opens: the reader's saved progress or the server's. It follows
// progress.Merge (internal/progress/progress.go) line for line, and says why.

export type LocalProgress = {page: number; completed: boolean; unsynced: boolean};
export type RemoteProgress = {page: number; completed: boolean};

export type MergeReason = 'not-from-server' | 'unsynced' | 'same' | 'server-wins';

export type MergeResult = {
  merged: LocalProgress;
  /** Whether the book changed, so book.json must be saved: Merge's second result. */
  changed: boolean;
  reason: MergeReason;
};

export function merge(
  local: LocalProgress,
  remote: RemoteProgress,
  fromServer: boolean,
): MergeResult {
  // if !fromServer || local.Unsynced || (same page && same completed) { return local, false }
  if (!fromServer) return {merged: local, changed: false, reason: 'not-from-server'};
  if (local.unsynced) return {merged: local, changed: false, reason: 'unsynced'};
  if (local.page === remote.page && local.completed === remote.completed) {
    return {merged: local, changed: false, reason: 'same'};
  }
  return {
    merged: {...local, page: remote.page, completed: remote.completed},
    changed: true,
    reason: 'server-wins',
  };
}
