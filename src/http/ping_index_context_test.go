package http

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"smartping/src/g"
	"sync/atomic"
	"testing"
	"time"

	"modernc.org/sqlite"
)

type cancelPingIndexOnCheckContext struct {
	context.Context
	cancel   context.CancelFunc
	checks   int
	cancelAt int
}

func (ctx *cancelPingIndexOnCheckContext) Err() error {
	ctx.checks++
	if ctx.checks == ctx.cancelAt {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func pingIndexContextLabels(size int, rollback bool) []string {
	labels := make([]string, size)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range labels {
		labels[i] = start.Add(time.Duration(i) * time.Minute).Format("2006-01-02 15:04")
	}
	if rollback && size > 1 {
		// A final rollback detects non-monotonicity before the map is built.
		labels[size-1] = labels[0]
	}
	return labels
}

func TestPingIndexRejectsFinishedContextsWithoutPartialState(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		for _, size := range []int{0, 1, 361, maxPingRangeMinutes + 1} {
			for _, expired := range []bool{false, true} {
				t.Run(fmt.Sprintf("Size=%d/Rollback=%t/Expired=%t", size, rollback, expired), func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					if expired {
						ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
					}
					defer cancel()
					index, err := newPingTimelineIndexContext(ctx, pingIndexContextLabels(size, rollback))
					if !errors.Is(err, ctx.Err()) || !reflect.DeepEqual(index, pingTimelineIndex{}) {
						t.Fatalf("finished context returned index state: error=%v, want %v", err, ctx.Err())
					}
				})
			}
		}
	}
}

func TestPingIndexCancellationDiscardsOrderingAndMapWork(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		t.Run(fmt.Sprintf("Rollback=%t", rollback), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			check := 2
			if rollback {
				check = 4 // After some entries have been copied to the fallback map.
			}
			index, err := newPingTimelineIndexContext(&cancelPingIndexOnCheckContext{
				Context: ctx, cancel: cancel, cancelAt: check,
			}, pingIndexContextLabels(4096, rollback))
			if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(index, pingTimelineIndex{}) {
				t.Fatalf("canceled construction returned partial index: %v", err)
			}
		})
	}
}

func TestPingIndexContextPreservesLookupsAndResultIndependence(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		t.Run(fmt.Sprintf("Rollback=%t", rollback), func(t *testing.T) {
			labels := pingIndexContextLabels(4096, rollback)
			original := append([]string(nil), labels...)
			want := make(map[string]int)
			for i, stamp := range labels {
				want[stamp] = i
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			index, err := newPingTimelineIndexContext(ctx, labels)
			if err != nil {
				t.Fatal(err)
			}
			queries := append([]string{"", "2026-01-01 0:00", "2027-01-01 00:00"}, labels...)
			for round := 0; round < 2; round++ {
				for i := len(queries) - 1; i >= 0; i-- {
					stamp := queries[i]
					expected, present := want[stamp]
					got, found := index.lookup(stamp)
					if got != expected || found != present {
						t.Fatalf("lookup(%q)=(%d,%t), want (%d,%t)", stamp, got, found, expected, present)
					}
				}
			}
			second, err := newPingTimelineIndexContext(ctx, labels)
			if err != nil || !reflect.DeepEqual(labels, original) {
				t.Fatal("construction or lookup changed input labels")
			}
			if rollback {
				second.lookup(labels[0])
				second.positions[labels[0]] = 0
				if got, _ := index.lookup(labels[0]); got != want[labels[0]] {
					t.Fatal("independent indexes shared their fallback map")
				}
			} else if index.positions != nil || second.positions != nil {
				t.Fatal("ordered labels allocated a fallback map")
			}
		})
	}
}

type cancelAfterPingRowContext struct {
	context.Context
	cancel context.CancelFunc
	armed  atomic.Bool
}

func (ctx *cancelAfterPingRowContext) Err() error {
	if ctx.armed.Load() {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestPingEndpointStopsIndexWorkAfterFirstDatabaseRow(t *testing.T) {
	for _, zone := range []string{"UTC", "America/New_York"} {
		t.Run(zone, func(t *testing.T) {
			base, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx := &cancelAfterPingRowContext{Context: base, cancel: cancel}
			var enabled atomic.Bool
			var rowsRead atomic.Int32
			wrapped := &responseCancellationDriver{base: &sqlite.Driver{}, afterRowsNext: func() {
				if enabled.Load() {
					rowsRead.Add(1)
					ctx.armed.Store(true)
				}
			}}
			name := fmt.Sprintf("ping-index-cancellation-%d", responseCancellationDriverID.Add(1))
			sql.Register(name, wrapped)
			db, handler := historyReadFixtureWithDriver(t, name)
			location, err := time.LoadLocation(zone)
			if err != nil {
				t.Fatal(err)
			}
			// The New York request includes the rollback and needs map lookup.
			g.LocalTimezone = location
			if _, err := db.Exec(`INSERT INTO pinglog VALUES
				('2011-11-06 00:00','192.0.2.1','0','0','0','0'),
				('2011-11-06 02:00','192.0.2.1','20','10','15','100')`); err != nil {
				t.Fatal(err)
			}
			query := url.Values{"ip": {"192.0.2.1"}, "starttime": {"2011-11-06 00:00"}, "endtime": {"2011-11-06 06:00"}}
			request := httptest.NewRequest(http.MethodGet, "/api/ping.json?"+query.Encode(), nil).WithContext(ctx)
			response := httptest.NewRecorder()
			enabled.Store(true)
			handler.ServeHTTP(response, request)
			enabled.Store(false)
			if response.Code != http.StatusInternalServerError || response.Body.String() != "Read ping data failed\n" || rowsRead.Load() != 1 {
				t.Fatalf("status=%d, body=%q, rows=%d; want canceled index before the second row", response.Code, response.Body.String(), rowsRead.Load())
			}
			if db.Stats().InUse != 0 {
				t.Fatal("canceled index retained its database connection")
			}
			var one int
			if err := db.QueryRow("SELECT 1").Scan(&one); err != nil || one != 1 {
				t.Fatalf("database did not recover: %v", err)
			}
			result := readPingIndexTestHistory(t, handler, "2011-11-06 00:00", "2011-11-06 06:00")
			position := 120
			if zone != "UTC" {
				position = 180
			}
			if result["avgdelay"][0] != "0" || result["avgdelay"][position] != "15" || result["losspk"][position] != "100" {
				t.Fatal("uncanceled request lost sample positions or measured values")
			}
		})
	}
}
