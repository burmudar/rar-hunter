package eventbus

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"sync"
	"time"
)

type EventCallback[T any] func(ctx context.Context, e *T) error

type Subscription interface {
	Close()
}

type Subscriber struct {
	id        int
	eventType reflect.Type
	handler   func(ctx context.Context, event any) error
	bus       *EventBus
}

type eventWorker struct {
	queue chan *job
	bus   *EventBus
}

type job struct {
	event     any
	eventType reflect.Type
}

func (e *eventWorker) run(ctx context.Context) {
	for {
		select {
		case j := <-e.queue:
			err := e.handleJob(ctx, j)
			if err != nil {
				fmt.Printf("handleEvent failure in worker: %v\n", err)
			}
		case <-ctx.Done():
			fmt.Println("ctx cancelled in worker")
			return
		}
	}
}

func (e *eventWorker) handleJob(ctx context.Context, j *job) error {
	subs := e.bus.getSubscribers(j.eventType)

	jobCtx, jobCancel := context.WithTimeout(ctx, 30*time.Second)
	defer jobCancel()

	var err error

	for _, s := range subs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			otherErr := e.invokeHandler(jobCtx, s, j.event)
			err = errors.Join(err, otherErr)
		}
	}
	return err
}

func (e *eventWorker) invokeHandler(ctx context.Context, sub *Subscriber, event any) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return sub.handler(ctx, event)
	}
}

type EventBus struct {
	subscribers map[reflect.Type][]*Subscriber
	workers     []*eventWorker
	jobQueue    chan *job
	jobs        sync.WaitGroup
	mu          sync.RWMutex
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	running     bool
}

func (e *EventBus) getSubscribers(t reflect.Type) []*Subscriber {
	e.mu.RLock()
	defer e.mu.RUnlock()
	items, ok := e.subscribers[t]
	if ok {
		return slices.Clone(items)
	}

	return []*Subscriber{}
}

func New(ctx context.Context) *EventBus {
	ctx, cancel := context.WithCancel(ctx)
	return &EventBus{
		subscribers: map[reflect.Type][]*Subscriber{},
		ctx:         ctx,
		cancel:      cancel,
		mu:          sync.RWMutex{},
	}
}

func (e *EventBus) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return
	}
	e.jobQueue = make(chan *job, 100)
	e.workers = []*eventWorker{
		{
			queue: e.jobQueue,
			bus:   e,
		}}

	for _, w := range e.workers {
		e.startWorker(w)
	}

	e.running = true
}

func (e *EventBus) startWorker(w *eventWorker) {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		w.run(e.ctx)
	}()
}

func (e *EventBus) Stop(timeout time.Duration) {
	e.mu.RLock()
	if !e.running {
		e.mu.RUnlock()
		return
	}
	e.mu.RUnlock()
	done := make(chan struct{})
	go func() {
		// wait for workers to exit
		e.wg.Wait()
		close(done)
		// workers exited
	}()

	// signal workers to exit
	e.cancel()

	select {
	case <-done:
		fmt.Println("all wg items exited")
	case <-time.After(timeout):
		fmt.Printf("eventBus did not stop after timeout of %s\n", timeout)
	}

	close(e.jobQueue)
	e.mu.Lock()
	e.running = false
	e.mu.Unlock()
}

func (s *Subscriber) Close() {
	Unsubscribe(s.bus, s)
}

func Unsubscribe(bus *EventBus, sub *Subscriber) {
	subs := bus.getSubscribers(sub.eventType)
	subs = slices.DeleteFunc(subs, func(v *Subscriber) bool { return sub.id == v.id })
	bus.mu.Lock()
	bus.subscribers[sub.eventType] = subs
	bus.mu.Unlock()
}

func Subscribe[T any](bus *EventBus, event T, cb EventCallback[T]) Subscription {
	id := rand.Int()
	eventType := reflect.TypeOf(event)
	sub := Subscriber{
		id:        id,
		eventType: eventType,
		// wrap the function so that we can capture the type
		handler: func(ctx context.Context, event any) error {
			value, ok := event.(T)
			if ok {
				return cb(ctx, &value)
			}
			return nil
		},
		bus: bus,
	}

	bus.mu.Lock()
	defer bus.mu.Unlock()
	items := bus.subscribers[eventType]
	bus.subscribers[eventType] = append(items, &sub)
	return &sub
}

func Publish[T any](bus *EventBus, event T) {
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	if !bus.running {
		return
	}
	select {
	case <-bus.ctx.Done():
		return
	default:
		bus.wg.Add(1)
		go func() {
			defer bus.wg.Done()
			bus.jobQueue <- &job{
				event:     event,
				eventType: reflect.TypeOf(event),
			}
		}()
	}
}
