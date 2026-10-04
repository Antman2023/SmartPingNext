package http

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"
)

// Include the original ordered scan and, when needed, its first-lookup map
// construction. Both benchmark paths measure complete preparation plus lookups.
func benchmarkPingIndexWithoutRequestContext(timestamps []string) pingTimelineIndex {
	index := pingTimelineIndex{timestamps: timestamps, ordered: sort.StringsAreSorted(timestamps)}
	if !index.ordered {
		index.positions = make(map[string]int, len(timestamps))
		for position, stamp := range timestamps {
			index.positions[stamp] = position
		}
	}
	return index
}

func BenchmarkPingTimelineIndexContext(b *testing.B) {
	active, finish := context.WithCancel(context.Background())
	defer finish()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()
	for _, size := range []int{361, maxPingRangeMinutes + 1} {
		for _, rollback := range []bool{false, true} {
			labels := pingIndexContextLabels(size, rollback)
			positions := make(map[string]int, size)
			for position, stamp := range labels {
				positions[stamp] = position
			}
			queries := []string{"", labels[0], labels[size-1]}
			for _, mode := range []struct {
				name   string
				ctx    context.Context
				legacy bool
				want   error
			}{
				{"WithoutRequestContext", canceled, true, nil},
				{"Active", active, false, nil},
				{"Canceled", canceled, false, context.Canceled},
				{"Deadline", expired, false, context.DeadlineExceeded},
			} {
				b.Run(fmt.Sprintf("%dSamples/Rollback=%t/%s", size, rollback, mode.name), func(b *testing.B) {
					build := func() (pingTimelineIndex, error) {
						if mode.legacy {
							return benchmarkPingIndexWithoutRequestContext(labels), nil
						}
						return newPingTimelineIndexContext(mode.ctx, labels)
					}
					check := func(index pingTimelineIndex, err error) {
						b.Helper()
						if !errors.Is(err, mode.want) {
							b.Fatalf("index error=%v, want %v", err, mode.want)
						}
						if mode.want != nil {
							if !reflect.DeepEqual(index, pingTimelineIndex{}) {
								b.Fatal("finished request returned index state")
							}
							return
						}
						if !reflect.DeepEqual(index.timestamps, labels) || index.ordered == rollback ||
							(rollback && !reflect.DeepEqual(index.positions, positions)) || (!rollback && index.positions != nil) {
							b.Fatal("index changed labels, ordering or fallback positions")
						}
						for _, stamp := range queries {
							want, exists := positions[stamp]
							got, found := index.lookup(stamp)
							if got != want || found != exists {
								b.Fatal("index lookup lost a sample or its last position")
							}
						}
					}
					check(build())
					b.ReportAllocs()
					b.ResetTimer()
					var last pingTimelineIndex
					var err error
					for i := 0; i < b.N; i++ {
						last, err = build()
						if !errors.Is(err, mode.want) {
							b.Fatal(err)
						}
						if err == nil {
							for _, stamp := range queries {
								last.lookup(stamp)
							}
						}
					}
					b.StopTimer()
					check(last, err)
				})
			}
		}
	}
}
