package pocketbook

import "testing"

func TestSystemEventString(t *testing.T) {
	for _, test := range []struct {
		event SystemEvent
		want  string
	}{
		{event: SaveState, want: "save state"},
		{event: NetDisconnected, want: "net disconnected"},
		{event: SystemEvent(-1), want: "unknown"},
		{event: SystemEvent(99), want: "unknown"},
	} {
		if got := test.event.String(); got != test.want {
			t.Errorf("SystemEvent(%d).String() = %q, want %q", int(test.event), got, test.want)
		}
	}
}
