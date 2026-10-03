package funcs

import (
	"context"
	"errors"
	"fmt"
	"smartping/src/g"
	"testing"
	"time"
)

func TestScheduledPingProbesDoNotCatchUpAfterSlowProbe(t *testing.T) {
	const interval = 30 * time.Millisecond
	var starts []time.Time
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := runScheduledPingProbesContext(ctx, 4, interval, time.Now(), func(seq int) {
		if seq != len(starts) {
			t.Fatalf("probe sequence = %d, want %d", seq, len(starts))
		}
		starts = append(starts, time.Now())
		if seq == 0 {
			time.Sleep(4 * interval)
		}
	})
	if err != nil || len(starts) != 4 {
		t.Fatalf("probes = %d, error = %v, want 4 completed probes", len(starts), err)
	}
	for i := 1; i < len(starts); i++ {
		// The scheduler timestamps just before calling the probe. Allow a small
		// difference from this callback's timestamp on high-resolution clocks.
		if spacing := starts[i].Sub(starts[i-1]); spacing < interval-time.Millisecond {
			t.Errorf("probe %d started %v after the previous probe, want at least %v", i, spacing, interval)
		}
	}
}

func TestScheduledPingProbesHonorFirstStartAndFastProbeInterval(t *testing.T) {
	const interval = 20 * time.Millisecond
	firstStart := time.Now().Add(2 * interval)
	var starts []time.Time
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := runScheduledPingProbesContext(ctx, 3, interval, firstStart, func(int) {
		starts = append(starts, time.Now())
	})
	if err != nil || len(starts) != 3 {
		t.Fatalf("probes = %d, error = %v, want 3 completed probes", len(starts), err)
	}
	if starts[0].Before(firstStart) {
		t.Fatalf("first probe started at %v before its staggered start %v", starts[0], firstStart)
	}
	for i := 1; i < len(starts); i++ {
		if spacing := starts[i].Sub(starts[i-1]); spacing < interval-time.Millisecond {
			t.Errorf("probe %d interval = %v, want at least %v", i, spacing, interval)
		}
	}
}

func TestScheduledPingProbesCanceledBeforeFirstStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runScheduledPingProbesContext(ctx, 3, time.Second, time.Now().Add(time.Hour), func(int) {
		t.Fatal("canceled schedule started a probe")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err = runScheduledPingProbesContext(ctx, 3, time.Second, time.Now().Add(time.Hour), func(int) {
		t.Fatal("probe started before the deadline interrupted its stagger wait")
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}

func TestScheduledPingProbesCancellationStopsFurtherProbes(t *testing.T) {
	for _, cancelDuringProbe := range []bool{false, true} {
		t.Run(fmt.Sprint("during probe=", cancelDuringProbe), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			started := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- runScheduledPingProbesContext(ctx, 4, time.Hour, time.Now(), func(int) {
					calls++
					if cancelDuringProbe {
						cancel()
					}
					close(started)
				})
			}()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("first probe did not start")
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) || calls != 1 {
					t.Fatalf("calls = %d, error = %v, want one probe and cancellation", calls, err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation did not interrupt the probe schedule")
			}
		})
	}
}

func TestPingContextDoesNotStartCanceledRound(t *testing.T) {
	oldGate := pingRoundGate
	pingRoundGate = newBoundedJobGate()
	defer func() { pingRoundGate = oldGate }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	PingContext(ctx)
	running, waiting := boundedJobGateState(pingRoundGate)
	if running || waiting {
		t.Fatalf("gate state after canceled call = (running=%v, waiting=%v), want both false", running, waiting)
	}
}

func TestBoundedJobGateQueuesOneRoundAndRejectsAdditionalWork(t *testing.T) {
	gate := newBoundedJobGate()
	if acquired, queued := gate.acquire(context.Background()); !acquired || queued {
		t.Fatalf("first acquire = (%v, %v), want (true, false)", acquired, queued)
	}

	type acquireResult struct {
		acquired bool
		queued   bool
	}
	secondResult := make(chan acquireResult, 1)
	go func() {
		acquired, queued := gate.acquire(context.Background())
		secondResult <- acquireResult{acquired: acquired, queued: queued}
	}()

	deadline := time.Now().Add(time.Second)
	for !boundedJobGateHasWaiter(gate) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !boundedJobGateHasWaiter(gate) {
		t.Fatal("waiting slot was not occupied")
	}
	if acquired, queued := gate.acquire(context.Background()); acquired || queued {
		t.Fatalf("third acquire = (%v, %v), want (false, false)", acquired, queued)
	}

	gate.release()
	select {
	case result := <-secondResult:
		if !result.acquired || !result.queued {
			t.Fatalf("queued acquire = (%v, %v), want (true, true)", result.acquired, result.queued)
		}
	case <-time.After(time.Second):
		t.Fatal("queued acquire did not resume after release")
	}
	gate.release()
}

func TestBoundedJobGateCanceledWaiterReleasesQueueSlot(t *testing.T) {
	gate := newBoundedJobGate()
	if acquired, _ := gate.acquire(context.Background()); !acquired {
		t.Fatal("first acquire failed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan acquireResultForTest, 1)
	go func() {
		acquired, queued := gate.acquire(ctx)
		result <- acquireResultForTest{acquired: acquired, queued: queued}
	}()
	deadline := time.Now().Add(time.Second)
	for !boundedJobGateHasWaiter(gate) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !boundedJobGateHasWaiter(gate) {
		cancel()
		gate.release()
		t.Fatal("waiting slot was not occupied before cancellation")
	}
	cancel()

	select {
	case got := <-result:
		if got.acquired || !got.queued {
			t.Fatalf("canceled acquire = (%v, %v), want (false, true)", got.acquired, got.queued)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled acquire did not return")
	}
	if boundedJobGateHasWaiter(gate) {
		t.Fatal("waiting slot remained occupied after cancellation")
	}
	gate.release()
}

func TestBoundedJobGateCancellationAfterHandoffDoesNotLeakRunningSlot(t *testing.T) {
	gate := newBoundedJobGate()
	if acquired, _ := gate.acquire(context.Background()); !acquired {
		t.Fatal("first acquire failed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan acquireResultForTest, 1)
	go func() {
		acquired, queued := gate.acquire(ctx)
		result <- acquireResultForTest{acquired: acquired, queued: queued}
	}()
	waitForBoundedJobGateWaiter(t, gate)

	gate.release()
	cancel()
	select {
	case got := <-result:
		if !got.acquired || !got.queued {
			t.Fatalf("handoff acquire = (%v, %v), want (true, true)", got.acquired, got.queued)
		}
	case <-time.After(time.Second):
		t.Fatal("handoff acquire did not return")
	}

	gate.release()
	running, waiting := boundedJobGateState(gate)
	if running || waiting {
		t.Fatalf("gate state after handoff release = (running=%v, waiting=%v), want both false", running, waiting)
	}
}

func TestBoundedJobGateHandsOffToQueuedRoundBeforeLaterTrigger(t *testing.T) {
	gate := newBoundedJobGate()
	if acquired, queued := gate.acquire(context.Background()); !acquired || queued {
		t.Fatalf("first acquire = (%v, %v), want (true, false)", acquired, queued)
	}

	secondResult := make(chan acquireResultForTest, 1)
	go func() {
		acquired, queued := gate.acquire(context.Background())
		secondResult <- acquireResultForTest{acquired: acquired, queued: queued}
	}()
	waitForBoundedJobGateWaiter(t, gate)

	gate.release()
	thirdResult := make(chan acquireResultForTest, 1)
	go func() {
		acquired, queued := gate.acquire(context.Background())
		thirdResult <- acquireResultForTest{acquired: acquired, queued: queued}
	}()

	select {
	case result := <-secondResult:
		if !result.acquired || !result.queued {
			t.Fatalf("second acquire = (%v, %v), want (true, true)", result.acquired, result.queued)
		}
	case <-time.After(time.Second):
		t.Fatal("queued acquire did not receive handoff")
	}
	select {
	case result := <-thirdResult:
		t.Fatalf("later acquire completed before second release: (%v, %v)", result.acquired, result.queued)
	default:
	}

	waitForBoundedJobGateWaiter(t, gate)
	gate.release()
	select {
	case result := <-thirdResult:
		if !result.acquired || !result.queued {
			t.Fatalf("third acquire = (%v, %v), want (true, true)", result.acquired, result.queued)
		}
	case <-time.After(time.Second):
		t.Fatal("later acquire did not resume after second release")
	}
	gate.release()
}

func boundedJobGateState(gate *boundedJobGate) (running bool, waiting bool) {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	return gate.running, gate.waiter != nil
}

func boundedJobGateHasWaiter(gate *boundedJobGate) bool {
	_, waiting := boundedJobGateState(gate)
	return waiting
}

func waitForBoundedJobGateWaiter(t *testing.T, gate *boundedJobGate) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !boundedJobGateHasWaiter(gate) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !boundedJobGateHasWaiter(gate) {
		t.Fatal("waiting slot was not occupied")
	}
}

type acquireResultForTest struct {
	acquired bool
	queued   bool
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

func TestPingProbeStartPreservesSpacingAfterDelayedRound(t *testing.T) {
	interval := 3 * time.Second
	now := time.Date(2026, 9, 27, 12, 2, 20, 0, time.UTC)
	roundTime := now.Add(-2 * time.Minute)
	start := pingProbeStart(roundTime, 100*time.Millisecond, now)
	if !start.Equal(now) {
		t.Fatalf("delayed probe starts at %v, want %v", start, now)
	}
	if next := start.Add(interval); !next.After(now) {
		t.Fatalf("next probe at %v must remain scheduled after current time", next)
	}

	futureRound := now.Add(2 * time.Second)
	if start := pingProbeStart(futureRound, 100*time.Millisecond, now); !start.Equal(futureRound.Add(100 * time.Millisecond)) {
		t.Fatalf("future probe starts at %v, want %v", start, futureRound.Add(100*time.Millisecond))
	}
}
