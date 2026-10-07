package g

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestLocalSnapshotContextPreservesValuesAndIndependence(t *testing.T) {
	withGlobalConfigState(t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		for _, member := range []NetworkMember{{}, {Ping: []string{}, Topology: []map[string]string{}},
			{Addr: "192.0.2.1", Name: "local", Smartping: true, Ping: []string{"192.0.2.2"}, Topology: []map[string]string{nil, {}, {"Addr": "192.0.2.2", "extension": "value"}}}} {
			// SetConfig normalizes nil lists; assign under the lock to test the
			// snapshot contract for both original representations.
			CfgLock.Lock()
			Cfg = Config{Addr: "192.0.2.1", Network: map[string]NetworkMember{"192.0.2.1": member}}
			CfgLock.Unlock()
			wantAddr, want := LocalNetworkSnapshot()
			for _, active := range []context.Context{context.Background(), ctx} {
				addr, got, err := LocalNetworkSnapshotContext(active)
				if err != nil || addr != wantAddr || !reflect.DeepEqual(got, want) {
					t.Fatalf("snapshot = %q, %#v, %v", addr, got, err)
				}
				if len(got.Ping) > 0 {
					got.Ping[0] = "changed"
					got.Topology[2]["extension"] = "changed"
				}
				_, after := LocalNetworkSnapshot()
				if !reflect.DeepEqual(after, want) {
					t.Fatal("snapshot aliases source data")
				}
			}
		}
		SetConfig(Config{Addr: "missing"})
		addr, member, err := LocalNetworkSnapshotContext(ctx)
		if err != nil || addr != "missing" || !reflect.DeepEqual(member, NetworkMember{}) {
			t.Fatal("missing member contract changed")
		}
	})
}

func TestLocalSnapshotContextSkipsLockWhenFinished(t *testing.T) {
	for _, expired := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if expired {
			ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		}
		CfgLock.Lock()
		done := make(chan error, 1)
		go func() { _, _, err := LocalNetworkSnapshotContext(ctx); done <- err }()
		select {
		case err := <-done:
			CfgLock.Unlock()
			cancel()
			if !errors.Is(err, ctx.Err()) {
				t.Fatalf("finished context error = %v", err)
			}
		case <-time.After(time.Second):
			CfgLock.Unlock()
			cancel()
			<-done
			t.Fatal("finished context waited for configuration lock")
		}
	}
}

func TestLocalSnapshotContextDiscardsCanceledCopy(t *testing.T) {
	withGlobalConfigState(t, func() {
		SetConfig(snapshotContextFixture(1, 1024, 0))
		before := ConfigSnapshot()
		base, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx := &snapshotCancelOnCheckContext{Context: base, cancel: cancel}
		addr, member, err := LocalNetworkSnapshotContext(ctx)
		if !errors.Is(err, context.Canceled) || addr != "" || !reflect.DeepEqual(member, NetworkMember{}) {
			t.Fatal("canceled snapshot returned partial data")
		}
		if !CfgLock.TryLock() {
			t.Fatal("canceled local snapshot retained read lock")
		}
		CfgLock.Unlock()
		if !reflect.DeepEqual(before, ConfigSnapshot()) {
			t.Fatal("canceled copy changed configuration")
		}
		addr, member, err = LocalNetworkSnapshotContext(context.Background())
		if err != nil || addr != before.Addr || !reflect.DeepEqual(member, before.Network[addr]) {
			t.Fatal("subsequent local snapshot failed")
		}
	})
}
