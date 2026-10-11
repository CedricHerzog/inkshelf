// A toy model of shelf.Store.Download for a book of page images (internal/shelf/download.go):
// fetchers take page numbers in order and ask the server for each page, unless it is on disk
// already (pageOnDisk); they hand the page to a queue that holds fetchedQueue = 2 pages; the
// preparers take pages from it, prepare them (pageimage.Prepare: decode, fit, tone, dither and
// pack) and write them with writeFileAtomic (a .tmp file, then a rename). Progress counts the
// leading pages on disk (finishPage), and book.json is written once every page is there.
//
// The timings are made up, only their proportions matter. The server's slowdown under many
// requests follows the comment on fetchWorkers ("more only slows the server down"): here it
// serves three requests at full speed and shares that speed among more.

export const fetchedQueue = 2;
export const prepareStages = ['decode', 'fit', 'tone', 'dither', 'write'] as const;
export type PrepareStage = (typeof prepareStages)[number];

const fetchMs = 900;
const stageMs: Record<PrepareStage, number> = {
  decode: 260,
  fit: 200,
  tone: 90,
  dither: 300,
  write: 180,
};
const serverFullSpeed = 3;

export type FileState = 'tmp' | 'pg4';

export type Fetcher = {page: number | null; progress: number; blocked: boolean; skipped: number | null};
export type Preparer = {page: number | null; stage: number; progress: number};

export type DownloadState = {
  pages: number;
  nextPage: number; // the next page number to hand to a fetcher, from 1
  fetchers: Fetcher[];
  queue: number[];
  preparers: Preparer[];
  files: Map<number, {state: FileState; progress: number}>;
  done: number; // pages 1 to done are on disk
  bookJson: boolean;
  elapsed: number;
  skippedCount: number;
};

/** jitter makes some pages slower to fetch than others, the same on every run. */
function jitter(page: number): number {
  return 0.7 + ((page * 37) % 11) / 18;
}

export function newDownload(
  pages: number,
  fetchers: number,
  preparers: number,
  leftFiles: ReadonlyMap<number, {state: FileState; progress: number}> = new Map(),
): DownloadState {
  // The files an interrupted download left: whole pages, and maybe a .tmp cut short.
  const files = new Map(leftFiles);
  const state: DownloadState = {
    pages,
    nextPage: 1,
    fetchers: Array.from({length: Math.min(fetchers, pages)}, () => ({
      page: null,
      progress: 0,
      blocked: false,
      skipped: null,
    })),
    queue: [],
    preparers: Array.from({length: Math.min(preparers, pages)}, () => ({
      page: null,
      stage: 0,
      progress: 0,
    })),
    files,
    done: 0,
    bookJson: false,
    elapsed: 0,
    skippedCount: 0,
  };
  advanceDone(state);
  return state;
}

function advanceDone(state: DownloadState) {
  while (state.done < state.pages && state.files.get(state.done + 1)?.state === 'pg4') state.done++;
}

export function finished(state: DownloadState): boolean {
  return state.bookJson;
}

/** tick advances the download by elapsedMs, changing state in place. */
export function tick(state: DownloadState, elapsedMs: number) {
  if (state.bookJson) return;
  state.elapsed += elapsedMs;
  const fetching = state.fetchers.filter((fetcher) => fetcher.page !== null && !fetcher.blocked).length;
  const speed = Math.min(1, serverFullSpeed / Math.max(1, fetching));

  for (const fetcher of state.fetchers) {
    fetcher.skipped = null;
    if (fetcher.page === null) {
      // Take the next page number, skipping the pages already on disk.
      while (state.nextPage <= state.pages && state.files.get(state.nextPage)?.state === 'pg4') {
        fetcher.skipped = state.nextPage;
        state.skippedCount++;
        state.nextPage++;
      }
      if (state.nextPage > state.pages) continue;
      fetcher.page = state.nextPage++;
      fetcher.progress = 0;
      fetcher.blocked = false;
      continue;
    }
    if (!fetcher.blocked) {
      fetcher.progress += (elapsedMs * speed) / (fetchMs * jitter(fetcher.page));
      if (fetcher.progress >= 1) fetcher.blocked = true; // fetched: wait for room in the queue
    }
    if (fetcher.blocked && state.queue.length < fetchedQueue) {
      state.queue.push(fetcher.page);
      fetcher.page = null;
      fetcher.blocked = false;
      fetcher.progress = 0;
    }
  }

  for (const preparer of state.preparers) {
    if (preparer.page === null) {
      const page = state.queue.shift();
      if (page === undefined) continue;
      preparer.page = page;
      preparer.stage = 0;
      preparer.progress = 0;
      continue;
    }
    const stage = prepareStages[preparer.stage];
    preparer.progress += elapsedMs / stageMs[stage];
    if (stage === 'write') {
      state.files.set(preparer.page, {state: 'tmp', progress: Math.min(1, preparer.progress)});
    }
    if (preparer.progress >= 1) {
      preparer.progress = 0;
      preparer.stage++;
      if (preparer.stage >= prepareStages.length) {
        state.files.set(preparer.page, {state: 'pg4', progress: 1}); // the rename
        preparer.page = null;
        preparer.stage = 0;
        advanceDone(state);
      }
    }
  }

  if (state.done === state.pages) state.bookJson = true; // Store.Save, last
}

/** pullPlug stops everything at once, as the reader dying would: only the files stay. */
export function pullPlug(state: DownloadState): Map<number, {state: FileState; progress: number}> {
  return new Map(state.files);
}
