# Finding what is slow

Three tools, from the quickest to the most detailed:

1. **The log** says how long each page turn, chapter layout, refresh and download took. Start
   here: it tells you *which* operation is slow, on the reader itself.
2. **Benchmarks** time one function in a loop on your computer, and benchstat says whether a
   change made it faster or slower. They tell you *how much* a piece of work costs, and catch
   regressions.
3. **Profiles** of a whole run (in the emulator or on the reader) say *where* the time went:
   the CPU profile by function, the execution trace on a timeline, per goroutine, with the
   page turns marked.

## Spans: the log and the trace

[internal/timing](../../internal/timing) times a piece of work with a span:

```go
span := timing.Start(ctx, "layout chapter")
pages, err := layout.Paginate(...)
log.Printf("layout chapter %d: %v, %d pages", chapter, span.End(), len(pages))
```

`End` returns the duration, rounded to the millisecond, for the log line. While an execution
trace is being captured, the span is also a *region* of the trace, named as the span, so the
log and the trace use the same names. When no trace runs, a region costs nanoseconds: spans stay
in the code. `span.Log("%d chapters", count)` ends and logs in one call (`name: duration,
what follows`).

A *task* groups the regions of one operation across goroutines. A page turn is one:
`openbook.TurnPage` starts a `page turn` task (`timing.StartPending`), kept in
`state.Reader.Turn`, and the reader's `DrawAndRefreshLocked` ends it once the page is drawn and
the panel refreshed. A download is a `download` task, with its `http request`, `fetch page` and
`prepare page` regions inside.

The spans in place:

| Span or region | Where | In the log |
|---|---|---|
| `draw`, `key`, `tap`, `long press` | `app.Draw`, `app.Key`, `app.Pointer` | no |
| `page turn` (task) | `openbook.TurnPage` → the next draw | no |
| `draw page`, `load page` | the reader's draw; a page not preloaded | `page 12: 85ms (load 0s) area …`, `chapter 3 page 5: …` |
| `preload page` | the preloader | `preload page 13: …` (page images) |
| `open book` | `openbook.OpenLocked` | `open book 42: 12ms` |
| `layout chapter` with `chapter document`, `paginate` | `reading`'s renderer | `layout chapter 3: 140ms, 22 pages` |
| `render page` | `reading`'s renderer | `render chapter 3 page 5: 30ms` |
| `build index` | `epub.BuildIndex` | no |
| `refresh`, `wait for the panel` | the firmware's flush, on the reader and in the emulator | `refresh …: wait …, copy …, send …` (reader only) |
| `download` (task and span), `fetch page`, `prepare page` with `decode`, `fit`, `dither`, `write page` | `downloadqueue`, `shelf` | `download 42 in 31s: <nil>` and the per-page line |
| `draw fixed-layout pages` | `shelf` | `download 42: fixed-layout EPUB of 30 pages drawn in …` |
| `http request`, `http body` | `httpclient` | no |
| `cover view` | `covers` | `cover view of book 42 in …` |
| `connect Wi-Fi` | the reader's Wi-Fi | `Wi-Fi connected in …` |

New work that can take a while gets a span. Spans never go inside a `HOT PATH` function
([image-pipeline.md](image-pipeline.md#hot-paths)), only around its callers: a region per pixel
or glyph would cost more than the work.

## Profiling a run

`timing.Capture(dir, limit)` writes, into `dir`:

| File | What | Read it with |
|---|---|---|
| `cpu.pprof` | where the CPU went, by function | `go tool pprof -http=: cpu.pprof` |
| `trace.out` | every goroutine on a timeline, with the spans' regions and tasks | `go tool trace trace.out` |
| `heap.pprof`, `allocs.pprof` | what is allocated at the end, and all that was allocated | `go tool pprof -sample_index=alloc_space allocs.pprof` |
| `block.pprof`, `mutex.pprof` | where goroutines waited on channels, and on contended locks | `go tool pprof` |
| `goroutine.pprof` | the goroutines at the end | `go tool pprof` |

Block and mutex sampling are on only during a capture.

**In the emulator**, `-profile DIR` captures the whole run. With a script, the run can be repeated
exactly:

```sh
make profile SCRIPT=emulator/scripts/calibreweb-epub.txt    # -> build/profile/calibreweb-epub/
make emu-fake EMUFLAGS="-profile build/profile/manual"      # interactive, until Exit
```

`make profile` gives each script its fake servers, as `shots-all` does. Without `-eink` the
emulated refreshes take no time, so the profile shows the app's own work; with it, the trace
shows page turns as long as on the reader. Your computer is many times faster than the reader's
Cortex-A7: compare profiles with each other, not with the reader's log.

**On the reader**, create the file `applications/inkshelf/profile` over USB, then start the app.
It profiles into `applications/inkshelf-profile/` until the app exits, or for the duration
written in the file (`90s`, `10m`; 5 minutes if empty), since the firmware may kill the app
without letting it stop the capture and a trace grows by megabytes a minute. The log says
`profiling to …` and `… stopped`. Copy the folder back over USB, or under
[remote control](remote-control.md) run `make reader-profile` (into `build/reader-profile/`). Then
delete the marker file: profiling slows the app down.

### Reading a trace

`go tool trace trace.out` opens a page in the browser.

- **User-defined tasks** lists the `page turn` and `download` tasks with their durations. Click
  one to see its regions: was the time spent laying the chapter out, drawing the page, or
  waiting for the panel?
- **User-defined regions** gives each region's durations: how `render page` and `refresh` vary.
- **View trace by proc** shows what ran on each core and when: the preloader drawing the next
  page while the panel refreshes, or not.

In `go tool pprof -http=:`, the flame graph shows the functions the CPU went to. `-focus=Paginate`
keeps only what was under one function.

## Benchmarks

Every package with costly work has benchmarks, in its `bench_test.go` or next to its tests,
including an end-to-end page turn in `internal/app`. They build their inputs in code. Each reports
allocations too.

```sh
make bench                                        # all of them, 6 times each -> build/bench/new.txt
make bench BENCH=Paginate PKG=./internal/typesetting/layout
make bench-baseline                               # the same, saved as build/bench/base.txt
make bench-compare                                # benchstat base.txt new.txt
```

The whole set takes about ten minutes on a recent laptop, so narrow it to what a change touches.
Before a change: `make bench-baseline` (narrowed with `BENCH` and `PKG`). After it: `make bench`
with the same narrowing, then `make bench-compare`. benchstat prints each benchmark's change with
its confidence: `~` means no significant difference. For one benchmark's own profile:

```sh
go test -run '^$' -bench PaginateChapter -cpuprofile build/cpu.pprof ./internal/typesetting/layout
go tool pprof -http=: build/cpu.pprof
```

A hot path has further rules: [image-pipeline.md](image-pipeline.md#hot-paths).
