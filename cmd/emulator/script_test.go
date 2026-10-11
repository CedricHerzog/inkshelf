package main

import (
	"image"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
)

func TestParseScript(t *testing.T) {
	steps, err := parseScript(`
# a comment
tap 10 20
long 1 2   # trailing comment
swipe 100 900 1200 900
key next
hold prev 2s
home
resume
battery 15 charging
net down
wait 300ms
idle
idle 1m
shot 01-series
exit
`)
	if err != nil {
		t.Fatal(err)
	}
	want := []step{
		{line: 3, command: "tap", points: []image.Point{{10, 20}}},
		{line: 4, command: "long", points: []image.Point{{1, 2}}},
		{line: 5, command: "swipe", points: []image.Point{{100, 900}, {1200, 900}}},
		{line: 6, command: "key", key: pocketbook.KeyNext},
		{line: 7, command: "hold", key: pocketbook.KeyPrev, duration: 2 * time.Second},
		{line: 8, command: "home"},
		{line: 9, command: "resume"},
		{line: 10, command: "battery", number: 15, flag: true},
		{line: 11, command: "net"},
		{line: 12, command: "wait", duration: 300 * time.Millisecond},
		{line: 13, command: "idle", duration: defaultTimeout},
		{line: 14, command: "idle", duration: time.Minute},
		{line: 15, command: "shot", name: "01-series"},
		{line: 16, command: "exit"},
	}
	if !reflect.DeepEqual(steps, want) {
		t.Errorf("parseScript =\n%+v\nwant\n%+v", steps, want)
	}
}

func TestParseScriptErrors(t *testing.T) {
	for _, source := range []string{
		"tap 10",
		"tap ten 20",
		"swipe 1 2 3",
		"key power",
		"hold next",
		"hold next soon",
		"home now",
		"battery 101",
		"battery 50 plugged",
		"net sideways",
		"wait",
		"shot ../escape",
		"jump 1 2",
	} {
		if _, err := parseScript("tap 1 1\n" + source); err == nil || !strings.HasPrefix(err.Error(), "line 2: ") {
			t.Errorf("parseScript(%q) = %v, want an error on line 2", source, err)
		}
	}
}
