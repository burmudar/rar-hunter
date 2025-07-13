package eventbus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type dummyEvent struct {
	Value int
}

type otherDummyEvent struct {
	Value int
}

func TestEventBus(t *testing.T) {
	t.Run("subscription", func(t *testing.T) {
		bus := New(t.Context())
		bus.Start()
		defer bus.Stop(100 * time.Millisecond)

		expected1 := dummyEvent{Value: 10}
		expected2 := otherDummyEvent{Value: 20}
		var called1 atomic.Uint32
		var called2 atomic.Uint32
		var eventWg sync.WaitGroup
		var onEvent1 EventCallback[dummyEvent] = func(_ context.Context, received *dummyEvent) error {
			t.Helper()
			called1.Add(1)
			defer eventWg.Done()
			if received.Value != expected1.Value {
				t.Errorf("incorrect event received - got %d wanted %d", received.Value, expected1.Value)
			}
			return nil
		}
		var onEvent2 EventCallback[otherDummyEvent] = func(ctx context.Context, received *otherDummyEvent) error {
			t.Helper()
			called2.Add(1)
			defer eventWg.Done()
			if received.Value != expected2.Value {
				t.Errorf("incorrect event received - got %d wanted %d", received.Value, expected2.Value)
			}
			return nil
		}

		Subscribe(bus, dummyEvent{}, onEvent1)
		Subscribe(bus, otherDummyEvent{}, onEvent2)

		for range 100 {
			eventWg.Add(2)
			Publish(bus, expected1)
			Publish(bus, expected2)
		}

		done := make(chan struct{})
		go func() {
			eventWg.Wait()
			close(done)
		}()

		select {
		case <-time.After(1 * time.Second):
			t.Fatal("event receive timeout")
		case <-done:
			t.Logf("publishing done")
		}

		if called1.Load() != 100 {
			t.Errorf("[1] expected callback to be called %d - got %d", 100, called1.Load())
		}
		if called2.Load() != 100 {
			t.Errorf("[2] expected callback to be called %d - got %d", 100, called2.Load())
		}
		t.Logf("[1] called %d", called1.Load())
		t.Logf("[2] called %d", called2.Load())

	})
	t.Run("unsubscribe", func(t *testing.T) {
		bus := New(t.Context())
		bus.Start()
		defer bus.Stop(100 * time.Millisecond)

		expected := dummyEvent{Value: 10}
		eventCalled := make(chan struct{}, 1)
		var onEvent EventCallback[dummyEvent] = func(_ context.Context, received *dummyEvent) error {
			eventCalled <- struct{}{}
			t.Helper()
			if received.Value != expected.Value {
				t.Errorf("incorrect event received - got %d wanted %d", received.Value, expected.Value)
			}
			return nil
		}

		sub := Subscribe(bus, dummyEvent{}, onEvent)
		Publish(bus, expected)
		ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
		// Unsubscribe now, so called should stay at 1 if we publish again
		select {
		case <-eventCalled:
			cancel()
		case <-ctx.Done():
			t.Fatalf("eventcallback not called within timeout")
		}
		sub.Close()
		Publish(bus, expected) // this should have no affect
		select {
		case <-time.After(100 * time.Millisecond):
			t.Logf("event not called due to unsubscription")
		case <-eventCalled:
			t.Errorf("callback for event should not be called after unscribing")
		}
	})
	t.Run("publish with no subscribers", func(t *testing.T) {
		bus := New(t.Context())
		bus.Start()
		defer bus.Stop(100 * time.Millisecond)
		defer func() {
			t.Helper()
			if err := recover(); err != nil {
				t.Fatalf("publishing with no subscribers should not panic")
			}
		}()

		Publish(bus, dummyEvent{})
	})
	t.Run("publish on stopped eventbus", func(t *testing.T) {
		bus := New(t.Context())
		// shoud not panic if we haven't started
		Publish(bus, dummyEvent{})
		bus.Start()
		bus.Stop(100 * time.Millisecond)
		// should also not panic if we explicitly stopped
		Publish(bus, dummyEvent{})
		defer func() {
			t.Helper()
			if err := recover(); err != nil {
				t.Fatalf("publishing on stopped eventbus should not panic")
			}
		}()
	})
	t.Run("calling stop on not started eventbus", func(t *testing.T) {
		bus := New(t.Context())
		bus.Stop(100 * time.Millisecond)
		defer func() {
			t.Helper()
			if err := recover(); err != nil {
				t.Fatalf("publishing on stopped eventbus should not panic")
			}
		}()
	})
}
