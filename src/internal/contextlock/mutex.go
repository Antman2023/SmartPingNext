package contextlock

import (
	"context"
	"sync"
)

// Mutex is a zero-value-ready lock whose queued callers may cancel.
// Like sync.Mutex, a Mutex must not be copied after first use.
// Waiting does not spawn a goroutine to acquire the lock.
type Mutex struct {
	once   sync.Once
	locked chan struct{}
}

func (mutex *Mutex) init() {
	mutex.once.Do(func() { mutex.locked = make(chan struct{}, 1) })
}

func (mutex *Mutex) Lock() {
	mutex.init()
	mutex.locked <- struct{}{}
}

func (mutex *Mutex) LockContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mutex.init()
	select {
	case mutex.locked <- struct{}{}:
		// Cancellation may win at the same time as the previous holder releases.
		if err := ctx.Err(); err != nil {
			mutex.Unlock()
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (mutex *Mutex) Unlock() {
	select {
	case <-mutex.locked:
	default:
		panic("unlock of unlocked context mutex")
	}
}
