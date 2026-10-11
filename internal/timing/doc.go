// Package timing finds out where the time goes, in the log and in profiles.
//
// A span times one piece of work: a page drawn, a chapter laid out, a refresh of the panel, a
// download. Ending it gives its duration, which the log prints, so that the log of a session
// on the reader says how long each page turn took and where. While an execution trace runs
// (see Capture), a span is also a region of that trace: go tool trace then shows each piece of
// work on a timeline, on the goroutine that did it, nested in the spans around it. When no
// trace runs, a region costs a few nanoseconds, so spans stay in the code for good.
//
// A task groups the regions of one operation that crosses goroutines, such as a page turn:
// the input on the event loop, the page drawn by the preloader, the draw and the refresh. Its
// context is passed to the spans that belong to it, and go tool trace lists tasks with their
// latency, the first place to look for a slow page turn.
//
// Capture writes the profiles that explain a slow span: where the CPU went (cpu.pprof), the
// timeline (trace.out), and what was allocated, what blocked and on which locks (heap,
// allocs, block, mutex, goroutine). Read them with go tool pprof and go tool trace;
// docs/reference/profiling.md shows how.
//
// Spans never go inside the hot paths (docs/reference/image-pipeline.md), only around their callers: a
// region in a function called per pixel or per glyph would cost more than the function.
package timing
