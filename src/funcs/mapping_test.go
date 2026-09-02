package funcs

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"smartping/src/g"
	"sync/atomic"
	"testing"
)

func TestMappingContextDoesNotStartCanceledRound(t *testing.T) {
	atomic.StoreInt32(&mappingRunning, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	MappingContext(ctx)
	if got := atomic.LoadInt32(&mappingRunning); got != 0 {
		t.Fatalf("mappingRunning = %d after canceled call, want 0", got)
	}
}

func TestAggregateMappingDelayIgnoresQuarterOfFailedProbes(t *testing.T) {
	stats := []g.PingSt{
		{AvgDelay: 2000, LossPk: 100},
		{AvgDelay: 20, RevcPk: 1},
		{AvgDelay: 30, RevcPk: 1},
	}

	if got := aggregateMappingDelay(stats); got != 25 {
		t.Fatalf("aggregateMappingDelay() = %.2f, want 25.00", got)
	}
}

func TestAggregateMappingDelayPenalizesFailuresBeyondAllowance(t *testing.T) {
	stats := []g.PingSt{
		{AvgDelay: 2000, LossPk: 100},
		{AvgDelay: 20, RevcPk: 1},
		{AvgDelay: 2000, LossPk: 100},
		{AvgDelay: 40, RevcPk: 1},
	}

	if got := aggregateMappingDelay(stats); got != 686.67 {
		t.Fatalf("aggregateMappingDelay() = %.2f, want 686.67", got)
	}
}

func TestAggregateMappingDelayUsesFailureSentinelWhenAllProbesFail(t *testing.T) {
	stats := []g.PingSt{
		{AvgDelay: 2000, LossPk: 100},
		{AvgDelay: 2000, LossPk: 100},
		{AvgDelay: 2000, LossPk: 100},
	}

	if got := aggregateMappingDelay(stats); got != 2000 {
		t.Fatalf("aggregateMappingDelay() = %.2f, want 2000.00", got)
	}
}

func TestStoreMappingResultUsesCarrierKeyAndProvinceName(t *testing.T) {
	MapLock.Lock()
	oldStatus := MapStatus
	MapStatus = make(map[string][]g.MapVal)
	MapLock.Unlock()
	defer func() {
		MapLock.Lock()
		MapStatus = oldStatus
		MapLock.Unlock()
	}()

	storeMappingResult("ctcc", "北京", 12.5)
	MapLock.Lock()
	values := append([]g.MapVal(nil), MapStatus["ctcc"]...)
	_, hasProvinceKey := MapStatus["北京"]
	MapLock.Unlock()

	if hasProvinceKey {
		t.Fatal("mapping result used province as map key")
	}
	if len(values) != 1 || values[0].Name != "北京" || values[0].Value != 12.5 {
		t.Fatalf("stored mapping result = %#v", values)
	}
}

func TestMappingStatusSnapshotIsSortedAndIndependent(t *testing.T) {
	MapLock.Lock()
	oldStatus := MapStatus
	MapStatus = map[string][]g.MapVal{
		"ctcc": {
			{Name: "浙江", Value: 20},
			{Name: "北京", Value: 10},
		},
	}
	MapLock.Unlock()
	defer func() {
		MapLock.Lock()
		MapStatus = oldStatus
		MapLock.Unlock()
	}()

	snapshot := mappingStatusSnapshot()
	if got := snapshot["ctcc"]; len(got) != 2 || got[0].Name != "北京" || got[1].Name != "浙江" {
		t.Fatalf("snapshot order = %#v, want province name order", got)
	}

	snapshot["ctcc"][0].Name = "changed"
	snapshot["cucc"] = []g.MapVal{{Name: "新增", Value: 30}}
	MapLock.Lock()
	original := append([]g.MapVal(nil), MapStatus["ctcc"]...)
	_, addedCarrier := MapStatus["cucc"]
	MapLock.Unlock()
	if original[0].Name != "浙江" || addedCarrier {
		t.Fatalf("snapshot mutation changed global status: %#v", MapStatus)
	}
}

func TestMapPingStorageRejectsInvalidJSONWithoutWriting(t *testing.T) {
	schema := []string{
		`CREATE TABLE mappinglog (logtime TEXT UNIQUE, mapjson TEXT);`,
	}

	withFuncTestDB(t, schema, func(db *sql.DB) {
		MapLock.Lock()
		oldStatus := MapStatus
		MapStatus = map[string][]g.MapVal{
			"ctcc": {{Name: "invalid", Value: math.NaN()}},
		}
		MapLock.Unlock()
		t.Cleanup(func() {
			MapLock.Lock()
			MapStatus = oldStatus
			MapLock.Unlock()
		})

		err := MapPingStorageContext(context.Background())
		if err == nil {
			t.Fatal("MapPingStorageContext should reject a non-finite mapping value")
		}

		var count int
		if queryErr := db.QueryRow(`SELECT count(1) FROM mappinglog`).Scan(&count); queryErr != nil {
			t.Fatalf("count mapping rows: %v", queryErr)
		}
		if count != 0 {
			t.Fatalf("mapping rows = %d, want no write after JSON failure", count)
		}
	})
}

func TestMapPingStorageReturnsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := MapPingStorageContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("MapPingStorageContext error = %v, want context canceled", err)
	}
}
