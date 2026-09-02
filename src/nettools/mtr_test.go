package nettools

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestRunMtrContextReturnsImmediatelyWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := RunMtrContext(ctx, "127.0.0.1", time.Second, 8, 3)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunMtrContext error = %v, want context canceled", err)
	}
	if len(res) != 0 {
		t.Fatalf("canceled MTR result = %#v, want empty", res)
	}
}

func TestResolveIPv4ContextRejectsIPv6Literal(t *testing.T) {
	if _, err := ResolveIPv4Context(context.Background(), "::1"); err == nil {
		t.Fatal("ResolveIPv4Context should reject IPv6-only literal")
	}
}

func TestResolveIPv4ContextHonorsCancellationBeforeLiteralParsing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := ResolveIPv4Context(ctx, "127.0.0.1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ResolveIPv4Context error = %v, want context canceled", err)
	}
}

func TestResolveIPv4ContextNormalizesLiteral(t *testing.T) {
	address, err := ResolveIPv4Context(context.Background(), " 127.0.0.1 ")
	if err != nil {
		t.Fatalf("ResolveIPv4Context returned error: %v", err)
	}
	if got := address.String(); got != "127.0.0.1" {
		t.Fatalf("resolved address = %q, want 127.0.0.1", got)
	}
}

func TestRunMtrInvalidHost(t *testing.T) {
	res, err := RunMtr("invalid host !@", time.Second, 8, 3)
	if err == nil {
		t.Fatalf("RunMtr should fail for invalid host")
	}
	if !strings.Contains(err.Error(), "Unable to resolve destination host") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("result should be empty on resolve failure")
	}
}

func TestRunMtrZeroTTL(t *testing.T) {
	res, err := RunMtr("invalid host !@", time.Second, 0, 3)
	if err != nil {
		t.Fatalf("RunMtr with maxttl=0 should not error, got: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("RunMtr with maxttl=0 should return empty result")
	}
}

func TestRunMtrRejectsInvalidLimitsBeforeSocketInitialization(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		ttl         int
		maxTimeouts int
	}{
		{name: "oversized TTL", timeout: time.Second, ttl: 256, maxTimeouts: 3},
		{name: "zero probe timeout", ttl: 8, maxTimeouts: 3},
		{name: "zero consecutive timeout limit", timeout: time.Second, ttl: 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := RunMtr("127.0.0.1", tt.timeout, tt.ttl, tt.maxTimeouts); err == nil {
				t.Fatal("RunMtr should reject invalid limits")
			}
		})
	}
}

func TestRunMtrReturnsICMPInitializationErrorImmediately(t *testing.T) {
	if err := pool.close(); err != nil {
		t.Fatalf("close shared ICMP pool before test: %v", err)
	}
	pool.initMu.Lock()
	previousListenPacket := pool.listenPacket
	wantErr := errors.New("ICMP unavailable")
	listenCalls := 0
	pool.listenPacket = func(_, _ string) (net.PacketConn, error) {
		listenCalls++
		return nil, wantErr
	}
	pool.initMu.Unlock()
	t.Cleanup(func() {
		_ = pool.close()
		pool.initMu.Lock()
		pool.listenPacket = previousListenPacket
		pool.initMu.Unlock()
	})

	result, err := RunMtrContext(context.Background(), "127.0.0.1", time.Second, 64, 6)
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunMtrContext error = %v, want %v", err, wantErr)
	}
	if listenCalls != 1 {
		t.Fatalf("ICMP initialization attempts = %d, want 1", listenCalls)
	}
	if len(result) != 0 {
		t.Fatalf("RunMtrContext returned data after initialization failure: %#v", result)
	}
}

func TestIsTerminalMtrResponse(t *testing.T) {
	tests := []struct {
		name     string
		response ICMP
		want     bool
	}{
		{name: "intermediate hop", response: ICMP{}, want: false},
		{name: "echo reply", response: ICMP{Final: true}, want: true},
		{name: "destination unreachable", response: ICMP{Down: true}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTerminalMtrResponse(tt.response); got != tt.want {
				t.Fatalf("isTerminalMtrResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRecordMtrDiscoveryProbeKeepsThresholdTimeout(t *testing.T) {
	results := make(map[int][]ICMP)
	timeouts, stop := recordMtrDiscoveryProbe(results, 1, ICMP{Timeout: true}, 0, 1)
	if timeouts != 1 || !stop {
		t.Fatalf("timeout state = (%d, %v), want (1, true)", timeouts, stop)
	}
	if len(results[1]) != 1 || !results[1][0].Timeout {
		t.Fatalf("threshold timeout was not retained: %#v", results)
	}
}

func TestRecordMtrDiscoveryProbeResetsConsecutiveTimeouts(t *testing.T) {
	results := make(map[int][]ICMP)
	timeouts, stop := recordMtrDiscoveryProbe(results, 2, ICMP{}, 3, 6)
	if timeouts != 0 || stop {
		t.Fatalf("timeout state after response = (%d, %v), want (0, false)", timeouts, stop)
	}
	if len(results[2]) != 1 {
		t.Fatalf("successful discovery probe was not retained: %#v", results)
	}
}

func TestSummarizeMtrInitializesBestFromFirstSuccessfulResponse(t *testing.T) {
	got := summarizeMtr([]ICMP{
		{Timeout: true},
		{Addr: &net.IPAddr{IP: net.ParseIP("192.0.2.1")}, RTT: 8 * time.Millisecond},
		{Addr: &net.IPAddr{IP: net.ParseIP("192.0.2.2")}, RTT: 3 * time.Millisecond},
	})

	if got.Send != 3 || got.Loss != 1 {
		t.Fatalf("send/loss = %d/%d, want 3/1", got.Send, got.Loss)
	}
	if got.Best != 3*time.Millisecond || got.Wrst != 8*time.Millisecond {
		t.Fatalf("best/worst = %v/%v, want 3ms/8ms", got.Best, got.Wrst)
	}
	if got.Avg != 5500*time.Microsecond || got.Last != 3*time.Millisecond {
		t.Fatalf("avg/last = %v/%v, want 5.5ms/3ms", got.Avg, got.Last)
	}
	if got.StDev != 2.5 {
		t.Fatalf("stdev = %v, want 2.5", got.StDev)
	}
}

func TestSummarizeMtrAllFailures(t *testing.T) {
	got := summarizeMtr([]ICMP{{Timeout: true}, {Error: errors.New("read failed")}})
	if got.Send != 2 || got.Loss != 2 {
		t.Fatalf("send/loss = %d/%d, want 2/2", got.Send, got.Loss)
	}
	if got.Best != 0 || got.Avg != 0 || got.Wrst != 0 || got.StDev != 0 {
		t.Fatalf("all-failure summary should not report latency: %#v", got)
	}
}
