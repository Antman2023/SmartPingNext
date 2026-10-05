package funcs

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"reflect"
	"smartping/src/g"
	"sync/atomic"
	"testing"
	"time"
)

func TestMappingCancellationExitsWhileResultLockIsHeld(t *testing.T) {
	for _, round := range []bool{false, true} {
		for _, deadline := range []bool{false, true} {
			t.Run(map[bool]string{false: "storage", true: "round"}[round]+map[bool]string{false: "/cancel", true: "/deadline"}[deadline], func(t *testing.T) {
				withFuncTestDB(t, []string{`CREATE TABLE mappinglog (logtime TEXT UNIQUE, mapjson TEXT);`}, func(db *sql.DB) {
					oldConfig := g.ConfigSnapshot()
					g.SetConfig(g.Config{Chinamap: map[string]map[string][]string{}})
					defer g.SetConfig(oldConfig)
					MapLock.Lock()
					oldStatus := MapStatus
					want := map[string][]g.MapVal{"ctcc": {{Name: "existing", Value: 12.5}}}
					MapStatus = want
					held := true
					ctx, cancel := context.WithCancel(context.Background())
					wantErr := context.Canceled
					if deadline {
						cancel()
						ctx, cancel = context.WithTimeout(context.Background(), 150*time.Millisecond)
						wantErr = context.DeadlineExceeded
					}
					observed := &observedStorageContext{Context: ctx, checked: make(chan error, 1), waiting: make(chan struct{})}
					done := make(chan error, 1)
					finished := false
					defer func() {
						cancel()
						if held {
							MapLock.Unlock()
						}
						if !finished {
							select {
							case <-done:
							case <-time.After(time.Second):
								t.Error("mapping worker did not exit after cleanup")
							}
						}
						MapLock.Lock()
						MapStatus = oldStatus
						MapLock.Unlock()
					}()
					go func() {
						if round {
							MappingContext(observed)
							done <- observed.Err()
						} else {
							done <- MapPingStorageContext(observed)
						}
					}()
					select {
					case err := <-observed.checked:
						if err != nil {
							t.Fatalf("mapping started after cancellation: %v", err)
						}
					case <-time.After(time.Second):
						t.Fatal("mapping did not start")
					}
					if !deadline {
						cancel()
					}
					select {
					case err := <-done:
						finished = true
						if !errors.Is(err, wantErr) {
							t.Errorf("mapping error=%v, want %v", err, wantErr)
						}
					case <-time.After(500 * time.Millisecond):
						t.Error("canceled mapping waited for the result lock to be released")
					}
					MapLock.Unlock()
					held = false
					if !finished {
						select {
						case <-done:
							finished = true
						case <-time.After(time.Second):
							t.Fatal("mapping did not exit after releasing result lock")
						}
					}
					MapLock.Lock()
					got := MapStatus
					MapLock.Unlock()
					if !reflect.DeepEqual(got, want) {
						t.Errorf("canceled mapping changed results: %v", got)
					}
					if atomic.LoadInt32(&mappingRunning) != 0 {
						t.Fatal("canceled mapping retained its running guard")
					}
					var count int
					if err := db.QueryRow("SELECT count(*) FROM mappinglog").Scan(&count); err != nil || count != 0 {
						t.Fatalf("canceled mapping wrote rows: count=%d error=%v", count, err)
					}
					if err := MapPingStorageContext(context.Background()); err != nil {
						t.Fatalf("mapping storage could not recover: %v", err)
					}
					if err := db.QueryRow("SELECT count(*) FROM mappinglog").Scan(&count); err != nil || count != 1 {
						t.Fatalf("recovered mapping did not store a row: count=%d error=%v", count, err)
					}
				})
			})
		}
	}
}

type mappingCancelOnCheckContext struct {
	context.Context
	cancel           context.CancelFunc
	checks, cancelAt int
}

func (ctx *mappingCancelOnCheckContext) Err() error {
	ctx.checks++
	if ctx.checks == ctx.cancelAt {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func setMappingSnapshotFixture(t testing.TB, status map[string][]g.MapVal) {
	t.Helper()
	MapLock.Lock()
	old := MapStatus
	MapStatus = status
	MapLock.Unlock()
	t.Cleanup(func() {
		MapLock.Lock()
		MapStatus = old
		MapLock.Unlock()
	})
}

func TestMappingSnapshotContextDiscardsCanceledCopiesAndKeepsSource(t *testing.T) {
	source := make([]g.MapVal, 1024)
	for i := range source {
		source[i] = g.MapVal{Name: string(rune(2048 - i)), Value: float64(i)}
	}
	setMappingSnapshotFixture(t, map[string][]g.MapVal{"ctcc": source})
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &mappingCancelOnCheckContext{Context: base, cancel: cancel, cancelAt: 6}
	result, err := mappingStatusSnapshotContext(ctx)
	if result != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled copy returned partial results: result=%v error=%v", result != nil, err)
	}
	if source[0].Value != 0 || source[len(source)-1].Value != 1023 {
		t.Fatal("canceled copy changed source data")
	}
	result, err = mappingStatusSnapshotContext(context.Background())
	if err != nil || len(result["ctcc"]) != len(source) || result["ctcc"][0].Value != 1023 {
		t.Fatalf("snapshot lock did not recover: %v", err)
	}
}

func TestMappingSnapshotContextPreservesLegacyEmptyShapesAndIndependentValues(t *testing.T) {
	for _, status := range []map[string][]g.MapVal{
		nil, {}, {"ctcc": nil, "cucc": {}},
		{"ctcc": {{Name: "Z", Value: 0}, {Name: "A", Value: 12.5}}, "extra": {{Name: "B", Value: 30}}},
	} {
		setMappingSnapshotFixture(t, status)
		want := mappingStatusSnapshot()
		active, stop := context.WithCancel(context.Background())
		got, err := mappingStatusSnapshotContext(active)
		stop()
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("context snapshot=%v error=%v, want=%v", got, err, want)
		}
		if len(got["ctcc"]) > 0 {
			got["ctcc"][0].Name = "changed"
			if status["ctcc"][0].Name != "Z" || status["ctcc"][1].Name != "A" {
				t.Fatal("snapshot mutation changed source order or names")
			}
		}
		for _, expired := range []bool{false, true} {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if expired {
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			}
			got, err := mappingStatusSnapshotContext(ctx)
			cancel()
			if got != nil || !errors.Is(err, ctx.Err()) {
				t.Fatalf("finished context returned data: result=%v error=%v", got, err)
			}
		}
	}
}

func TestMappingSortChecksCancellationBeforeAndAfterCarrierSort(t *testing.T) {
	for _, checkpoint := range []int{1, 2} {
		base, cancel := context.WithCancel(context.Background())
		ctx := &mappingCancelOnCheckContext{Context: base, cancel: cancel, cancelAt: checkpoint}
		result := map[string][]g.MapVal{"ctcc": {{Name: "Z"}, {Name: "A"}}}
		err := sortMappingStatusSnapshotContext(ctx, result)
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("sort checkpoint=%d error=%v", checkpoint, err)
		}
	}
}

func TestMappingResultPublicationCanCancelInLockQueue(t *testing.T) {
	setMappingSnapshotFixture(t, map[string][]g.MapVal{"ctcc": {{Name: "existing", Value: 12.5}}})
	MapLock.Lock()
	base, cancel := context.WithCancel(context.Background())
	ctx := &observedStorageContext{Context: base, checked: make(chan error, 1), waiting: make(chan struct{})}
	done := make(chan error, 1)
	go func() { done <- storeMappingResultContext(ctx, "ctcc", "canceled", 20) }()
	select {
	case <-ctx.waiting:
	case <-time.After(time.Second):
		cancel()
		MapLock.Unlock()
		<-done
		t.Fatal("publisher did not enter result lock queue")
	}
	cancel()
	select {
	case err := <-done:
		MapLock.Unlock()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("publication cancellation=%v", err)
		}
	case <-time.After(time.Second):
		MapLock.Unlock()
		<-done
		t.Fatal("canceled publication waited for result lock release")
	}
	if got := mappingStatusSnapshot()["ctcc"]; len(got) != 1 || got[0].Name != "existing" {
		t.Fatalf("canceled publication changed status: %v", got)
	}
	if err := storeMappingResultContext(context.Background(), "ctcc", "next", 30); err != nil {
		t.Fatal(err)
	}
	if got := mappingStatusSnapshot()["ctcc"]; len(got) != 2 {
		t.Fatalf("result lock did not recover: %v", got)
	}
}

func TestMapPingStorageCancellationBeforeEncodingSkipsInvalidJSON(t *testing.T) {
	withFuncTestDB(t, []string{`CREATE TABLE mappinglog (logtime TEXT UNIQUE, mapjson TEXT);`}, func(db *sql.DB) {
		setMappingSnapshotFixture(t, map[string][]g.MapVal{"ctcc": {{Name: "invalid", Value: math.NaN()}}})
		base, cancel := context.WithCancel(context.Background())
		defer cancel()
		// With one carrier and one value, this is the check after the sorted
		// snapshot returns and before JSON marshaling begins.
		ctx := &mappingCancelOnCheckContext{Context: base, cancel: cancel, cancelAt: 9}
		if err := MapPingStorageContext(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled encoding returned %v, want cancellation before NaN encoding", err)
		}
		var count int
		if err := db.QueryRow("SELECT count(*) FROM mappinglog").Scan(&count); err != nil || count != 0 {
			t.Fatalf("canceled encoding wrote rows: count=%d error=%v", count, err)
		}
		if err := MapPingStorageContext(context.Background()); err == nil || errors.Is(err, context.Canceled) {
			t.Fatalf("active encoding should still reject invalid JSON: %v", err)
		}
	})
}
