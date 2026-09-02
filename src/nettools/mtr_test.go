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
	if _, err := resolveIPv4Context(context.Background(), "::1"); err == nil {
		t.Fatal("resolveIPv4Context should reject IPv6-only literal")
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
