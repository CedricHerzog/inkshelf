package pocketbook

import "testing"

func TestFirmwareSettings(t *testing.T) {
	settings := parseSettings([]byte("[general]\nlanguage=fr\n night_mode = 1 \n\nbad line\n"))
	if settings["language"] != "fr" || settings["night_mode"] != "1" || len(settings) != 2 {
		t.Errorf("parseSettings = %v", settings)
	}
	// As in global.cfg on an InkPad 3 in dark mode, out of it, and cut short.
	for _, test := range []struct {
		file        string
		dark, found bool
	}{
		{"night_mode=1\nscreen_mode_inverted=1\n", true, true},
		{"night_mode=1\nscreen_mode_inverted=0\n", false, true},
		{"night_mode=1\n", false, false},
		{"night_mode=1\nscreen_mode_inverted=", false, false},
	} {
		if dark, found := darkModeOf(parseSettings([]byte(test.file))); dark != test.dark || found != test.found {
			t.Errorf("darkModeOf(%q) = %v, %v; want %v, %v", test.file, dark, found, test.dark, test.found)
		}
	}
}
