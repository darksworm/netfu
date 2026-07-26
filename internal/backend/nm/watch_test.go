package nm

import (
	"testing"
	"time"

	"github.com/ilmars/netfu/internal/domain"
)

func TestStrengthGate_PassesFirstEventPerAP(t *testing.T) {
	g := newStrengthGate()
	now := time.Unix(1000, 0)
	if !g.allow("ap/1", now) {
		t.Error("first event for an AP should pass")
	}
	if !g.allow("ap/2", now) {
		t.Error("a different AP is gated independently")
	}
}

func TestStrengthGate_DropsEventsWithinASecondOfTheLastEmitted(t *testing.T) {
	g := newStrengthGate()
	now := time.Unix(1000, 0)
	g.allow("ap/1", now)
	if g.allow("ap/1", now.Add(500*time.Millisecond)) {
		t.Error("event 500ms after the last emitted should be dropped")
	}
	if !g.allow("ap/1", now.Add(time.Second)) {
		t.Error("event 1s after the last emitted should pass")
	}
}

func TestStrengthGate_DroppedEventsDoNotResetTheWindow(t *testing.T) {
	g := newStrengthGate()
	now := time.Unix(1000, 0)
	g.allow("ap/1", now)
	g.allow("ap/1", now.Add(900*time.Millisecond)) // dropped
	if !g.allow("ap/1", now.Add(1100*time.Millisecond)) {
		t.Error("window is measured from the last emitted event, not the last dropped one")
	}
}

func strengthEvent(name string) domain.Event {
	return domain.Event{Kind: domain.EventAPStrength, DeviceName: name}
}

func stateEvent(name string) domain.Event {
	return domain.Event{Kind: domain.EventDeviceChanged, DeviceName: name}
}

func TestEventBuffer_DeliversInFIFOOrder(t *testing.T) {
	b := newEventBuffer(4)
	b.push(stateEvent("a"))
	b.push(stateEvent("b"))
	if e, ok := b.pop(); !ok || e.DeviceName != "a" {
		t.Errorf("first pop = %v, %v", e, ok)
	}
	if e, ok := b.pop(); !ok || e.DeviceName != "b" {
		t.Errorf("second pop = %v, %v", e, ok)
	}
	if _, ok := b.pop(); ok {
		t.Error("empty buffer should report not-ok")
	}
}

func TestEventBuffer_WhenFullEvictsTheOldestStrengthEvent(t *testing.T) {
	b := newEventBuffer(3)
	b.push(strengthEvent("s1"))
	b.push(stateEvent("d1"))
	b.push(strengthEvent("s2"))
	b.push(stateEvent("d2")) // full: s1 is evicted

	var got []string
	for {
		e, ok := b.pop()
		if !ok {
			break
		}
		got = append(got, e.DeviceName)
	}
	want := []string{"d1", "s2", "d2"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestEventBuffer_NeverDropsStateEventsEvenWhenFull(t *testing.T) {
	b := newEventBuffer(2)
	b.push(stateEvent("d1"))
	b.push(stateEvent("d2"))
	b.push(stateEvent("d3")) // full of state events: grows instead of dropping

	var got []string
	for {
		e, ok := b.pop()
		if !ok {
			break
		}
		got = append(got, e.DeviceName)
	}
	if len(got) != 3 {
		t.Fatalf("got %d events, want all 3 state events kept: %v", len(got), got)
	}
}
