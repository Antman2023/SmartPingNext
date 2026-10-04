package http

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"smartping/src/g"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func BenchmarkCanceledPingHistory(b *testing.B) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE pinglog (logtime TEXT, target TEXT, maxdelay TEXT, mindelay TEXT, avgdelay TEXT, losspk TEXT)`); err != nil {
		db.Close()
		b.Fatal(err)
	}
	previousConfig, previousDB, previousZone := g.ConfigSnapshot(), g.Db, g.LocalTimezone
	previousOutput := logrus.StandardLogger().Out
	g.Db, g.LocalTimezone = db, time.UTC
	g.SetConfig(g.Config{Addr: "127.0.0.1", Network: map[string]g.NetworkMember{}})
	logrus.SetOutput(io.Discard)
	b.Cleanup(func() {
		logrus.SetOutput(previousOutput)
		g.Db, g.LocalTimezone = previousDB, previousZone
		g.SetConfig(previousConfig)
		_ = db.Close()
	})
	handler := newAppHandler()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()
	for _, minutes := range []int{360, maxPingRangeMinutes} {
		query := url.Values{"ip": {"192.0.2.1"}, "starttime": {start.Format("2006-01-02 15:04")},
			"endtime": {start.Add(time.Duration(minutes) * time.Minute).Format("2006-01-02 15:04")}}
		for _, tc := range []struct {
			name string
			ctx  context.Context
		}{{"Canceled", canceled}, {"Deadline", expired}} {
			request := httptest.NewRequest(http.MethodGet, "/api/ping.json?"+query.Encode(), nil).WithContext(tc.ctx)
			b.Run(fmt.Sprintf("%dMinutes/%s", minutes, tc.name), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					response := httptest.NewRecorder()
					handler.ServeHTTP(response, request)
					if response.Code != http.StatusInternalServerError {
						b.Fatalf("status=%d, body=%s; want 500", response.Code, response.Body.String())
					}
				}
			})
		}
	}
}

func BenchmarkPingHistoryTimeline(b *testing.B) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, size := range []int{0, 1, 361, 1441, maxPingRangeMinutes + 1} {
		b.Run(fmt.Sprintf("%dSamples", size), func(b *testing.B) {
			want := expectedPingHistoryTimeline(start, size)
			check := func(timeline pingHistoryTimeline, err error) {
				b.Helper()
				if err != nil || !reflect.DeepEqual(timeline, want) {
					b.Fatalf("timeline changed labels, gaps, markers or query bounds: %v", err)
				}
			}
			check(newPingHistoryTimelineContext(ctx, start, size))
			b.ReportAllocs()
			b.ResetTimer()
			var last pingHistoryTimeline
			for i := 0; i < b.N; i++ {
				var err error
				last, err = newPingHistoryTimelineContext(ctx, start, size)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			check(last, nil)
		})
	}
}

func BenchmarkPingHistoryTimelineCancellation(b *testing.B) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, size := range []int{1, 361, maxPingRangeMinutes + 1} {
		for _, tc := range pingTimelineCancellationCases() {
			b.Run(fmt.Sprintf("%dSamples/%s", size, tc.name), func(b *testing.B) {
				ctx, cancel := tc.new()
				initial, err := newPingHistoryTimelineContext(ctx, start, size)
				cancel()
				if !errors.Is(err, tc.want) || !reflect.DeepEqual(initial, pingHistoryTimeline{}) {
					b.Fatalf("cancellation published partial state: %v", err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				var last pingHistoryTimeline
				for i := 0; i < b.N; i++ {
					// Include context creation and cancellation equally for both
					// implementations; each in-progress case needs a fresh context.
					ctx, cancel := tc.new()
					last, err = newPingHistoryTimelineContext(ctx, start, size)
					cancel()
					if !errors.Is(err, tc.want) {
						b.Fatal(err)
					}
				}
				b.StopTimer()
				if !reflect.DeepEqual(last, pingHistoryTimeline{}) {
					b.Fatal("cancellation published partial state")
				}
			})
		}
	}
}
