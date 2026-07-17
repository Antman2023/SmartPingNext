package nettools

import (
	"errors"
	"testing"
	"time"
)

func TestEvaluatePingResultRequiresFinalReply(t *testing.T) {
	if _, err := evaluatePingResult(ICMP{RTT: time.Millisecond}); err == nil {
		t.Fatalf("non-final ICMP response should not count as successful ping")
	}
	delay, err := evaluatePingResult(ICMP{RTT: 1500 * time.Microsecond, Final: true})
	if err != nil {
		t.Fatalf("final reply returned error: %v", err)
	}
	if delay != 1.5 {
		t.Fatalf("delay = %v, want 1.5ms", delay)
	}
}

func TestEvaluatePingResultPreservesErrors(t *testing.T) {
	want := errors.New("send failed")
	if _, err := evaluatePingResult(ICMP{Error: want}); !errors.Is(err, want) {
		t.Fatalf("evaluatePingResult error = %v, want %v", err, want)
	}
}

func TestICMPPoolRegisterRejectsCollision(t *testing.T) {
	pool := &icmpPool{waiters: make(map[uint32]chan icmpResponse)}
	if _, ok := pool.register(42); !ok {
		t.Fatalf("first waiter registration should succeed")
	}
	if _, ok := pool.register(42); ok {
		t.Fatalf("duplicate waiter registration should be rejected")
	}
}
