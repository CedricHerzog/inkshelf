package timing

import (
	"context"
	"fmt"
	"log"
	"runtime/trace"
	"time"
)

// Span times one piece of work for the log and, while a trace runs, marks it as a region of
// the trace. End it on the goroutine that started it, which a trace region requires.
type Span struct {
	name   string
	start  time.Time
	region *trace.Region
}

// Start starts a span named name. ctx carries the task the work belongs to, if any; a span
// with no task takes context.Background().
func Start(ctx context.Context, name string) Span {
	return Span{name: name, start: time.Now(), region: trace.StartRegion(ctx, name)}
}

// End ends the span and returns how long it took, rounded to the millisecond for the log.
func (span Span) End() time.Duration {
	span.region.End()
	return Round(time.Since(span.start))
}

// Log ends the span and logs its name, its duration, then what format and args say.
func (span Span) Log(format string, args ...any) {
	duration := span.End()
	if format == "" {
		log.Printf("%s: %v", span.name, duration)
		return
	}
	log.Printf("%s: %v, %s", span.name, duration, fmt.Sprintf(format, args...))
}

// Round rounds a duration to the millisecond, the precision of the log: a page turn takes
// tens to hundreds of them.
func Round(duration time.Duration) time.Duration { return duration.Round(time.Millisecond) }

// Task starts a task named name, which the spans started with the returned context belong to,
// whichever goroutine starts them. Call end once the operation is over.
func Task(ctx context.Context, name string) (taskContext context.Context, end func()) {
	taskContext, task := trace.NewTask(ctx, name)
	return taskContext, task.End
}

// Pending is a task that one event starts and a later one ends, kept in between where both
// can reach it: a page turn starts with a key and ends once its page is drawn. Its zero value
// is no task. Guard it as what holds it.
type Pending struct {
	ctx  context.Context
	task *trace.Task
}

// StartPending starts a pending task named name.
func StartPending(name string) Pending {
	ctx, task := trace.NewTask(context.Background(), name)
	return Pending{ctx: ctx, task: task}
}

// Context returns the context of the task, for the spans that belong to it, or
// context.Background() if there is no task.
func (pending Pending) Context() context.Context {
	if pending.ctx == nil {
		return context.Background()
	}
	return pending.ctx
}

// End ends the task, if there is one, and leaves no task.
func (pending *Pending) End() {
	if pending.task != nil {
		pending.task.End()
	}
	*pending = Pending{}
}
