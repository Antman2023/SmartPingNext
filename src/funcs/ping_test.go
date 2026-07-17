package funcs

import (
	"smartping/src/g"
	"sync/atomic"
	"testing"
	"time"
)

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
