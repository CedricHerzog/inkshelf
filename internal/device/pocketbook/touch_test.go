package pocketbook

import (
	"image"
	"slices"
	"testing"
	"time"
)

func TestGestures(t *testing.T) {
	// touchAt is the raw touch for screen point (x, y).
	touchAt := func(kind int32, x, y int) []byte {
		rawX, rawY := rawTouch(image.Pt(x, y), screenSize)
		return encodedEvent(kind, 0, rawX, rawY)
	}
	for _, test := range []struct {
		name  string
		moveX int             // where the finger moves after going down at x 100
		polls []time.Duration // when to poll, after going down
		want  []PointerState
	}{
		{
			name:  "tap",
			moveX: 100,
			polls: []time.Duration{longPressDelay / 2},
			want:  []PointerState{PointerDown, PointerMove},
		},
		{
			name:  "long, then holds",
			moveX: 105,
			polls: []time.Duration{longPressDelay - time.Millisecond, longPressDelay, longPressDelay + holdInterval/2, longPressDelay + holdInterval},
			want:  []PointerState{PointerDown, PointerMove, PointerLong, PointerHold},
		},
		{
			name:  "drag",
			moveX: 140,
			polls: []time.Duration{2 * longPressDelay},
			want:  []PointerState{PointerDown, PointerMove, PointerDrag},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			loop, fake, app := newTestLoop()
			fake.events = [][]byte{touchAt(eventPointerDown, 100, 500), touchAt(eventPointerMove, test.moveX, 500)}
			loop.poll(testStart)
			for _, after := range test.polls {
				loop.poll(testStart.Add(after))
			}
			got := app.pointerStates()
			if !slices.Equal(got, test.want) {
				t.Fatalf("states = %v, want %v", got, test.want)
			}
			if last := app.pointers[len(app.pointers)-1]; last.State == PointerHold && last.Point != image.Pt(105, 500) {
				t.Errorf("held at %v, want where the finger is, (105, 500)", last.Point)
			}

			// Lifting the finger ends the touch: no more holds.
			fake.events = [][]byte{touchAt(eventPointerUp, test.moveX, 500)}
			loop.poll(testStart.Add(10 * longPressDelay))
			loop.poll(testStart.Add(20 * longPressDelay))
			if want := append(slices.Clone(got), PointerUp); !slices.Equal(app.pointerStates(), want) {
				t.Errorf("after up, states = %v, want %v", app.pointerStates(), want)
			}
		})
	}
}

// Taps on the centres of squares drawn at known screen points, logged on an InkPad 3.
func TestTouchPoint(t *testing.T) {
	for _, tap := range []struct {
		rawX, rawY int32
		want       image.Point
	}{
		{rawX: 1760, rawY: 97, want: image.Pt(100, 100)},
		{rawX: 1765, rawY: 1325, want: image.Pt(1304, 100)},
		{rawX: 300, rawY: 1295, want: image.Pt(1304, 1572)},
		{rawX: 312, rawY: 103, want: image.Pt(100, 1572)},
		{rawX: 933, rawY: 699, want: image.Pt(702, 936)},
	} {
		got := touchPoint(tap.rawX, tap.rawY, screenSize)
		if delta := got.Sub(tap.want); abs(delta.X) > 30 || abs(delta.Y) > 30 {
			t.Errorf("touchPoint(%d, %d) = %v, want about %v", tap.rawX, tap.rawY, got, tap.want)
		}
	}
	if got := touchPoint(-5, 5000, screenSize); got != image.Pt(1403, 1871) {
		t.Errorf("touchPoint off the screen = %v, want it clamped to (1403, 1871)", got)
	}
}

// abs returns the absolute value of value.
func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func TestRawTouch(t *testing.T) {
	for _, point := range []image.Point{{0, 0}, {100, 100}, {1304, 1572}, {1403, 1871}} {
		rawX, rawY := rawTouch(point, screenSize)
		if got := touchPoint(rawX, rawY, screenSize); got != point {
			t.Errorf("touchPoint(rawTouch(%v)) = %v", point, got)
		}
	}
}
