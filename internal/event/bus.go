// Package event provides an in-process event bus for process lifecycle events.
package event

import (
	"sync"
	"time"
)

// EventType represents the type of event.
type EventType string

const (
	EventProcessStarted   EventType = "process_started"
	EventProcessStopped   EventType = "process_stopped"
	EventProcessExited    EventType = "process_exited"
	EventProcessFatal     EventType = "process_fatal"
	EventProcessBackoff   EventType = "process_backoff"
	EventHealthCheckFail  EventType = "health_check_fail"
	EventScheduleRun      EventType = "schedule_run"
	EventScheduleComplete EventType = "schedule_complete"
	EventConfigReloaded   EventType = "config_reloaded"
)

// Event represents a system event.
type Event struct {
	Type      EventType
	Process   string
	Timestamp time.Time
	Data      map[string]interface{}
}

// Subscriber receives events.
type Subscriber struct {
	ID       int
	Ch       chan Event
	Filter   []EventType // if empty, receives all events
}

// Bus is an in-process event bus with topic filtering.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[int]*Subscriber
	nextID      int
	buffer      int
}

// NewBus creates a new event bus.
func NewBus(buffer int) *Bus {
	return &Bus{
		subscribers: make(map[int]*Subscriber),
		buffer:      buffer,
	}
}

// Subscribe creates a new subscription. Returns the subscriber ID.
func (b *Bus) Subscribe(filter ...EventType) *Subscriber {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	sub := &Subscriber{
		ID:     b.nextID,
		Ch:     make(chan Event, b.buffer),
		Filter: filter,
	}
	b.subscribers[sub.ID] = sub
	return sub
}

// Unsubscribe removes a subscription.
func (b *Bus) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if sub, ok := b.subscribers[id]; ok {
		close(sub.Ch)
		delete(b.subscribers, id)
	}
}

// Publish sends an event to all matching subscribers.
func (b *Bus) Publish(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, sub := range b.subscribers {
		if b.matches(sub, event.Type) {
			select {
			case sub.Ch <- event:
			default:
				// Drop event if subscriber is slow.
			}
		}
	}
}

func (b *Bus) matches(sub *Subscriber, eventType EventType) bool {
	if len(sub.Filter) == 0 {
		return true
	}
	for _, f := range sub.Filter {
		if f == eventType {
			return true
		}
	}
	return false
}

// Close closes the event bus and all subscriber channels.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, sub := range b.subscribers {
		close(sub.Ch)
	}
	b.subscribers = make(map[int]*Subscriber)
}