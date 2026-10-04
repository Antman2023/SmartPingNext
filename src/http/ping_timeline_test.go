package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"smartping/src/g"
	"strings"
	"testing"
	"time"
)

// Cancel after the first successful context check, when initialization is
// already in progress. The test does not depend on a particular batch size.
type cancelAfterTimelineStartContext struct {
	context.Context
	cancel  context.CancelFunc
	checked bool
}

func (ctx *cancelAfterTimelineStartContext) Err() error {
	if ctx.checked {
		ctx.cancel()
	}
	ctx.checked = true
	return ctx.Context.Err()
}

func pingTimelineCancellationCases() []struct {
	name string
	want error
	new  func() (context.Context, context.CancelFunc)
} {
	return []struct {
		name string
		want error
		new  func() (context.Context, context.CancelFunc)
	}{
		{"already canceled", context.Canceled, func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, cancel
		}},
		{"deadline exceeded", context.DeadlineExceeded, func() (context.Context, context.CancelFunc) {
			return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		}},
		{"canceled during initialization", context.Canceled, func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			return &cancelAfterTimelineStartContext{Context: ctx, cancel: cancel}, cancel
		}},
	}
}

func TestPingTimelineCancellationDiscardsAllPartialState(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, size := range []int{1, 361, maxPingRangeMinutes + 1} {
		for _, tc := range pingTimelineCancellationCases() {
			t.Run(fmt.Sprintf("%dSamples/%s", size, tc.name), func(t *testing.T) {
				ctx, cancel := tc.new()
				defer cancel()
				timeline, err := newPingHistoryTimelineContext(ctx, start, size)
				if !errors.Is(err, tc.want) {
					t.Fatalf("error=%v, want %v", err, tc.want)
				}
				if !reflect.DeepEqual(timeline, pingHistoryTimeline{}) {
					t.Fatal("canceled initialization published arrays, population markers or query bounds")
				}
			})
		}
	}
}

func TestPingEndpointCanceledInitializationNeverQueriesDatabase(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range pingTimelineCancellationCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, handler := pingIndexTestEndpoint(t, time.UTC)
			// A query through this sentinel panics, proving canceled construction
			// returns before any database access rather than relying on SQLite.
			g.Db = nil
			for _, minutes := range []int{0, 360, maxPingRangeMinutes} {
				t.Run(fmt.Sprintf("%dMinutes", minutes), func(t *testing.T) {
					ctx, cancel := tc.new()
					defer cancel()
					query := url.Values{"ip": {"192.0.2.1"}, "starttime": {start.Format("2006-01-02 15:04")},
						"endtime": {start.Add(time.Duration(minutes) * time.Minute).Format("2006-01-02 15:04")}}
					request := httptest.NewRequest(http.MethodGet, "/api/ping.json?"+query.Encode(), nil).WithContext(ctx)
					response := httptest.NewRecorder()
					defer func() {
						if caught := recover(); caught != nil {
							t.Fatalf("canceled initialization touched the database: %v", caught)
						}
					}()
					handler.ServeHTTP(response, request)
					if response.Code != http.StatusInternalServerError || strings.TrimSpace(response.Body.String()) != "Query ping data failed" {
						t.Fatalf("status=%d, body=%q; want 500 without partial JSON", response.Code, response.Body.String())
					}
				})
			}
			g.Db = db
			if _, err := db.Exec(`INSERT INTO pinglog VALUES ('2026-01-01 00:00', '192.0.2.1', '0', '0', '0', '0')`); err != nil {
				t.Fatal(err)
			}
			result := readPingIndexTestHistory(t, handler, "2026-01-01 00:00", "2026-01-01 00:01")
			if !reflect.DeepEqual(result["avgdelay"], []string{"0", "-"}) {
				t.Fatalf("uncanceled query did not recover: %v", result)
			}
		})
	}
}

func TestCanceledPingRequestPreservesParameterValidationOrder(t *testing.T) {
	_, handler := pingIndexTestEndpoint(t, time.UTC)
	g.Db = nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		method, query string
		want          int
	}{
		{http.MethodPost, "ip=192.0.2.1", http.StatusMethodNotAllowed},
		{http.MethodGet, "", http.StatusNotAcceptable},
		{http.MethodGet, "ip=192.0.2.1&ip=192.0.2.2", http.StatusNotAcceptable},
		{http.MethodGet, "ip=192.0.2.1&starttime=invalid&endtime=invalid", http.StatusNotAcceptable},
		{http.MethodGet, "ip=192.0.2.1&starttime=2026-01-01+00%3A00", http.StatusNotAcceptable},
	} {
		request := httptest.NewRequest(tc.method, "/api/ping.json?"+tc.query, nil).WithContext(ctx)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != tc.want {
			t.Errorf("%s %q: status=%d, want %d", tc.method, tc.query, response.Code, tc.want)
		}
	}
}
