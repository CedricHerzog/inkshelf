package pocketbook

import (
	"bytes"
	"strings"
)

// The firmware keeps the reader's settings in text files of key=value lines, global.cfg first,
// which its settings app rewrites when a setting changes.

// darkModeSetting is the global.cfg setting that is 1 in dark mode. The firmware does not
// document it: it was found by switching dark mode on an InkPad 3 (firmware 6.x) and comparing
// the settings. night_mode, next to it, stays 1 either way.
const darkModeSetting = "screen_mode_inverted"

// parseSettings returns the settings of a firmware settings file. Lines without "=" (blank,
// [section] headers, comments) are skipped.
func parseSettings(data []byte) map[string]string {
	settings := map[string]string{}
	for line := range bytes.Lines(data) {
		key, value, found := strings.Cut(string(line), "=")
		if found {
			settings[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return settings
}

// darkModeOf reports whether settings turn dark mode on, and whether they say at all: a file
// read while the firmware rewrites it may lack the setting, or end before its value.
func darkModeOf(settings map[string]string) (dark, found bool) {
	value := settings[darkModeSetting]
	return value != "" && value != "0", value != ""
}
