package nettools

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestWaiterKeyUsesIdentifierAndSequence(t *testing.T) {
	base := waiterKey(100, 200)
	if base == waiterKey(100, 201) {
		t.Fatalf("waiterKey should distinguish different sequence values")
	}
	if base == waiterKey(101, 200) {
		t.Fatalf("waiterKey should distinguish different identifier values")
	}
}

func TestNextICMPSequenceCoversFullCycleWithoutDuplicates(t *testing.T) {
	seen := make([]bool, 1<<16)
	for i := 0; i < len(seen); i++ {
		sequence := nextICMPSequence()
		if sequence < 0 || sequence >= len(seen) {
			t.Fatalf("ICMP sequence is outside 16-bit range: %d", sequence)
		}
		if seen[sequence] {
			t.Fatalf("ICMP sequence %d repeated before the full cycle completed", sequence)
		}
		seen[sequence] = true
	}
}

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

func TestRunPingContextReturnsBeforeSocketInitializationWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RunPingContext(ctx, &net.IPAddr{IP: net.ParseIP("127.0.0.1")}, time.Second, 64, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunPingContext error = %v, want context canceled", err)
	}
}

func TestRunPingContextRejectsInvalidArgumentsBeforeSocketInitialization(t *testing.T) {
	validAddress := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
	tests := []struct {
		name    string
		address *net.IPAddr
		timeout time.Duration
		ttl     int
	}{
		{name: "nil address", timeout: time.Second, ttl: 64},
		{name: "IPv6 address", address: &net.IPAddr{IP: net.ParseIP("2001:db8::1")}, timeout: time.Second, ttl: 64},
		{name: "zero timeout", address: validAddress, ttl: 64},
		{name: "zero TTL", address: validAddress, timeout: time.Second},
		{name: "oversized TTL", address: validAddress, timeout: time.Second, ttl: 256},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := RunPingContext(context.Background(), tt.address, tt.timeout, tt.ttl, 0); err == nil {
				t.Fatal("RunPingContext should reject invalid arguments")
			}
		})
	}
}

func TestWaitForICMPResponseHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := waitForICMPResponse(ctx, make(chan icmpResponse), time.Now(), time.Minute)
	if !errors.Is(result.Error, context.Canceled) {
		t.Fatalf("waitForICMPResponse error = %v, want context canceled", result.Error)
	}
	if result.Timeout || result.Final || result.Down {
		t.Fatalf("canceled response contains an unexpected terminal state: %#v", result)
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

func TestICMPPoolRetriesInitializationAfterFailure(t *testing.T) {
	wantErr := errors.New("temporary listen failure")
	listenCalls := 0
	pool := &icmpPool{
		listenPacket: func(_, _ string) (net.PacketConn, error) {
			listenCalls++
			return nil, wantErr
		},
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if err := pool.init(); !errors.Is(err, wantErr) {
			t.Fatalf("attempt %d error = %v, want %v", attempt, err, wantErr)
		}
	}
	if listenCalls != 2 {
		t.Fatalf("listen calls = %d, want retry on each initialization", listenCalls)
	}
	if pool.conn != nil || pool.ipconn != nil {
		t.Fatalf("failed initialization retained partial connection state")
	}
}
