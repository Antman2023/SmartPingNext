package http

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	for _, minutes := range []int{360, maxPingRangeMinutes} {
		b.Run(fmt.Sprintf("%dMinutes", minutes), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				timeline, err := newPingHistoryTimelineContext(ctx, start, minutes+1)
				if err != nil || len(timeline.lastcheck) != minutes+1 || timeline.lastcheck[0] != "2026-01-01 00:00" {
					b.Fatalf("timeline length=%d, err=%v", len(timeline.lastcheck), err)
				}
			}
		})
	}
}
