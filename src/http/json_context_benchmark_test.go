package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkHistoryJSONContext(b *testing.B) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()
	for _, minutes := range []int{360, maxPingRangeMinutes} {
		timeline, err := newPingHistoryTimelineContext(context.Background(), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), minutes+1)
		if err != nil {
			b.Fatal(err)
		}
		value := map[string][]string{"lastcheck": timeline.lastcheck, "maxdelay": timeline.maxdelay,
			"mindelay": timeline.mindelay, "avgdelay": timeline.avgdelay, "losspk": timeline.losspk}
		encoded, err := json.Marshal(value)
		if err != nil {
			b.Fatal(err)
		}
		for _, mode := range []struct {
			name   string
			ctx    context.Context
			legacy bool
		}{
			{"WithoutRequestContext", canceled, true},
			{"Active", context.Background(), false},
			{"Canceled", canceled, false},
			{"Deadline", expired, false},
		} {
			b.Run(fmt.Sprintf("%dMinutes/%s", minutes, mode.name), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				var response *httptest.ResponseRecorder
				for i := 0; i < b.N; i++ {
					response = httptest.NewRecorder()
					if mode.legacy {
						RenderJson(response, value)
					} else {
						renderJSONContext(mode.ctx, response, value)
					}
				}
				b.StopTimer()
				if mode.legacy || mode.ctx.Err() == nil {
					if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), encoded) {
						b.Fatalf("successful output changed: status=%d, bytes=%d", response.Code, response.Body.Len())
					}
				} else if response.Code != http.StatusInternalServerError || response.Body.String() != mode.ctx.Err().Error()+"\n" {
					b.Fatalf("canceled output status=%d, bytes=%d", response.Code, response.Body.Len())
				}
			})
		}
	}
}
