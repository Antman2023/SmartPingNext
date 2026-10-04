package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"smartping/src/g"
	"testing"
	"time"
)

func BenchmarkMappingDecodeContext(b *testing.B) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()
	active, finish := context.WithCancel(context.Background())
	defer finish()
	for _, count := range []int{0, 3, 102, 8192} {
		want := emptyMappingData()
		carriers := []string{"ctcc", "cucc", "cmcc"}
		values := []float64{0, 12.5, 2000}
		for index := 0; index < count; index++ {
			carrier := carriers[index%len(carriers)]
			want[carrier] = append(want[carrier], g.MapVal{Name: fmt.Sprintf("地区 %d <&>", index), Value: values[index%len(values)]})
		}
		encoded, err := json.Marshal(want)
		if err != nil {
			b.Fatal(err)
		}
		raw := string(encoded)
		for _, mode := range []struct {
			name   string
			ctx    context.Context
			legacy bool
		}{
			{"WithoutRequestContext", canceled, true},
			{"Active", active, false},
			{"Canceled", canceled, false},
			{"Deadline", expired, false},
		} {
			b.Run(fmt.Sprintf("%dSamples/%s", count, mode.name), func(b *testing.B) {
				decode := func() (map[string][]g.MapVal, error) {
					if mode.legacy {
						return decodeMappingData(raw)
					}
					return decodeMappingDataContext(mode.ctx, raw)
				}
				verify := func(result map[string][]g.MapVal, err error) {
					if mode.legacy || mode.ctx.Err() == nil {
						if err != nil || !reflect.DeepEqual(result, want) {
							b.Fatalf("decoded values changed: err=%v", err)
						}
					} else if result != nil || !errors.Is(err, mode.ctx.Err()) {
						b.Fatalf("canceled decode returned partial data: result=%v, err=%v", result != nil, err)
					}
				}
				verify(decode())
				b.ReportAllocs()
				b.ResetTimer()
				var result map[string][]g.MapVal
				var err error
				for index := 0; index < b.N; index++ {
					result, err = decode()
				}
				b.StopTimer()
				verify(result, err)
				b.ReportMetric(float64(len(raw)), "input-B")
			})
		}
	}
}
