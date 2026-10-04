package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"smartping/src/g"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"modernc.org/sqlite"
)

func TestMappingDecodeHonorsFinishedContexts(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()
	for _, ctx := range []context.Context{canceled, expired} {
		for _, raw := range []string{`null`, `{}`, `{"ctcc":null}`, `{"ctcc":[`,
			`{"ctcc":[{"name":"广东","value":0}]}`} {
			result, err := decodeMappingDataContext(ctx, raw)
			if result != nil || !errors.Is(err, ctx.Err()) {
				t.Errorf("finished context %v: decoded=%v, error=%v", ctx.Err(), result != nil, err)
			}
		}
	}
}

type mappingCancelOnCheckContext struct {
	context.Context
	cancel context.CancelFunc
	checks atomic.Int32
}

func (ctx *mappingCancelOnCheckContext) Err() error {
	if ctx.checks.Add(1) == 4 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestMappingDecodeDiscardsPartiallyNormalizedResult(t *testing.T) {
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &mappingCancelOnCheckContext{Context: base, cancel: cancel}
	raw := `{"ctcc":[` + strings.TrimSuffix(strings.Repeat(`{"name":"广东","value":0},`, 4096), ",") + `]}`
	result, err := decodeMappingDataContext(ctx, raw)
	if result != nil || !errors.Is(err, context.Canceled) || base.Err() == nil {
		t.Fatalf("cancellation while normalizing: result=%v, error=%v, context=%v", result != nil, err, base.Err())
	}
}

func TestMappingDecodePreservesEveryCarrierSample(t *testing.T) {
	want := emptyMappingData()
	for _, carrier := range []string{"ctcc", "cucc", "cmcc"} {
		for index := 0; index < 65; index++ {
			want[carrier] = append(want[carrier], g.MapVal{Name: fmt.Sprintf("%s 地区 %d <&>", carrier, index),
				Value: []float64{0, 12.5, 2000}[index%3]})
		}
	}
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for attempt := 0; attempt < 2; attempt++ {
		got, err := decodeMappingDataContext(ctx, string(encoded))
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("attempt %d lost or changed carrier samples: %v", attempt, err)
		}
		// Neither another carrier nor a later request may reuse this result.
		got["ctcc"][0] = g.MapVal{Name: "changed", Value: 999}
		if !reflect.DeepEqual(got["cucc"], want["cucc"]) || !reflect.DeepEqual(got["cmcc"], want["cmcc"]) {
			t.Fatal("carrier result slices overlap")
		}
	}
}

func TestMappingEndpointSkipsDecodeAfterReadCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var once sync.Once
	driver := &responseCancellationDriver{base: &sqlite.Driver{}, afterRowsClose: func() { once.Do(cancel) }}
	driverName := fmt.Sprintf("mapping-decode-cancellation-%d", responseCancellationDriverID.Add(1))
	sql.Register(driverName, driver)
	db, handler := historyReadFixtureWithDriver(t, driverName)
	if _, err := db.Exec(`CREATE TABLE mappinglog (logtime TEXT, mapjson TEXT);
		INSERT INTO mappinglog VALUES ('2026-01-01 00:00','{"ctcc":[')`); err != nil {
		t.Fatal(err)
	}
	const path = "/api/mapping.json?d=2026-01-01+00%3A00"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx))
	if ctx.Err() == nil || response.Code != http.StatusInternalServerError || response.Body.String() != "context canceled\n" {
		t.Fatalf("canceled read status=%d, body=%q, context=%v", response.Code, response.Body.String(), ctx.Err())
	}
	if db.Stats().InUse != 0 {
		t.Fatal("canceled decode retained a database connection")
	}
	var one int
	if err := db.QueryRow("SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatalf("pool did not recover: value=%d, error=%v", one, err)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != http.StatusInternalServerError || response.Body.String() != "Invalid mapping data\n" {
		t.Fatalf("active request accepted invalid data: status=%d, body=%q", response.Code, response.Body.String())
	}
	if _, err := db.Exec(`UPDATE mappinglog SET mapjson=?`, `{"ctcc":[{"name":"广东 <&>","value":0}],"cmcc":[{"name":"北京","value":2000}]}`); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	var data g.ChinaMp
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil || response.Code != http.StatusOK {
		t.Fatalf("subsequent request failed: status=%d, error=%v", response.Code, err)
	}
	if len(data.Avgdelay["ctcc"]) != 1 || data.Avgdelay["ctcc"][0] != (g.MapVal{Name: "广东 <&>", Value: 0}) ||
		data.Avgdelay["cucc"] == nil || len(data.Avgdelay["cucc"]) != 0 ||
		len(data.Avgdelay["cmcc"]) != 1 || data.Avgdelay["cmcc"][0].Value != 2000 {
		t.Fatalf("normalized measurements changed: %+v", data.Avgdelay)
	}
}
