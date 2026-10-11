package timing

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"
)

func TestSpanEnd(t *testing.T) {
	span := Start(context.Background(), "work")
	time.Sleep(2 * time.Millisecond)
	duration := span.End()
	if duration < 2*time.Millisecond || duration%time.Millisecond != 0 {
		t.Errorf("End() = %v, want at least 2ms, rounded to the millisecond", duration)
	}
}

func TestSpanLog(t *testing.T) {
	var output bytes.Buffer
	writer, flags := log.Writer(), log.Flags()
	log.SetOutput(&output)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(writer); log.SetFlags(flags) })

	Start(context.Background(), "open book 7").Log("%d chapters", 12)
	Start(context.Background(), "index").Log("")
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "open book 7: ") || !strings.HasSuffix(lines[0], ", 12 chapters") ||
		!strings.HasPrefix(lines[1], "index: ") || strings.Contains(lines[1], ",") {
		t.Errorf("log = %q", lines)
	}
}

func TestTask(t *testing.T) {
	ctx, end := Task(context.Background(), "page turn")
	Start(ctx, "draw").End()
	end()
}

func TestPending(t *testing.T) {
	var pending Pending
	if pending.Context() == nil {
		t.Error("the zero Pending has no context")
	}
	pending.End()
	pending = StartPending("page turn")
	Start(pending.Context(), "draw page").End()
	pending.End()
	if pending != (Pending{}) {
		t.Error("End left a task")
	}
}
