package event

import (
	"testing"
	"time"
)

func TestBusPublishSubscribe(t *testing.T) {
	bus := NewBus(16)
	defer bus.Close()

	sub := bus.Subscribe()
	bus.Publish(Event{Type: EventProcessStarted, Process: "test"})

	select {
	case ev := <-sub.Ch:
		if ev.Type != EventProcessStarted {
			t.Errorf("expected EventProcessStarted, got %s", ev.Type)
		}
		if ev.Process != "test" {
			t.Errorf("expected process test, got %s", ev.Process)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestBusFiltering(t *testing.T) {
	bus := NewBus(16)
	defer bus.Close()

	sub := bus.Subscribe(EventProcessStarted)
	bus.Publish(Event{Type: EventProcessStopped, Process: "test"})
	bus.Publish(Event{Type: EventProcessStarted, Process: "test"})

	select {
	case ev := <-sub.Ch:
		if ev.Type != EventProcessStarted {
			t.Errorf("expected EventProcessStarted, got %s", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for filtered event")
	}

	// Should not receive the stopped event.
	select {
	case ev := <-sub.Ch:
		t.Errorf("unexpected event: %s", ev.Type)
	case <-time.After(100 * time.Millisecond):
		// Expected: no more events.
	}
}

func TestBusUnsubscribe(t *testing.T) {
	bus := NewBus(16)
	defer bus.Close()

	sub := bus.Subscribe()
	bus.Unsubscribe(sub.ID)

	// Channel should be closed.
	_, ok := <-sub.Ch
	if ok {
		t.Error("expected closed channel after unsubscribe")
	}
}

func TestBusMultipleSubscribers(t *testing.T) {
	bus := NewBus(16)
	defer bus.Close()

	sub1 := bus.Subscribe()
	sub2 := bus.Subscribe()

	bus.Publish(Event{Type: EventProcessStarted, Process: "test"})

	for _, sub := range []*Subscriber{sub1, sub2} {
		select {
		case ev := <-sub.Ch:
			if ev.Type != EventProcessStarted {
				t.Errorf("expected EventProcessStarted, got %s", ev.Type)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}
}

func TestBusTimestampAutoSet(t *testing.T) {
	bus := NewBus(16)
	defer bus.Close()

	sub := bus.Subscribe()
	before := time.Now()
	bus.Publish(Event{Type: EventProcessStarted})

	select {
	case ev := <-sub.Ch:
		if ev.Timestamp.Before(before) {
			t.Error("expected timestamp to be set automatically")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
