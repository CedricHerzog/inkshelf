package pocketbook

import (
	"image"
	"testing"
)

func TestBattery(t *testing.T) {
	fake := &fakeFirmware{global: make([]byte, globalShmSize)}
	putInt32s(fake.global[globalBattery:], 87)
	putInt32s(fake.global[globalCharging:], 1)
	current = &device{canvas: image.NewGray(image.Rect(0, 0, 4, 4)), firmware: fake}
	t.Cleanup(func() { current = nil })
	if percent, charging, ok := Battery(); percent != 87 || !charging || !ok {
		t.Errorf("Battery() = %d, %v, %v, want 87, charging", percent, charging, ok)
	}
}

func TestBatteryState(t *testing.T) {
	global := make([]byte, globalShmSize)
	putInt32s(global[globalBattery:], 87)
	putInt32s(global[globalCharging:], 1)
	if percent, charging := batteryState(global); percent != 87 || !charging {
		t.Errorf("batteryState = %d, %v, want 87, charging", percent, charging)
	}
}
