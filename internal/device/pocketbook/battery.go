package pocketbook

// Battery returns the battery's charge in percent and whether it is charging, as the firmware
// sees it. ok is false before Run, or if the firmware's state is missing.
func Battery() (percent int, charging, ok bool) {
	if current == nil {
		return 0, false, false
	}
	return current.firmware.battery()
}

// batteryState reads the battery's charge and whether it is charging from the firmware's
// shared state, where libinkview's GetBatteryPower and IsCharging read them too.
func batteryState(global []byte) (percent int, charging bool) {
	return int(int32At(global, globalBattery)), global[globalCharging]&1 != 0
}
