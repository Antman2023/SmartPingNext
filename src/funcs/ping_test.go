package funcs

import (
	"context"
	"smartping/src/g"
	"testing"
	"time"
)

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
