package nettools

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMtrProbeSpacingIncludesDiscovery(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		slowProbe int
	}{{"fast responses", -1}, {"slow discovery", 0}, {"slow sampling", 3}} {
		t.Run(scenario.name, func(t *testing.T) {
			const interval = 15 * time.Millisecond
			var starts []time.Time
			result, err := runMtrProbesContext(context.Background(), 8, 3, interval, func(ctx context.Context, ttl int) ICMP {
				index := len(starts)
				starts = append(starts, time.Now())
				if index == scenario.slowProbe {
					if err := waitForContext(ctx, 4*interval); err != nil {
						return ICMP{Error: err}
					}
				}
				if ttl != 1 {
					t.Errorf("terminal discovery continued to TTL %d", ttl)
				}
				return ICMP{Final: true, Addr: &net.IPAddr{IP: net.IPv4(127, 0, 0, 1)}, RTT: time.Millisecond}
			})
			if err != nil || len(result) != 1 || result[0].Send != mtrProbeCount || result[0].Loss != 0 {
				t.Fatalf("MTR result = %+v, %v, want one hop with ten successful probes", result, err)
			}
			if len(starts) != mtrProbeCount {
				t.Fatalf("probe count = %d", len(starts))
			}
			for i := 1; i < len(starts); i++ {
				if gap := starts[i].Sub(starts[i-1]); gap < interval-time.Millisecond {
					t.Errorf("probe %d follows probe %d after %v, want at least %v", i, i-1, gap, interval)
				}
			}
		})
	}
}

func TestMtrCancellationDuringFirstInterval(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "deadline"}[deadline], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
			}
			defer cancel()
			var count atomic.Int32
			done := make(chan error, 1)
			go func() {
				_, err := runMtrProbesContext(ctx, 8, 3, time.Second, func(context.Context, int) ICMP {
					count.Add(1)
					return ICMP{Final: true}
				})
				done <- err
			}()
			if !deadline {
				limit := time.Now().Add(time.Second)
				for count.Load() == 0 && time.Now().Before(limit) {
					time.Sleep(time.Millisecond)
				}
				// Give the worker time to enter the first interval wait.
				time.Sleep(30 * time.Millisecond)
				cancel()
			}
			select {
			case err := <-done:
				want := context.Canceled
				if deadline {
					want = context.DeadlineExceeded
				}
				if !errors.Is(err, want) {
					t.Errorf("MTR error = %v, want %v", err, want)
				}
			case <-time.After(time.Second):
				t.Fatal("MTR did not interrupt the interval wait")
			}
			if got := count.Load(); got != 1 {
				t.Errorf("probes before cancellation = %d, want discovery only", got)
			}
		})
	}
}

func TestMtrHopSamplingRemainsConcurrent(t *testing.T) {
	var mu sync.Mutex
	counts := make(map[int]int)
	active := make(chan int, 3)
	release := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		result, err := runMtrProbesContext(ctx, 8, 3, 5*time.Millisecond, func(ctx context.Context, ttl int) ICMP {
			mu.Lock()
			counts[ttl]++
			count := counts[ttl]
			mu.Unlock()
			if count == 2 {
				active <- ttl
				select {
				case <-release:
				case <-ctx.Done():
					return ICMP{Error: ctx.Err()}
				}
			}
			return ICMP{Final: ttl == 3, RTT: time.Duration(ttl) * time.Millisecond}
		})
		if err == nil {
			if len(result) != 3 {
				t.Errorf("MTR hop count = %d, want 3", len(result))
			}
			for _, hop := range result {
				if hop.Send != mtrProbeCount {
					t.Errorf("hop probe count = %d, want %d", hop.Send, mtrProbeCount)
				}
			}
		}
		done <- err
	}()
	hops := make(map[int]bool)
	for len(hops) < 3 {
		select {
		case ttl := <-active:
			hops[ttl] = true
		case <-ctx.Done():
			close(release)
			<-done
			t.Fatal("hop sampling became serial")
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestMtrSamplingErrorCancelsOtherHopsAndDiscovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	wantErr := errors.New("probe connection failed")
	var mu sync.Mutex
	counts := make(map[int]int)
	blocked := make(chan struct{})
	samplingBlocked := make(chan struct{})
	var stopped atomic.Int32
	result, err := runMtrProbesContext(ctx, 8, 3, 5*time.Millisecond, func(ctx context.Context, ttl int) ICMP {
		mu.Lock()
		counts[ttl]++
		count := counts[ttl]
		mu.Unlock()
		if ttl == 2 && count == 2 {
			select {
			case <-blocked:
				select {
				case <-samplingBlocked:
					return ICMP{Error: wantErr}
				case <-ctx.Done():
					return ICMP{Error: ctx.Err()}
				}
			case <-ctx.Done():
				return ICMP{Error: ctx.Err()}
			}
		}
		if ttl == 3 || (ttl == 1 && count == 2) {
			if ttl == 3 {
				close(blocked)
			} else {
				close(samplingBlocked)
			}
			<-ctx.Done()
			stopped.Add(1)
			return ICMP{Error: ctx.Err()}
		}
		return ICMP{}
	})
	if !errors.Is(err, wantErr) || len(result) != 0 {
		t.Fatalf("MTR returned %+v, %v, want original probe error", result, err)
	}
	if got := stopped.Load(); got != 2 {
		t.Errorf("canceled probes = %d, want sampling and discovery", got)
	}
}

func TestMtrDiscoveryTimeoutLimitRetainsFinalHop(t *testing.T) {
	result, err := runMtrProbesContext(context.Background(), 8, 3, time.Millisecond, func(context.Context, int) ICMP {
		return ICMP{Timeout: true}
	})
	if err != nil || len(result) != 3 {
		t.Fatalf("MTR result = %+v, %v, want three timeout hops", result, err)
	}
	for i, hop := range result {
		want := mtrProbeCount
		if i == 2 {
			want = 1
		}
		if hop.Send != want || hop.Loss != want {
			t.Errorf("hop %d send/loss = %d/%d, want %d/%d", i+1, hop.Send, hop.Loss, want, want)
		}
	}
}
