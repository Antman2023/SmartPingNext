package funcs

import (
	"context"
	"smartping/src/g"
	"sync/atomic"
	"testing"
)

func TestStartCloudMonitorContextDoesNotStartWhenCanceled(t *testing.T) {
	atomic.StoreInt32(&cloudMonitorRunning, 0)
	t.Cleanup(func() { atomic.StoreInt32(&cloudMonitorRunning, 0) })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	StartCloudMonitorContext(ctx)

	if got := atomic.LoadInt32(&cloudMonitorRunning); got != 0 {
		t.Fatalf("cloud monitor state after canceled call = %d, want 0", got)
	}
}

func TestStartCloudMonitorSkipsOverlappingRun(t *testing.T) {
	atomic.StoreInt32(&cloudMonitorRunning, 1)
	t.Cleanup(func() { atomic.StoreInt32(&cloudMonitorRunning, 0) })

	StartCloudMonitor()

	if got := atomic.LoadInt32(&cloudMonitorRunning); got != 1 {
		t.Fatalf("cloud monitor state after overlapping call = %d, want 1", got)
	}
}

func TestStartCloudMonitorReleasesGuardAfterFailure(t *testing.T) {
	atomic.StoreInt32(&cloudMonitorRunning, 0)
	t.Cleanup(func() { atomic.StoreInt32(&cloudMonitorRunning, 0) })
	oldClient := g.HttpClient
	g.HttpClient = nil
	t.Cleanup(func() { g.HttpClient = oldClient })

	StartCloudMonitor()

	if got := atomic.LoadInt32(&cloudMonitorRunning); got != 0 {
		t.Fatalf("cloud monitor state after failure = %d, want 0", got)
	}
}
