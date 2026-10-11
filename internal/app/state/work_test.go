package state

import (
	"slices"
	"testing"
)

// A wake-up is work from Wake until Done, and a second Wake while one waits adds nothing.
func TestWakeupCountsUntilDone(t *testing.T) {
	var work Work
	wakeup := NewWakeup(&work, "covers")
	wakeup.Wake()
	wakeup.Wake()
	if names, _ := work.InProgress(); !slices.Equal(names, []string{"covers"}) {
		t.Fatalf("in progress %v, want [covers]", names)
	}
	<-wakeup.Woken()
	wakeup.Done()
	if names, lastEnded := work.InProgress(); len(names) != 0 || lastEnded.IsZero() {
		t.Fatalf("in progress %v, last ended %v; want none, and a time", names, lastEnded)
	}
}

// Work of the same name counts until each one has ended.
func TestWorkCountsEachBegin(t *testing.T) {
	var work Work
	work.Begin("downloading")
	work.Begin("downloading")
	work.Begin("loading")
	work.End("downloading")
	if names, _ := work.InProgress(); !slices.Equal(names, []string{"downloading", "loading"}) {
		t.Fatalf("in progress %v, want [downloading loading]", names)
	}
}
