package funcs

import (
	"sync/atomic"
	"testing"
)

func TestPingSkipsOverlappingRound(t *testing.T) {
	atomic.StoreInt32(&pingRunning, 1)
	defer atomic.StoreInt32(&pingRunning, 0)

	Ping()
	if got := atomic.LoadInt32(&pingRunning); got != 1 {
		t.Fatalf("pingRunning = %d, want existing round to remain active", got)
	}
}
