package nettools

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

type observedICMPContext struct {
	context.Context
	checked     chan error
	waiting     chan struct{}
	checkedOnce sync.Once
	waitingOnce sync.Once
}

func (ctx *observedICMPContext) Err() error {
	err := ctx.Context.Err()
	ctx.checkedOnce.Do(func() { ctx.checked <- err })
	return err
}

func (ctx *observedICMPContext) Done() <-chan struct{} {
	ctx.waitingOnce.Do(func() { close(ctx.waiting) })
	return ctx.Context.Done()
}

func observeICMPContext(ctx context.Context) *observedICMPContext {
	return &observedICMPContext{Context: ctx, checked: make(chan error, 1), waiting: make(chan struct{})}
}

func waitForQueuedICMPSender(t *testing.T, p *icmpPool, ctx *observedICMPContext, key uint32) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.waiting:
			return // A cancellable lock has entered its wait.
		default:
		}
		p.mu.RLock()
		_, registered := p.waiters[key]
		p.mu.RUnlock()
		if registered {
			return // The old implementation registered before waiting on sendMu.
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("sender did not enter the queue")
}

func TestICMPPoolContextCanExitWhileWaitingForLocks(t *testing.T) {
	for _, lockName := range []string{"initialization", "send"} {
		for _, withDeadline := range []bool{false, true} {
			name := lockName + "/cancel"
			if withDeadline {
				name = lockName + "/deadline"
			}
			t.Run(name, func(t *testing.T) {
				receiver, err := net.ListenPacket("udp4", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer receiver.Close()
				localPool := &icmpPool{listenPacket: func(_, _ string) (net.PacketConn, error) {
					return net.ListenPacket("udp4", "127.0.0.1:0")
				}}
				defer localPool.close()
				var lock sync.Locker = &localPool.initMu
				if lockName == "send" {
					if err := localPool.init(); err != nil {
						t.Fatal(err)
					}
					lock = &localPool.sendMu
				}
				lock.Lock()
				held := true
				result := make(chan ICMP, 1)
				finished := false
				defer func() {
					if held {
						lock.Unlock()
					}
					if !finished {
						select {
						case <-result:
						case <-time.After(time.Second):
							t.Error("sender did not stop during cleanup")
						}
					}
				}()
				base, cancel := context.WithCancel(context.Background())
				wantErr := error(context.Canceled)
				if withDeadline {
					cancel()
					base, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
					wantErr = context.DeadlineExceeded
				}
				defer cancel()
				ctx := observeICMPContext(base)
				go func() {
					result <- localPool.sendICMPContext(ctx, 123, 456, 64, []byte("canceled"), receiver.LocalAddr(), time.Millisecond)
				}()
				select {
				case err := <-ctx.checked:
					if err != nil {
						t.Fatalf("context expired before entering the request: %v", err)
					}
				case <-time.After(time.Second):
					t.Fatal("request did not start")
				}
				if lockName == "send" {
					waitForQueuedICMPSender(t, localPool, ctx, waiterKey(123, 456))
				}
				if !withDeadline {
					cancel()
				}
				select {
				case response := <-result:
					finished = true
					if !errors.Is(response.Error, wantErr) || response.Timeout {
						t.Fatalf("queued request = %+v, want %v", response, wantErr)
					}
				case <-time.After(time.Second):
					t.Fatal("request could not exit while the shared lock was held")
				}
				localPool.mu.RLock()
				waiters := len(localPool.waiters)
				localPool.mu.RUnlock()
				if waiters != 0 {
					t.Fatalf("canceled request left %d waiters", waiters)
				}
				lock.Unlock()
				held = false
				next := localPool.sendICMP(123, 456, 64, []byte("next"), receiver.LocalAddr(), time.Millisecond)
				if !next.Timeout || next.Error != nil {
					t.Fatalf("next request could not use the released slot: %+v", next)
				}
				if err := receiver.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				packet := make([]byte, 32)
				n, _, err := receiver.ReadFrom(packet)
				if err != nil || string(packet[:n]) != "next" {
					t.Fatalf("received %q, error %v; canceled request must never be sent", packet[:n], err)
				}
			})
		}
	}
}

func TestICMPPoolQueuedSenderDoesNotBlockResponseDispatch(t *testing.T) {
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	localPool := &icmpPool{conn: conn, waiters: make(map[uint32]icmpWaiter)}
	defer localPool.close()
	responses, _ := localPool.register(42, nil)
	localPool.sendMu.Lock()
	held := true
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := observeICMPContext(base)
	senderDone := make(chan ICMP, 1)
	senderFinished := false
	dispatchDone := make(chan struct{})
	defer func() {
		cancel()
		if held {
			localPool.sendMu.Unlock()
		}
		if !senderFinished {
			select {
			case <-senderDone:
			case <-time.After(time.Second):
				t.Error("queued sender did not stop")
			}
		}
		select {
		case <-dispatchDone:
		case <-time.After(time.Second):
			t.Error("dispatch did not finish")
		}
	}()
	go func() {
		senderDone <- localPool.sendICMPContext(ctx, 123, 456, 64, nil, conn.LocalAddr(), time.Second)
	}()
	waitForQueuedICMPSender(t, localPool, ctx, waiterKey(123, 456))
	go func() {
		localPool.dispatchFrom(conn, 42, icmpResponse{down: true})
		close(dispatchDone)
	}()
	select {
	case response := <-responses:
		if !response.down {
			t.Fatal("incorrect response delivered")
		}
	case <-time.After(time.Second):
		t.Fatal("queued sender blocked another probe's response")
	}
	cancel()
	select {
	case result := <-senderDone:
		senderFinished = true
		if !errors.Is(result.Error, context.Canceled) {
			t.Fatalf("queued request = %+v, want cancellation", result)
		}
	case <-time.After(time.Second):
		t.Fatal("queued request did not stop while another probe was active")
	}
	localPool.mu.RLock()
	_, active := localPool.waiters[42]
	count := len(localPool.waiters)
	localPool.mu.RUnlock()
	if !active || count != 1 {
		t.Fatal("canceling the queued sender changed another probe's waiting state")
	}
	localPool.sendMu.Unlock()
	held = false
}
