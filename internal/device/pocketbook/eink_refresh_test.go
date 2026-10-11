package pocketbook

import (
	"image"
	"slices"
	"testing"
)

func TestMXCFBUpdate(t *testing.T) {
	data := mxcfbUpdate(image.Rect(10, 20, 110, 220), GL16, false, 7)
	// top, left, width, height, waveform, mode, marker, temperature
	want := int32s(20, 10, 100, 200, int32(GL16), updateModePartial, 7, tempUseAmbient)
	if !slices.Equal(data[:len(want)], want) {
		t.Errorf("partial update = %x, want %x", data, want)
	}
	const modeOffset = 20
	if full := mxcfbUpdate(image.Rect(0, 0, 1, 1), GC16, true, 1); int32At(full[:], modeOffset) != updateModeFull {
		t.Error("full update: mode is not UPDATE_MODE_FULL")
	}
}
