package contextlock

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type observedContext struct {
	context.Context
	once    sync.Once
	waiting chan struct{}
}

func (ctx *observedContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.waiting) })
	return ctx.Context.Done()
}

func TestContextMutexCancellationDuringHandoffKeepsSlotReusable(t *testing.T) {
	var mutex Mutex
	for i := range 200 {
		mutex.Lock()
		ctx, cancel := context.WithCancel(context.Background())
		observed := &observedContext{Context: ctx, waiting: make(chan struct{})}
		result := make(chan error, 1)
		go func() { result <- mutex.LockContext(observed) }()
		select {
		case <-observed.waiting:
		case <-time.After(time.Second):
			cancel()
			mutex.Unlock()
			t.Fatalf("handoff %d did not enter the queue", i)
		}
		if i%2 == 0 {
			cancel()
			mutex.Unlock()
		} else {
			mutex.Unlock()
			cancel()
		}
		select {
		case err := <-result:
			if err == nil {
				mutex.Unlock()
			} else if !errors.Is(err, context.Canceled) {
				t.Fatalf("handoff %d: %v", i, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("handoff %d did not finish", i)
		}
		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
		err := mutex.LockContext(ctx)
		cancel()
		if err != nil {
			t.Fatalf("handoff %d leaked the slot: %v", i, err)
		}
		mutex.Unlock()
	}
}
