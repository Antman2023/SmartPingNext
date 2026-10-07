package funcs

import (
	"context"
	"reflect"
	"smartping/src/g"
	"sync/atomic"
	"testing"
	"time"
)

func TestMappingConfigSnapshotCancellationPreservesResults(t *testing.T) {
	previous := g.ConfigSnapshot()
	defer g.SetConfig(previous)
	setMappingSnapshotFixture(t, map[string][]g.MapVal{"ctcc": {{Name: "existing", Value: 12.5}}})
	want := mappingStatusSnapshot()
	g.SetConfig(g.Config{Chinamap: map[string]map[string][]string{"province": {"ctcc": make([]string, 8192)}}})
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &mappingCancelOnCheckContext{Context: base, cancel: cancel, cancelAt: 20}
	MappingContext(ctx)
	if base.Err() != context.Canceled {
		t.Fatal("mapping did not cancel during target copy")
	}
	if atomic.LoadInt32(&mappingRunning) != 0 {
		t.Fatal("canceled mapping retained running guard")
	}
	if !g.CfgLock.TryLock() {
		t.Fatal("canceled mapping retained config lock")
	}
	g.CfgLock.Unlock()
	if !reflect.DeepEqual(want, mappingStatusSnapshot()) {
		t.Fatal("canceled preparation cleared previous results")
	}
}

func TestMappingFinishedContextSkipsConfigLock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	g.CfgLock.Lock()
	done := make(chan struct{})
	go func() { defer close(done); MappingContext(ctx) }()
	defer func() { g.CfgLock.Unlock(); <-done }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled mapping waited for config lock")
	}
}
