package funcs

import (
	"context"
	"fmt"
	"reflect"
	"smartping/src/g"
	"sync/atomic"
	"testing"
	"time"
)

func TestPingRoundSkipsConfigLockForFinishedContext(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(fmt.Sprintf("expired=%v", expired), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if expired {
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			}
			defer cancel()
			g.CfgLock.Lock()
			done := make(chan struct{})
			go func() {
				defer close(done)
				runPingRoundContext(ctx, time.Now())
			}()
			defer func() {
				g.CfgLock.Unlock()
				<-done
			}()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("finished ping round waited for the configuration lock")
			}
		})
	}
}

type pingSnapshotCancelContext struct {
	context.Context
	cancel context.CancelFunc
	checks atomic.Int32
}

func (ctx *pingSnapshotCancelContext) Err() error {
	// Cancellation during target preparation prevents any probes from starting.
	if ctx.checks.Add(1) == 40 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestPingSnapshotCancellationReleasesGateAndConfigLock(t *testing.T) {
	previous := g.ConfigSnapshot()
	defer g.SetConfig(previous)
	oldGate := pingRoundGate
	pingRoundGate = newBoundedJobGate()
	defer func() { pingRoundGate = oldGate }()
	config := g.Config{Addr: "192.0.0.0", Network: make(map[string]g.NetworkMember)}
	for i := 0; i < 1024; i++ {
		addr := fmt.Sprintf("192.0.%d.%d", i/256, i%256)
		config.Network[addr] = g.NetworkMember{Addr: addr, Name: addr}
	}
	local := config.Network[config.Addr]
	for i := 0; i < 1024; i++ {
		local.Ping = append(local.Ping, fmt.Sprintf("192.0.%d.%d", i/256, i%256))
	}
	config.Network[config.Addr] = local
	g.SetConfig(config)
	before := g.ConfigSnapshot()
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &pingSnapshotCancelContext{Context: base, cancel: cancel}
	PingContext(ctx)
	if base.Err() != context.Canceled {
		t.Fatal("ping snapshot never observed cancellation while preparing the round")
	}
	if running, waiting := boundedJobGateState(pingRoundGate); running || waiting {
		t.Fatal("canceled snapshot retained a ping round slot")
	}
	if !g.CfgLock.TryLock() {
		t.Fatal("canceled snapshot retained the configuration read lock")
	}
	g.CfgLock.Unlock()
	if !reflect.DeepEqual(before, g.ConfigSnapshot()) {
		t.Fatal("canceled snapshot changed the source configuration")
	}
	// No targets or alert rules: recovery exercises the real round entrypoint
	// without raw sockets, DNS or a database fixture.
	local.Ping = nil
	config.Network[config.Addr] = local
	g.SetConfig(config)
	PingContext(context.Background())
	if running, waiting := boundedJobGateState(pingRoundGate); running || waiting {
		t.Fatal("subsequent ping round did not release its slot")
	}
}
