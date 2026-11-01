// Package deadline bounds how long the reader waits for a piece of work, not the work itself.
//
// A book can hold an image that takes seconds to decode, an SVG with a hundred thousand shapes,
// a formula nested to the limit of its parser. The caps of each parser stop the worst of them,
// but what passes the caps can still be slow on the reader's processor, and a page must not
// stay blank while it runs. Go cannot stop a function from outside: a decoder that takes no
// context runs to its end. So the work goes to a worker, a goroutine that lives as long as the
// app, and the caller waits for the result only as long as its budget allows. Past it, the
// caller shows the element's fallback, and the worker's late result is dropped when it comes.
//
// Two things keep given-up work from piling up. A key given up once is not run again for the
// life of the pool: the next page turn shows the fallback at once instead of waiting again for
// the same image. And when every worker is still busy with work nobody waits for, a new job is
// refused at once instead of queuing behind it.
//
// The pool never starts a goroutine: its owner starts each worker with Loop, so that in the
// app only the main thread starts goroutines (docs/reference/app-threading.md).
package deadline
