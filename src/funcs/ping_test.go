package funcs

import (
	"context"
	"smartping/src/g"
	"sync/atomic"
	"testing"
	"time"
)

func TestPingContextDoesNotStartCanceledRound(t *testing.T) {
	atomic.StoreInt32(&pingRunning, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	PingContext(ctx)
	if got := atomic.LoadInt32(&pingRunning); got != 0 {
		t.Fatalf("pingRunning = %d after canceled call, want 0", got)
	}
}

func TestPingSkipsOverlappingRound(t *testing.T) {
	atomic.StoreInt32(&pingRunning, 1)
	defer atomic.StoreInt32(&pingRunning, 0)

	Ping()
	if got := atomic.LoadInt32(&pingRunning); got != 1 {
		t.Fatalf("pingRunning = %d, want existing round to remain active", got)
	}
}

func TestResolvePingRoundConfigBoundsValuesAndAllowsZeroStagger(t *testing.T) {
	config := g.Config{Base: map[string]int{
		"PingCount":      1000000,
		"PingIntervalMs": 1,
		"PingTimeoutMs":  1000000,
		"PingStaggerMs":  0,
	}}

	count, interval, timeout, stagger := resolvePingRoundConfig(config)
	if count != defaultPingCount || interval != defaultPingIntervalMs*time.Millisecond || timeout != defaultPingTimeoutMs*time.Millisecond {
		t.Fatalf("invalid values should fall back to defaults: count=%d interval=%v timeout=%v", count, interval, timeout)
	}
	if stagger != 0 {
		t.Fatalf("zero stagger should remain disabled, got %v", stagger)
	}
}

func TestBoundedBaseInt(t *testing.T) {
	config := g.Config{Base: map[string]int{"value": 65}}
	if got := boundedBaseInt(config, "value", 8, 1, 64); got != 8 {
		t.Fatalf("boundedBaseInt out-of-range value = %d, want default 8", got)
	}
}

func TestPingTargetOffsetStaysWithinInterval(t *testing.T) {
	interval := 3 * time.Second
	stagger := 100 * time.Millisecond
	tests := []struct {
		index int
		want  time.Duration
	}{
		{index: 0, want: 0},
		{index: 1, want: 100 * time.Millisecond},
		{index: 29, want: 2900 * time.Millisecond},
		{index: 30, want: 0},
		{index: 1023, want: 300 * time.Millisecond},
	}
	for _, tt := range tests {
		if got := pingTargetOffset(tt.index, stagger, interval); got != tt.want {
			t.Fatalf("pingTargetOffset(%d) = %v, want %v", tt.index, got, tt.want)
		}
	}

	if got := pingTargetOffset(10, 0, interval); got != 0 {
		t.Fatalf("zero stagger offset = %v, want 0", got)
	}
	if got := pingTargetOffset(10, stagger, 0); got != 0 {
		t.Fatalf("zero interval offset = %v, want 0", got)
	}
}
