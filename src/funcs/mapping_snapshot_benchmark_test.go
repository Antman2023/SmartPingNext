package funcs

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"smartping/src/g"
	"sort"
	"sync"
	"testing"
)

func BenchmarkMappingStatusSnapshotContext(b *testing.B) {
	for _, size := range []int{0, 64, 8192} {
		b.Run(fmt.Sprintf("%dSamples", size), func(b *testing.B) {
			status := map[string][]g.MapVal{"ctcc": nil, "cucc": nil, "cmcc": nil}
			carriers := []string{"ctcc", "cucc", "cmcc"}
			for i := range size {
				carrier := carriers[i%len(carriers)]
				status[carrier] = append(status[carrier], g.MapVal{Name: fmt.Sprintf("province-%05d", size-i), Value: float64(i)})
			}
			setMappingSnapshotFixture(b, status)
			// The legacy comparison retains its original sync.Mutex and immutable
			// source. Both paths run without concurrent producers in this benchmark.
			var legacyLock sync.Mutex
			legacy := func() map[string][]g.MapVal {
				legacyLock.Lock()
				snapshot := make(map[string][]g.MapVal, len(status))
				for carrier, values := range status {
					snapshot[carrier] = append([]g.MapVal(nil), values...)
				}
				legacyLock.Unlock()
				for carrier := range snapshot {
					sort.Slice(snapshot[carrier], func(i, j int) bool {
						return snapshot[carrier][i].Name < snapshot[carrier][j].Name
					})
				}
				return snapshot
			}
			want := legacy()
			active, stop := context.WithCancel(context.Background())
			defer stop()
			canceled, cancel := context.WithCancel(context.Background())
			cancel()
			for _, mode := range []struct {
				name   string
				ctx    context.Context
				legacy bool
			}{
				{"Legacy", context.Background(), true},
				{"Background", context.Background(), false},
				{"Active", active, false},
				{"PreCanceled", canceled, false},
			} {
				b.Run(mode.name, func(b *testing.B) {
					check := func(result map[string][]g.MapVal, err error) {
						b.Helper()
						if mode.ctx.Err() != nil {
							if result != nil || !errors.Is(err, mode.ctx.Err()) {
								b.Fatalf("finished context returned data: %v", err)
							}
						} else if err != nil || !reflect.DeepEqual(result, want) {
							b.Fatalf("snapshot differs from legacy output: %v", err)
						}
					}
					run := func() (map[string][]g.MapVal, error) {
						if mode.legacy {
							return legacy(), nil
						}
						return mappingStatusSnapshotContext(mode.ctx)
					}
					check(run())
					b.ReportAllocs()
					b.ResetTimer()
					var last map[string][]g.MapVal
					var err error
					for i := 0; i < b.N; i++ {
						last, err = run()
					}
					b.StopTimer()
					check(last, err)
				})
			}
		})
	}
}
