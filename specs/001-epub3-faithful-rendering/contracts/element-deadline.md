# Contract: per-element deadline (internal/deadline)

A utility package beside `timing` and `naturalorder`: it imports only `timing`, so any domain
may use it. It bounds the wait for one piece of work (FR-014a), not the work itself.

## Operations

- `NewPool(workers int)`: a pool whose `Loop(ctx)` method is a worker's body. The app makes a
  reading pool (2 workers) and a download pool (1) at `Init` and starts each worker with
  `state.GoLoop` (principle V); tests start theirs the same way.
- `deadline.Run(pool, name, key, limit, job) (result, ok)`, generic in the job's result type:
  runs `job` on a free worker, in a span called `name`, and waits at most `limit`. `ok` is
  false when the limit passed (or was zero), when the key was given up before, when every
  worker is busy with given-up work, or when the job panicked; the caller then shows the
  fallback. A result that arrives late is dropped.
- A nil pool runs the job inline with no limit (tests, tools).

## Guarantees

- `Run` never starts a goroutine.
- A given-up key is not run again until the pool is made again (the next app start).
- Each job runs inside its own `timing.Start` span, named by the caller (`decode image`,
  `rasterize svg`, `lay out formula`); a given-up job logs one line with its key and duration.

## Used by

`typesetting/layout` (images, SVGs and formulas of a page, at draw) through the reading pool in
`openbook.TextSettings`, and `ebook/fixedlayout` (the images of a page, at download) through the
download pool. The limit is per page (FR-014a): the caller passes each element
min(10 s, what is left of the page's 10 s), and once the page's time is spent every element left
shows its fallback without a wait.
